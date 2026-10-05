package atom

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFetchFeedParsesAtomFixtures(t *testing.T) {
	tests := []struct {
		name string
		file string
		want AtomFeed
	}{
		{
			name: "default namespace and HTML text construct",
			file: "atom-default.xml",
			want: AtomFeed{
				Title:   "News & Updates",
				Updated: "2026-10-01T12:30:00Z",
				Link:    "https://example.com/feed?a=1&b=2",
				Entries: []AtomEntry{
					{
						Title:     "First & entry",
						Link:      "https://example.com/posts/1?a=1&b=2",
						Content:   "<p>Fish &amp; chips</p>",
						Published: "2026-10-01T12:00:00Z",
					},
					{
						Title:   "Second entry",
						Link:    "https://example.com/posts/2",
						Content: "Summary & details",
					},
					{
						Title:   "Explicit text",
						Link:    "https://example.com/posts/3",
						Content: "Plain & safe",
					},
				},
			},
		},
		{
			name: "prefixed namespace and XHTML construct",
			file: "atom-prefixed.xml",
			want: AtomFeed{
				Title: "Prefixed feed",
				Link:  "https://example.com/",
				Entries: []AtomEntry{
					{
						Title:   "Rich entry",
						Link:    "https://example.com/rich",
						Content: `<div xmlns="http://www.w3.org/1999/xhtml"><p>Rich <strong>text</strong></p></div>`,
					},
				},
			},
		},
		{
			name: "empty feed",
			file: "atom-empty.xml",
			want: AtomFeed{Entries: []AtomEntry{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(body)
			}))
			defer server.Close()

			got, err := fetchFeed(context.Background(), server.URL)
			if err != nil {
				t.Fatalf("fetchFeed() error = %v", err)
			}
			if !reflect.DeepEqual(got, &tt.want) {
				t.Errorf("fetchFeed() = %#v, want %#v", got, &tt.want)
			}
		})
	}
}

func TestFetchFeedRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed XML", body: `<feed xmlns="http://www.w3.org/2005/Atom"><title>broken`},
		{name: "non-Atom root", body: `<html><title>not a feed</title></html>`},
		{name: "wrong namespace", body: `<feed xmlns="urn:not-atom"><title>not a feed</title></feed>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			if _, err := fetchFeed(context.Background(), server.URL); err == nil {
				t.Fatal("fetchFeed() error = nil, want an error")
			}
		})
	}
}

func TestFetchFeedHTTPBehavior(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantStatus int
		wantErr    bool
		wantAgent  bool
	}{
		{
			name:      "successful response sends user agent",
			status:    http.StatusOK,
			body:      `<feed xmlns="http://www.w3.org/2005/Atom"><title>OK</title></feed>`,
			wantAgent: true,
		},
		{
			name:       "server error is rejected",
			status:     http.StatusInternalServerError,
			body:       "temporarily unavailable",
			wantStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:   "redirect is followed",
			status: http.StatusFound,
			body:   `<feed xmlns="http://www.w3.org/2005/Atom"><title>Redirected</title></feed>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userAgent string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				userAgent = r.UserAgent()
				if tt.status == http.StatusFound && r.URL.Path != "/destination" {
					http.Redirect(w, r, "/destination", http.StatusFound)
					return
				}
				status := tt.status
				if status == http.StatusFound {
					status = http.StatusOK
				}
				w.WriteHeader(status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			got, err := fetchFeed(context.Background(), server.URL)
			if tt.wantErr {
				if err == nil {
					t.Fatal("fetchFeed() error = nil, want an error")
				}
				if !strings.Contains(err.Error(), fmt.Sprint(tt.wantStatus)) {
					t.Errorf("fetchFeed() error = %q, want status %d", err, tt.wantStatus)
				}
				return
			}
			if err != nil {
				t.Fatalf("fetchFeed() error = %v", err)
			}
			if got == nil {
				t.Fatal("fetchFeed() = nil, want parsed feed")
			}
			if tt.wantAgent && userAgent != "atom1c" {
				t.Errorf("User-Agent = %q, want atom1c", userAgent)
			}
		})
	}
}

func TestFetchFeedContextAndTimeout(t *testing.T) {
	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := fetchFeed(ctx, "http://example.invalid")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("fetchFeed() error = %v, want context.Canceled", err)
		}
	})

	t.Run("cancellation during request", func(t *testing.T) {
		started := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := fetchFeed(ctx, server.URL)
			done <- err
		}()
		<-started
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("fetchFeed() error = %v, want context.Canceled", err)
		}
	})

	t.Run("client timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()

		_, err := fetchFeedWithClient(context.Background(), server.URL, &http.Client{Timeout: 20 * time.Millisecond})
		if err == nil {
			t.Fatal("fetchFeedWithClient() error = nil, want timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("fetchFeedWithClient() error = %v, want timeout error", err)
		}
	})
}

func TestDefaultFeedHTTPClientHasTimeout(t *testing.T) {
	if feedHTTPClient.Timeout != 15*time.Second {
		t.Errorf("default feed HTTP timeout = %v, want 15s", feedHTTPClient.Timeout)
	}
}

func TestFetchFeedRejectsInvalidURL(t *testing.T) {
	if _, err := fetchFeed(context.Background(), "://invalid"); err == nil {
		t.Fatal("fetchFeed() error = nil, want invalid URL error")
	}
}

func TestFetchFeedWrapsBodyReadErrorsAndClosesBody(t *testing.T) {
	wantErr := errors.New("body read failed")
	body := &failingReadCloser{err: wantErr}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       body,
			Request:    req,
		}, nil
	})}

	_, err := fetchFeedWithClient(context.Background(), "https://example.com/feed", client)
	if !errors.Is(err, wantErr) {
		t.Errorf("fetchFeedWithClient() error = %v, want wrapped body read error", err)
	}
	if !body.closed {
		t.Error("response body was not closed after read failure")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type failingReadCloser struct {
	err    error
	closed bool
}

func (r *failingReadCloser) Read([]byte) (int, error) {
	return 0, r.err
}

func (r *failingReadCloser) Close() error {
	r.closed = true
	return nil
}

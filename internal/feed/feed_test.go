package feed

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

func TestFetchParsesAtomFixtures(t *testing.T) {
	tests := []struct {
		name string
		file string
		want Feed
	}{
		{
			name: "default namespace and HTML text construct",
			file: "atom-default.xml",
			want: Feed{
				Title:   "News & Updates",
				Updated: expectedDate("2026-10-01T12:30:00Z", "2026-10-01T12:30:00Z"),
				Link:    "https://example.com/feed?a=1&b=2",
				Entries: []Entry{
					{
						Title:       "First & entry",
						Link:        "https://example.com/posts/1?a=1&b=2",
						Content:     "<p>Fish &amp; chips</p>",
						ContentKind: ContentHTML,
						Published:   expectedDate("2026-10-01T12:00:00Z", "2026-10-01T12:00:00Z"),
					},
					{
						Title:       "Second entry",
						Link:        "https://example.com/posts/2",
						Content:     "Summary & details",
						ContentKind: ContentText,
					},
					{
						Title:       "Explicit text",
						Link:        "https://example.com/posts/3",
						Content:     "Plain & safe",
						ContentKind: ContentText,
					},
				},
			},
		},
		{
			name: "prefixed namespace and XHTML construct",
			file: "atom-prefixed.xml",
			want: Feed{
				Title: "Prefixed feed",
				Link:  "https://example.com/",
				Entries: []Entry{
					{
						Title:       "Rich entry",
						Link:        "https://example.com/rich",
						Content:     `<div xmlns="http://www.w3.org/1999/xhtml"><p>Rich <strong>text</strong></p></div>`,
						ContentKind: ContentXHTML,
					},
				},
			},
		},
		{
			name: "empty feed",
			file: "atom-empty.xml",
			want: Feed{Entries: []Entry{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			got := fetchFixture(t, body)
			if !reflect.DeepEqual(got, &tt.want) {
				t.Errorf("Fetch() = %#v, want %#v", got, &tt.want)
			}
		})
	}
}

func TestFetchParsesRSSFixtures(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "rss-comprehensive.xml"))
	if err != nil {
		t.Fatal(err)
	}
	got := fetchFixture(t, body)
	want := &Feed{
		Title:   "RSS & Updates",
		Link:    "https://example.com/",
		Updated: expectedDate("Thu, 01 Oct 2026 12:30:00 +0000", "2026-10-01T12:30:00Z"),
		Entries: []Entry{
			{
				ID:              "entry-encoded",
				GUIDIsPermaLink: boolPointer(false),
				Title:           "Encoded entry",
				Link:            "https://example.com/posts/encoded",
				Content:         "<p>Fish &amp; chips</p>",
				ContentKind:     ContentHTML,
				Published:       expectedDate("Thu, 01 Oct 2026 12:00:00 -0400", "2026-10-01T16:00:00Z"),
			},
			{
				ID:              "https://example.com/posts/guid",
				GUIDIsPermaLink: boolPointer(true),
				Title:           "GUID link",
				Link:            "https://example.com/posts/guid",
				Content:         "<p>Tom &amp; Jerry</p>",
				ContentKind:     ContentHTML,
				Published:       expectedDate("02 Oct 26 13:00 -0700", "2026-10-02T20:00:00Z"),
			},
			{
				ID:              "opaque-id",
				GUIDIsPermaLink: boolPointer(false),
				Title:           "Empty encoded content",
				ContentKind:     ContentHTML,
				Published:       SourceDate{Raw: "not a date"},
			},
			{
				Title:       "Missing optional values",
				ContentKind: ContentHTML,
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Fetch() = %#v, want %#v", got, want)
	}
}

func TestFetchParsesEmptyRSSFeed(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "rss-empty.xml"))
	if err != nil {
		t.Fatal(err)
	}
	got := fetchFixture(t, body)
	want := &Feed{Title: "Empty RSS", Entries: []Entry{}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Fetch() = %#v, want %#v", got, want)
	}
}

func TestFetchRSSLastBuildDatePrecedesChannelPubDate(t *testing.T) {
	body := []byte(`<rss version="2.0"><channel><pubDate>Thu, 01 Oct 2026 12:00:00 GMT</pubDate><lastBuildDate>Fri, 02 Oct 2026 12:00:00 GMT</lastBuildDate></channel></rss>`)
	got := fetchFixture(t, body)
	want := expectedDate("Fri, 02 Oct 2026 12:00:00 GMT", "2026-10-02T12:00:00Z")
	if !reflect.DeepEqual(got.Updated, want) {
		t.Errorf("Feed.Updated = %#v, want %#v", got.Updated, want)
	}
}

func TestFetchPreservesAtomIdentifiersAndEntryUpdateDates(t *testing.T) {
	body := []byte(`<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>tag:example.com,2026:entry</id><title>Updated</title><updated>2026-10-02T12:00:00+02:00</updated><content type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml">body</div></content></entry></feed>`)
	got := fetchFixture(t, body)
	want := Entry{
		ID:          "tag:example.com,2026:entry",
		Title:       "Updated",
		Content:     `<div xmlns="http://www.w3.org/1999/xhtml">body</div>`,
		ContentKind: ContentXHTML,
		Updated:     expectedDate("2026-10-02T12:00:00+02:00", "2026-10-02T10:00:00Z"),
	}
	if len(got.Entries) != 1 || !reflect.DeepEqual(got.Entries[0], want) {
		t.Errorf("Feed.Entries = %#v, want one entry %#v", got.Entries, want)
	}
}

func TestFetchRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed XML", body: `<feed xmlns="http://www.w3.org/2005/Atom"><title>broken`},
		{name: "non-feed root", body: `<html><title>not a feed</title></html>`},
		{name: "wrong Atom namespace", body: `<feed xmlns="urn:not-atom"><title>not a feed</title></feed>`},
		{name: "RSS 1.0 RDF", body: `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"/>`},
		{name: "unsupported RSS version", body: `<rss version="0.91"><channel/></rss>`},
		{name: "missing RSS version", body: `<rss><channel/></rss>`},
		{name: "missing RSS channel", body: `<rss version="2.0"/>`},
		{name: "namespaced RSS root", body: `<rss xmlns="urn:rss" version="2.0"><channel/></rss>`},
		{name: "invalid RSS GUID flag", body: `<rss version="2.0"><channel><item><guid isPermaLink="sometimes">id</guid></item></channel></rss>`},
		{name: "empty RSS GUID flag", body: `<rss version="2.0"><channel><item><guid isPermaLink="">id</guid></item></channel></rss>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			if _, err := Fetch(context.Background(), server.URL); err == nil {
				t.Fatal("Fetch() error = nil, want an error")
			}
		})
	}
}

func TestParseSourceDates(t *testing.T) {
	tests := []struct {
		name  string
		parse func(string) SourceDate
		raw   string
		want  string
	}{
		{name: "Atom UTC", parse: atomDate, raw: "2026-10-01T12:00:00Z", want: "2026-10-01T12:00:00Z"},
		{name: "Atom offset and fractional seconds", parse: atomDate, raw: "2026-10-01T12:00:00.123456+02:30", want: "2026-10-01T09:30:00.123456Z"},
		{name: "RSS RFC1123 numeric offset", parse: rssDate, raw: "Thu, 01 Oct 2026 12:00:00 -0400", want: "2026-10-01T16:00:00Z"},
		{name: "RSS RFC1123 named zone", parse: rssDate, raw: "Thu, 01 Oct 2026 12:00:00 GMT", want: "2026-10-01T12:00:00Z"},
		{name: "RSS common named zone", parse: rssDate, raw: "Fri, 02 Oct 2026 13:00:00 EDT", want: "2026-10-02T17:00:00Z"},
		{name: "RSS RFC822 short year numeric offset", parse: rssDate, raw: "02 Oct 26 13:00 -0700", want: "2026-10-02T20:00:00Z"},
		{name: "source date whitespace retained", parse: atomDate, raw: " 2026-10-01T12:00:00Z\n", want: "2026-10-01T12:00:00Z"},
		{name: "RSS source date whitespace retained", parse: rssDate, raw: " Thu, 01 Oct 2026 12:00:00 GMT ", want: "2026-10-01T12:00:00Z"},
		{name: "invalid Atom date retained", parse: atomDate, raw: "not an Atom date", want: ""},
		{name: "invalid date retained", parse: rssDate, raw: "bad date", want: ""},
		{name: "unknown timezone is not guessed", parse: rssDate, raw: "Thu, 01 Oct 2026 12:00:00 XYZ", want: ""},
		{name: "empty date", parse: atomDate, raw: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.parse(tt.raw)
			if got.Raw != tt.raw {
				t.Errorf("Raw = %q, want %q", got.Raw, tt.raw)
			}
			if tt.want == "" {
				if got.Normalized != nil {
					t.Errorf("Normalized = %s, want nil", got.Normalized)
				}
				return
			}
			want, err := time.Parse(time.RFC3339Nano, tt.want)
			if err != nil {
				t.Fatal(err)
			}
			if got.Normalized == nil || !got.Normalized.Equal(want) || got.Normalized.Location() != time.UTC {
				t.Errorf("Normalized = %v, want UTC %s", got.Normalized, want)
			}
		})
	}
}

func TestFetchHTTPBehavior(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantStatus int
		wantErr    bool
		wantAgent  bool
	}{
		{
			name:      "successful response sends user agent and ignores content type",
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
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			got, err := Fetch(context.Background(), server.URL)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Fetch() error = nil, want an error")
				}
				if !strings.Contains(err.Error(), fmt.Sprint(tt.wantStatus)) {
					t.Errorf("Fetch() error = %q, want status %d", err, tt.wantStatus)
				}
				return
			}
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if got == nil {
				t.Fatal("Fetch() = nil, want parsed feed")
			}
			if tt.wantAgent && userAgent != "atom1c" {
				t.Errorf("User-Agent = %q, want atom1c", userAgent)
			}
		})
	}
}

func TestFetchContextAndTimeout(t *testing.T) {
	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := Fetch(ctx, "http://example.invalid")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Fetch() error = %v, want context.Canceled", err)
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
			_, err := Fetch(ctx, server.URL)
			done <- err
		}()
		<-started
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Fetch() error = %v, want context.Canceled", err)
		}
	})

	t.Run("client timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()

		_, err := fetchWithClient(context.Background(), server.URL, &http.Client{Timeout: 20 * time.Millisecond})
		if err == nil {
			t.Fatal("fetchWithClient() error = nil, want timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("fetchWithClient() error = %v, want timeout error", err)
		}
	})
}

func TestDefaultFeedHTTPClientHasTimeout(t *testing.T) {
	if feedHTTPClient.Timeout != 15*time.Second {
		t.Errorf("default feed HTTP timeout = %v, want 15s", feedHTTPClient.Timeout)
	}
}

func TestFetchRejectsInvalidURL(t *testing.T) {
	if _, err := Fetch(context.Background(), "://invalid"); err == nil {
		t.Fatal("Fetch() error = nil, want invalid URL error")
	}
}

func TestFetchWrapsBodyReadErrorsAndClosesBody(t *testing.T) {
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

	_, err := fetchWithClient(context.Background(), "https://example.com/feed", client)
	if !errors.Is(err, wantErr) {
		t.Errorf("fetchWithClient() error = %v, want wrapped body read error", err)
	}
	if !body.closed {
		t.Error("response body was not closed after read failure")
	}
}

func TestFetchClosesResponseBodyForAllResponseOutcomes(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{
			name:   "success",
			status: http.StatusOK,
			body:   `<feed xmlns="http://www.w3.org/2005/Atom"></feed>`,
		},
		{
			name:      "HTTP failure",
			status:    http.StatusBadGateway,
			body:      "failed",
			wantError: true,
		},
		{
			name:      "parse failure",
			status:    http.StatusOK,
			body:      "not XML",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackedReadCloser{Reader: strings.NewReader(tt.body)}
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: tt.status,
					Status:     fmt.Sprintf("%d %s", tt.status, http.StatusText(tt.status)),
					Header:     make(http.Header),
					Body:       body,
					Request:    req,
				}, nil
			})}

			_, err := fetchWithClient(context.Background(), "https://example.com/feed", client)
			if tt.wantError && err == nil {
				t.Fatal("fetchWithClient() error = nil, want an error")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("fetchWithClient() error = %v", err)
			}
			if !body.closed {
				t.Error("response body was not closed")
			}
		})
	}
}

func fetchFixture(t *testing.T, body []byte) *Feed {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	got, err := Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	return got
}

func expectedDate(raw, normalized string) SourceDate {
	if normalized == "" {
		return SourceDate{Raw: raw}
	}
	parsed, err := time.Parse(time.RFC3339Nano, normalized)
	if err != nil {
		panic(err)
	}
	return SourceDate{Raw: raw, Normalized: &parsed}
}

func boolPointer(value bool) *bool { return &value }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type failingReadCloser struct {
	err    error
	closed bool
}

type trackedReadCloser struct {
	*strings.Reader
	closed bool
}

func (r *trackedReadCloser) Close() error {
	r.closed = true
	return nil
}

func (r *failingReadCloser) Read([]byte) (int, error) { return 0, r.err }

func (r *failingReadCloser) Close() error {
	r.closed = true
	return nil
}

package feed

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestRSSNamespaceCollisions(t *testing.T) {
	base := `<rss version="2.0" xmlns:x="urn:extension" xmlns:a="http://www.w3.org/2005/Atom" xmlns:c="http://purl.org/rss/1.0/modules/content/"><channel><title>Core</title><link>https://example.com</link><pubDate>Thu, 01 Oct 2026 12:00:00 GMT</pubDate><lastBuildDate>Thu, 01 Oct 2026 13:00:00 GMT</lastBuildDate><item><title>Item</title><link>https://example.com/item</link><guid isPermaLink="false">id</guid><description>Body</description><pubDate>Thu, 01 Oct 2026 12:00:00 GMT</pubDate></item></channel></rss>`
	want, err := parseFeed([]byte(base))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, old, replacement string }{
		{"Atom link after website", `</link>`, `</link><a:link href="https://example.com/feed"/>`},
		{"channel fields", `</item>`, `</item><x:title>Wrong</x:title><x:link>wrong</x:link><x:pubDate>bad</x:pubDate><x:lastBuildDate>bad</x:lastBuildDate>`},
		{"item fields", `</item>`, `<x:title>Wrong</x:title><x:link>wrong</x:link><x:guid isPermaLink="invalid">wrong</x:guid><x:description>Wrong</x:description><x:pubDate>bad</x:pubDate><x:encoded>Wrong</x:encoded></item>`},
		{"extension item", `</channel>`, `<x:item><title>Wrong</title></x:item></channel>`},
		{"extension channel", `</rss>`, `<x:channel><title>Wrong</title></x:channel></rss>`},
		{"extension root attribute", `version="2.0"`, `version="2.0" x:version="wrong"`},
		{"extension GUID attribute", `isPermaLink="false"`, `isPermaLink="false" x:isPermaLink="invalid"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFeed([]byte(strings.ReplaceAll(base, tt.old, tt.replacement)))
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v, %v; want %#v", got, err, want)
			}
		})
	}
	t.Run("namespaced attributes cannot supply core values", func(t *testing.T) {
		if _, err := parseFeed([]byte(`<rss xmlns:x="urn:x" x:version="2.0"><channel/></rss>`)); err == nil {
			t.Fatal("accepted extension version")
		}
		got := fetchFixture(t, []byte(`<rss version="2.0" xmlns:x="urn:x"><channel><item><guid x:isPermaLink="false">https://example.com</guid></item></channel></rss>`))
		if got.Entries[0].Link != "https://example.com" {
			t.Fatal("extension attribute replaced default GUID flag")
		}
	})
	t.Run("content URI independent of prefix", func(t *testing.T) {
		got := fetchFixture(t, []byte(strings.Replace(base, `</description>`, `</description><c:encoded>Encoded</c:encoded>`, 1)))
		if got.Entries[0].Content != "Encoded" {
			t.Fatal("standard content namespace ignored")
		}
	})
}

func TestRSSCommonDateVariants(t *testing.T) {
	for _, raw := range []string{
		"Thu, 1 Oct 2026 12:00:00 GMT", "Thu, 01 Oct 26 12:00:00 GMT",
		"Thu, 1 Oct 26 12:00:00 +0000", "Thu, 1 Oct 26 14:00:00 +02:00",
		"1 Oct 2026 12:00 GMT", "1 Oct 26 12:00:00 +0000", "Thu, 1 Oct 26 12:00 +0000",
	} {
		t.Run(raw, func(t *testing.T) {
			want := expectedDate(raw, "2026-10-01T12:00:00Z")
			if got := rssDate(raw); !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v; want %#v", got, want)
			}
		})
	}
	got := fetchFixture(t, []byte(`<rss version="2.0"><channel><item><pubDate>Thu, 1 Oct 2026 12:00:00 GMT</pubDate></item></channel></rss>`))
	if got.Entries[0].Published.Normalized == nil {
		t.Fatal("Fetch did not normalize single-digit date")
	}
}

func TestFetchWholeXMLDocument(t *testing.T) {
	for _, root := range []string{`<feed xmlns="http://www.w3.org/2005/Atom"/>`, `<rss version="2.0"><channel/></rss>`} {
		for _, tt := range []struct {
			name, prefix, suffix string
			valid                bool
		}{
			{"extra root", "", `<extra/>`, false},
			{"trailing text", "", `oops`, false},
			{"leading text", `oops`, "", false},
			{"broken trailing XML", "", `<`, false},
			{"broken comment", "", `<!--`, false},
			{"legal surrounding tokens", `<?xml version="1.0"?><!--before--> `, "\n<!--after--><?done ok?> ", true},
		} {
			t.Run(root+tt.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, tt.prefix+root+tt.suffix) }))
				defer server.Close()
				got, err := Fetch(context.Background(), server.URL)
				if (err == nil) != tt.valid || (!tt.valid && got != nil) {
					t.Fatalf("got %v, %v; valid=%v", got, err, tt.valid)
				}
			})
		}
	}
}

func TestFetchDecodedResponseLimit(t *testing.T) {
	const limit = 10 * 1024 * 1024
	root := `<feed xmlns="http://www.w3.org/2005/Atom"/>`
	for _, tt := range []struct {
		name   string
		size   int
		length int64
	}{
		{"exact limit", limit, limit}, {"oversize", limit + 4096, limit + 4096},
		{"absent length", limit + 1, -1}, {"misleading length", limit + 1, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackedReadCloser{Reader: strings.NewReader(root + strings.Repeat(" ", tt.size-len(root)))}
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: body, ContentLength: tt.length, Header: make(http.Header), Request: req}, nil
			})}
			got, err := fetchWithClient(context.Background(), "https://example.com", client)
			if tt.size > limit {
				if err == nil || !strings.Contains(err.Error(), "10485760") || got != nil {
					t.Fatalf("got %v, %v; want descriptive size rejection", got, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !body.closed {
				t.Fatal("body not closed")
			}
			if read := tt.size - body.Len(); read > limit+1 {
				t.Fatalf("read %d bytes, want at most limit+1", read)
			}
		})
	}
	for _, compressed := range []bool{false, true} {
		t.Run(fmt.Sprintf("streamed compressed=%v", compressed), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				var out io.Writer = w
				if compressed {
					w.Header().Set("Content-Encoding", "gzip")
					z := gzip.NewWriter(w)
					defer z.Close()
					out = z
				} else {
					w.(http.Flusher).Flush()
				}
				_, _ = io.WriteString(out, root+strings.Repeat(" ", limit+1-len(root)))
			}))
			defer server.Close()
			if got, err := Fetch(context.Background(), server.URL); err == nil || got != nil || !strings.Contains(err.Error(), "10485760") {
				t.Fatalf("got %v, %v; want size rejection", got, err)
			}
		})
	}
}

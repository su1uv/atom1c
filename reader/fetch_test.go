package reader

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/encoding/charmap"
)

func articleHTML() string {
	var body strings.Builder
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&body, "<p>Paragraph %d explains how a dependable article reader extracts meaningful story content while ignoring navigation, advertising, and unrelated interface controls. The text should preserve words and links with enough detail to exercise readability extraction.</p>", i)
	}
	return `<!doctype html><html><head><title>Site story | Example</title><meta name="author" content="Ada Author"><meta property="article:published_time" content="2026-06-01T12:00:00Z"><meta property="og:site_name" content="Example Journal"><nav>Subscribe Sign in Latest News</nav><script>tracking()</script></head><body><header>Website Header</header><main><article><h1>Site story</h1>` + body.String() + `<ul><li>First point</li><li>Second point</li></ul><a href="/related">Related</a><a href="javascript:alert(1)">Unsafe</a></article></main><footer>Copyright and footer links</footer></body></html>`
}

func TestFetcherExtractsArticleMetadataAndMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing user agent")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTML()))
	}))
	defer server.Close()
	article, err := NewFetcher(server.Client()).Fetch(t.Context(), server.URL+"/story")
	if err != nil {
		t.Fatal(err)
	}
	if article.URL != server.URL+"/story" || article.Title != "Site story | Example" || article.Author != "Ada Author" || article.SiteName != "Example Journal" {
		t.Fatalf("metadata = %#v", article)
	}
	for _, want := range []string{"Paragraph 0 explains", "- First point", "- Second point", "[Related](" + server.URL + "/related)"} {
		if !strings.Contains(article.Markdown, want) {
			t.Errorf("markdown lacks %q: %s", want, article.Markdown)
		}
	}
	for _, unwanted := range []string{"Subscribe Sign in", "tracking()", "Website Header", "Copyright and footer", "javascript:"} {
		if strings.Contains(article.Markdown, unwanted) {
			t.Errorf("markdown contains clutter %q", unwanted)
		}
	}
	if article.Published == nil || !article.Published.Equal(time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("published = %v", article.Published)
	}
}

func TestFetcherUsesFinalRedirectURLAsArticleBase(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(articleHTML()))
	}))
	defer final.Close()
	redirect := httptest.NewServer(http.RedirectHandler(final.URL+"/nested/story", http.StatusFound))
	defer redirect.Close()
	article, err := NewFetcher(redirect.Client()).Fetch(t.Context(), redirect.URL+"/redirect")
	if err != nil {
		t.Fatal(err)
	}
	if article.URL != final.URL+"/nested/story" || !strings.Contains(article.Markdown, "[Related]("+final.URL+"/related)") {
		t.Fatalf("redirect result: %#v", article)
	}
}

func TestFetcherRejectsUnusableResponses(t *testing.T) {
	for _, tt := range []struct {
		name              string
		status            int
		contentType, body string
	}{
		{"status", http.StatusForbidden, "text/html", articleHTML()},
		{"nonhtml", http.StatusOK, "application/pdf", "pdf"},
		{"no article", http.StatusOK, "text/html", "<html><body><nav>Only a navigation bar</nav></body></html>"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			if _, err := NewFetcher(server.Client()).Fetch(t.Context(), server.URL); err == nil {
				t.Fatal("expected fetch failure")
			}
		})
	}
}

func TestFetcherHTTPStatusErrorDoesNotExposeUntrustedReason(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Status: "503 \x1b]52;c;steal\a", Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})}
	_, err := NewFetcher(client).Fetch(t.Context(), "https://example.test/article")
	if err == nil || !strings.Contains(err.Error(), "503") || strings.Contains(err.Error(), "\x1b") || strings.Contains(err.Error(), "steal") {
		t.Fatalf("unsafe status error: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestFetcherBodyLimitAndCancellation(t *testing.T) {
	t.Run("limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(strings.Repeat("x", MaxResponseBytes+1)))
		}))
		defer server.Close()
		if _, err := NewFetcher(server.Client()).Fetch(t.Context(), server.URL); err == nil || !strings.Contains(err.Error(), "limit") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
		defer cancel()
		if _, err := NewFetcher(server.Client()).Fetch(ctx, server.URL); err == nil {
			t.Fatal("expected cancellation")
		}
	})
}

func TestFetcherDecodesLegacyCharset(t *testing.T) {
	content := strings.Replace(articleHTML(), "Paragraph 0", "Café 0", 1)
	encoded, err := charmap.Windows1252.NewEncoder().Bytes([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=windows-1252")
		_, _ = w.Write(encoded)
	}))
	defer server.Close()
	article, err := NewFetcher(server.Client()).Fetch(t.Context(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(article.Markdown, "Café 0") {
		t.Fatalf("legacy encoding not decoded: %q", article.Markdown)
	}
}

func TestValidateArticleURL(t *testing.T) {
	for _, raw := range []string{"https://example.test/story", "http://example.test:8080/a"} {
		if _, err := validateArticleURL(raw); err != nil {
			t.Errorf("%q: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "javascript:alert(1)", "file:///etc/passwd", "https://user:pass@example.test/", "//example.test/story"} {
		if _, err := validateArticleURL(raw); err == nil {
			t.Errorf("accepted unsafe URL %q", raw)
		}
	}
}

func TestSafeLinkRejectsActiveAndNonWebSchemes(t *testing.T) {
	for _, test := range []struct{ raw, base string }{
		{"javascript:alert(1)", "https://example.test/base"},
		{"data:text/html,evil", "https://example.test/base"},
		{"file:///etc/passwd", "https://example.test/base"},
		{"https://user:pass@example.test", "https://example.test/base"},
		{"mailto:writer@example.test", "https://example.test/base"},
		{"/relative", "https://user:secret@example.test/path/"},
	} {
		if got := SafeLink(test.raw, test.base); got != "" {
			t.Errorf("SafeLink(%q, %q) = %q", test.raw, test.base, got)
		}
	}
	if got := SafeLink("/story", "https://example.test/feed/"); got != "https://example.test/story" {
		t.Fatalf("relative link = %q", got)
	}
}

func TestSafeDialRejectsNonPublicAddresses(t *testing.T) {
	for _, address := range []string{"127.0.0.1:80", "10.2.3.4:443", "169.254.1.1:80", "100.64.1.1:80", "192.0.2.1:80", "0.1.2.3:80", "192.88.99.1:80", "[::1]:80", "[::7f00:1]:80", "[fe80::1]:80", "[fec0::1]:80", "[2001:db8::1]:80", "[64:ff9b::7f00:1]:80", "[2001:20::1]:80", "[2002:7f00:1::1]:80", "[4000::1]:80"} {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			t.Fatal(err)
		}
		ip, err := netip.ParseAddr(host)
		if err != nil {
			t.Fatal(err)
		}
		if isPublicAddress(ip) {
			t.Errorf("classified special-use address as public: %s", address)
		}
		if _, err := safeDialContext(t.Context(), "tcp", address); err == nil {
			t.Errorf("dial accepted non-public address %s", address)
		}
	}
	for _, address := range []string{"8.8.8.8:53", "[2606:4700:4700::1111]:53"} {
		parts, _, _ := net.SplitHostPort(address)
		ip, _ := netip.ParseAddr(parts)
		if !isPublicAddress(ip) {
			t.Errorf("public address rejected: %s", address)
		}
	}
}

func TestRenderMarkdownStylesStructureAndFitsWidth(t *testing.T) {
	markdown := "# Main heading\n\n## Section\n\nA **bold** and *italic* paragraph with a [link](https://example.test).\n\n- First\n- Second\n\n1. One\n2. Two\n\n> Quoted thought\n\n```go\nfmt.Println(\"hello\")\n```"
	for _, dark := range []bool{true, false} {
		got, err := RenderMarkdown(markdown, 50, dark)
		if err != nil {
			t.Fatal(err)
		}
		plain := stripANSI(got)
		for _, want := range []string{"Main heading", "Section", "bold", "italic", "First", "Second", "One", "Two", "Quoted thought", "fmt.Println"} {
			if !strings.Contains(plain, want) {
				t.Errorf("dark=%v missing %q in %q", dark, want, plain)
			}
		}
		for _, line := range strings.Split(got, "\n") {
			if width := terminalWidth(line); width > 50 {
				t.Errorf("width %d exceeds 50: %q", width, line)
			}
		}
	}
}

func TestFeedContentMarkdownPreservesPlainTextAndConvertsMarkup(t *testing.T) {
	for _, tt := range []struct{ kind, content, want string }{
		{"text", "# literal <tag>\n*literal*", "# literal <tag>\n*literal*"},
		{"html", `<h2>Heading</h2><ol><li>First</li><li>Second</li></ol><a href="/next">Next</a><a href="javascript:alert(1)">Unsafe</a>`, "## Heading"},
		{"xhtml", `<div xmlns="http://www.w3.org/1999/xhtml"><p>Body</p></div>`, "Body"},
		{"xhtml", `<div xmlns="http://www.w3.org/1999/xhtml"><p><![CDATA[A < B]]></p></div>`, "A &lt; B"},
	} {
		got, err := FeedContentMarkdown(tt.content, tt.kind, "https://example.test/post")
		if err != nil {
			t.Fatal(err)
		}
		if tt.kind != "text" && !strings.Contains(got, tt.want) {
			t.Errorf("kind %s markdown %q missing %q", tt.kind, got, tt.want)
		}
		if strings.Contains(got, "javascript:") {
			t.Errorf("unsafe link survived HTML conversion: %q", got)
		}
		if tt.kind == "text" {
			rendered, err := RenderMarkdown(got, 80, true)
			if err != nil {
				t.Fatal(err)
			}
			plain := strings.Join(strings.Fields(stripANSI(rendered)), " ")
			for _, token := range []string{"#", "literal", "<tag>", "*literal*"} {
				if !strings.Contains(plain, token) {
					t.Errorf("plain content lost %q: markdown=%q rendered=%q", token, got, rendered)
				}
			}
		}
	}
	got, err := FeedContentMarkdown("body", "unsupported", "https://example.test")
	if err == nil || got != "" {
		t.Fatalf("unsupported = %q, %v", got, err)
	}
}

func stripANSI(s string) string  { return ansi.Strip(s) }
func terminalWidth(s string) int { return ansi.StringWidth(s) }

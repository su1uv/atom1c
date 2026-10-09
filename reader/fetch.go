package reader

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	markdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// MaxResponseBytes limits decoded HTML for each website response.
const MaxResponseBytes = 10 << 20

const fetchTimeout = 25 * time.Second

var nonPublicPrefixes = prefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/96", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/23", "2001:10::/28", "2001:20::/28",
	"2001:db8::/32", "2002::/16", "3fff::/20", "fc00::/7", "fe80::/10", "fec0::/10", "ff00::/8",
)

var publicIPv6Prefix = netip.MustParsePrefix("2000::/3")

// Fetcher retrieves a public HTML page and extracts its main article.
type Fetcher struct{ client *http.Client }

// NewFetcher creates a fetcher. A nil client selects the default SSRF-resistant
// HTTP client; an injected client is useful for callers with a controlled
// transport or tests using local fixtures.
func NewFetcher(client *http.Client) *Fetcher {
	if client == nil {
		client = safeHTTPClient()
	} else {
		copy := *client
		originalRedirect := copy.CheckRedirect
		copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if _, err := validateArticleURL(req.URL.String()); err != nil {
				return err
			}
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if originalRedirect != nil {
				return originalRedirect(req, via)
			}
			return nil
		}
		client = &copy
	}
	return &Fetcher{client: client}
}

// Fetch downloads a public HTML article and converts extracted content to
// Markdown. It accepts HTTP(S), follows at most five safe redirects, honors
// context cancellation, and rejects responses larger than MaxResponseBytes.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (Article, error) {
	parsed, err := validateArticleURL(rawURL)
	if err != nil {
		return Article{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Article{}, fmt.Errorf("create article request: %w", err)
	}
	req.Header.Set("User-Agent", "Atom1c/1.0 (+https://github.com/su1uv/atom1c)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9")
	response, err := f.client.Do(req)
	if err != nil {
		return Article{}, fmt.Errorf("fetch article: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Article{}, fmt.Errorf("fetch article: unexpected HTTP status %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		return Article{}, fmt.Errorf("fetch article: invalid content type: %w", err)
	}
	if mediaType != "text/html" && mediaType != "application/xhtml+xml" {
		return Article{}, fmt.Errorf("fetch article: unsupported content type %q", mediaType)
	}
	decoded, err := charset.NewReader(io.LimitReader(response.Body, MaxResponseBytes+1), response.Header.Get("Content-Type"))
	if err != nil {
		return Article{}, fmt.Errorf("decode article charset: %w", err)
	}
	page, err := io.ReadAll(io.LimitReader(decoded, MaxResponseBytes+1))
	if err != nil {
		return Article{}, fmt.Errorf("read article: %w", err)
	}
	if len(page) > MaxResponseBytes {
		return Article{}, fmt.Errorf("read article: decoded body exceeds %d-byte limit", MaxResponseBytes)
	}
	finalURL := response.Request.URL
	document, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		return Article{}, fmt.Errorf("parse article page: %w", err)
	}
	if !readability.CheckDocument(document) {
		return Article{}, fmt.Errorf("extract article: page has no readable article content")
	}
	extracted, err := readability.FromDocument(document, finalURL)
	if err != nil {
		return Article{}, fmt.Errorf("extract article: %w", err)
	}
	if extracted.Node == nil {
		return Article{}, fmt.Errorf("extract article: page has no readable article content")
	}
	sanitizeArticleNode(extracted.Node)
	var html strings.Builder
	if err := extracted.RenderHTML(&html); err != nil {
		return Article{}, fmt.Errorf("render extracted article: %w", err)
	}
	content, err := markdown.ConvertString(html.String(), converter.WithDomain(finalURL.String()))
	if err != nil {
		return Article{}, fmt.Errorf("convert article to Markdown: %w", err)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return Article{}, fmt.Errorf("extract article: page has no readable article content")
	}
	if len(content) > MaxMarkdownBytes {
		return Article{}, fmt.Errorf("extract article: Markdown exceeds %d-byte limit", MaxMarkdownBytes)
	}
	article := Article{URL: finalURL.String(), Title: extracted.Title(), Author: extracted.Byline(), SiteName: extracted.SiteName(), Markdown: content}
	if published, err := extracted.PublishedTime(); err == nil {
		published = published.UTC()
		article.Published = &published
	}
	return article, nil
}

func validateArticleURL(raw string) (*url.URL, error) {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid article URL: %w", err)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, fmt.Errorf("article URL must be an absolute HTTP(S) URL without credentials")
	}
	if u.Port() != "" {
		if _, err := strconv.ParseUint(u.Port(), 10, 16); err != nil {
			return nil, fmt.Errorf("invalid article URL port")
		}
	}
	return u, nil
}

func safeHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:             nil,
		ForceAttemptHTTP2: true,
		DialContext:       safeDialContext,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   fetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if _, err := validateArticleURL(req.URL.String()); err != nil {
				return err
			}
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var addresses []netip.Addr
	if literal, err := netip.ParseAddr(host); err == nil {
		addresses = []netip.Addr{literal}
	} else {
		addresses, err = net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("article host resolved to no addresses")
	}
	for _, address := range addresses {
		if !isPublicAddress(address) {
			return nil, fmt.Errorf("article host resolves to a non-public address")
		}
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
}

func isPublicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	if address.Is6() && !publicIPv6Prefix.Contains(address) {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func prefixes(raw ...string) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(raw))
	for _, prefix := range raw {
		result = append(result, netip.MustParsePrefix(prefix))
	}
	return result
}

# Step 9 — Full-article reader

Status: implemented; full-article retrieval, Markdown presentation, persistent
cache, feed-preview fallback, explicit reload/retry, and offline restart verified.
Roadmap: [Step 9](roadmap.md#9-retrieve-and-cache-full-website-articles).

## Confirmed behavior

- Implement this before SSH access.
- A reusable top-level Go package in this repository, with no imports from
  Atom1c's database, feed, or application packages. Keep the app integration thin.
- The standalone example exercises the package without Atom1c state or SQLite.
- Enter uses a successful cached website article immediately. On a cache miss,
  show feed preview while asynchronously fetching/extracting the linked public
  HTML page. Replace preview with full content at the article top on success.
- If fetch/extraction fails, keep the preview and explain the failure. A failed
  explicit reload preserves the last successful cached article.
- Cache extracted Markdown and metadata in SQLite, keyed by post and exact source
  link. Reuse offline after restart. `R` reloads; `r` retries a failed fetch; `f`
  toggles feed/full content. Feed refresh never refetches website articles.
- Use extracted title, author, and date when available, with feed metadata as
  fallback. Keep the feed name and original post URL.
- Center the article to about 80 terminal cells with responsive resizing, dark/
  light-aware Markdown styling, headings, nested/numbered lists, quotes, emphasis,
  links, and code blocks. Preserve scroll controls and Esc list-state restoration.
- Support public HTTP(S) HTML only. JavaScript pages, browser runtime, login, and
  credential support are deferred. Never execute website JavaScript.
- The initial cache has no expiry or size eviction. A changed post URL prevents
  reuse of its previous cache entry; post/feed deletion removes cached content.

## Package and dependency choices

- `reader/` owns portable article types, safe HTTP retrieval, Readability-style
  extraction, relative-link handling, HTML-to-Markdown conversion, and Glamour
  Markdown rendering. `reader/view/` owns the focused-column viewport, scrolling,
  and terminal layout. Neither imports an Atom1c `internal` package.
- `examples/reader/` is an independently runnable Bubble Tea example.
- A thin `internal/ui` adapter coordinates post snapshots, reader actions, and
  SQLite persistence.
- Selected dependencies: Readeck `go-readability/v2` v2.1.3,
  `html-to-markdown/v2` v2.5.2, Glamour v2.0.1. Their licenses are MIT; versions
  are pinned and compatible with Go 1.26.5+.
- The default article HTTP client permits only public destinations, rejects unsafe
  redirects, is context-cancellable, and caps decoded responses at 10 MiB. Tests
  inject clients for local HTTP fixtures.

## Independently reviewable increments

1. Build the independent `reader` package and standalone example. Add meaningful
   failing tests for HTTP behavior, extraction, URL/charset handling, limits,
   Markdown structure, theme, and cell-width rendering before implementation.
2. Add the article cache migration and generated sqlc queries. Test source scoping,
   stale-link write rejection, failure preservation, restart persistence, and
   foreign-key deletion cleanup using temporary SQLite.
3. Add the thin cache/fetch adapters and feed-preview/full-document composition.
   Test website metadata precedence/fallback, safe links, content toggling, and
   rendering errors.
4. Connect open/cache/fetch/reload/retry and stale-result handling. Test immediate
   preview, automatic cache-miss fetch, duplicate suppression, cancellation,
   changed-link rejection, failed-reload preservation, and reopening cached data.
5. Verify local-HTTP Atom/RSS workflows through the TUI, restart/offline behavior,
   layout/resizing, large documents, and the standalone example. Manually inspect
   an interactive terminal session.
6. Update README, roadmap, this plan, and concise MEMORY.md. Check whether
   `AGENTS.md` needs changes; defer JavaScript and authenticated-site support.

These are work increments, not fixed commit boundaries. Split further when needed.
Each authorized atomic commit must compile, pass applicable checks, and include
its relevant tests/docs.

## Acceptance criteria

- A link-only feed post with an extractable public HTML article becomes readable
  in the terminal as styled Markdown.
- Main content is extracted without page navigation clutter; relative links are
  resolved safely and non-web schemes are not emitted as terminal hyperlinks.
- Feed preview remains available during loading and after failure.
- Successful full articles remain readable offline after restart; failed reloads
  keep the last successful article.
- Stale fetches and changed post URLs cannot replace current content/cache.
- Existing feed refresh never triggers website retrieval; `R`, `r`, and `f` are
  reader-scoped.
- The standalone reader builds/runs without initializing Atom1c or SQLite.

## Verification

Use Go 1.26.5 or newer. Write and run behavior tests failing before implementation.
Use table-driven fixtures, local HTTP servers, and temporary SQLite; never rely on
external network test dependencies. Include race coverage for async/cache flows.

Before each explicitly authorized commit, review the intended diff and run:

```sh
go test ./... -timeout 30s
go vet ./...
go test ./... -run '^$'
git diff --check
```

Run `sqlc generate` for SQL changes and review generated diffs. Do not hand-edit
generated database files. Rebuild after migration changes. Use Go 1.26.5+ and
repository commit prefixes. Publishing always needs separate fresh authorization.

## Verification completed

- Reader package tests cover extraction/clutter removal, metadata, redirects,
  relative links, legacy charset decoding, status/content-type failures, no-article
  pages, cancellation, response bounds, SSRF address rejection, HTML conversion,
  safe links, dark/light Markdown, headings/lists/quotes/code, controls, and width.
- SQLite tests cover post/source URL scoping, stale-link writes, failed replacement,
  restart persistence, and cascade deletion.
- UI tests cover cache hit/offline restart, feed preview, async fetch, successful
  replacement, explicit reload/retry, stale changed-link response, content toggle,
  persisted Atom/RSS reads, and failed-reload fallback.
- A live in-memory/local HTTP + temporary-SQLite workflow verifies extraction,
  cache persistence, restart reuse without a network request, and failed reload.
- The reusable `reader` and `reader/view` packages have no imports from Atom1c
  internal packages. The standalone example uses both without SQLite or app state.
- An interactive PTY smoke exercised startup, persisted feed-preview reading,
  scrolling/page navigation, terminal resize, Home, Esc return, and clean quit.
- Full tests, UI race tests, vet, compile check, sqlc generation, and diff checks
  passed on 2026-10-08. Two read-only review passes led to fixes for terminal
  control injection, in-flight toggle handling, metadata escaping, special-use
  IP ranges, credential-bearing base URLs, empty-content fallback, and cancellation.
  Follow-up review found no Critical or Important findings.

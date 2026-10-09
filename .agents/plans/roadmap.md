# Atom1c roadmap

## Product and release scope

- A personal, self-hosted CLI feed aggregator and terminal reader accessed over SSH.
- One owner per instance, with multiple authorized SSH keys accessing the same data.
- Version 1.0 supports Atom and RSS 2.0; RSS 1.0/RDF is excluded.
- The 1.0 reader displays feed-provided content, including summaries when that is
  all the feed supplies.
- Public website article extraction and caching is included in the current 1.0
  roadmap (step 9); JavaScript and authenticated sites remain deferred.

## Milestone 1 — Reliable foundations

### 1. Reconcile database timestamps
- Choose a consistent SQLite/Go timestamp representation.
- Align migrations, sqlc configuration, generated code, and callers.
- Migrations are not deployed yet; update existing migration sources directly
  when needed instead of adding compatibility migrations for development data.
- Verify feed creation, retrieval, and fetch timestamps using a temporary database.
- **Done:** sqlc regeneration compiles and timestamp round trips work.

### 2. Fix and verify Atom fetching
- Parse link href attributes and fix entry unescaping.
- Add HTTP timeouts, status validation, and cancellation support.
- Replace the live-network test with fixtures and a local HTTP test server.
- Verify parsed titles, links, content, dates, and failure handling.
- **Done:** representative Atom feeds parse correctly with deterministic tests;
  HTTP fetching has a 15-second timeout, status validation, and context
  cancellation. See [implementation plan](02-atom-fetching.md).

### 3. Add RSS 2.0 support
- Detect RSS 2.0 and normalize Atom/RSS into a shared feed/entry representation.
- Test representative RSS fixtures.
- **Done:** both formats return the shared representation, ready for the storage
  and UI integrations in steps 4–7. See [implementation plan](03-rss-support.md).

## Milestone 2 — Real-data workflow

### 4. Persist fetched posts
- Add post migrations and sqlc queries.
- Define stable entry identity and deduplication rules before implementation.
- Store entries with their originating feed.
- Record successful fetches only after post persistence succeeds.
- **Done:** repeated fetching updates rather than duplicates posts; entries and
  successful-fetch timestamps commit atomically. Posts survive restart. See
  [implementation plan](04-post-persistence.md).

### 5. Fix UI focus and input routing
- Focus the first input when opening the add-feed modal.
- Prevent global shortcuts from interfering with modal input or filtering.
- Apply pagination actions only to the focused pane.
- Verify navigation, filtering, and modal reopening.
- **Done:** root-owned routing sends keys to the modal, active filter, or focused
  list; modal drafts survive reopening and shared list-pane logic avoids duplicated
  focus handling. See [implementation plan](05-ui-input-routing.md).

### 6. Connect feed management to SQLite
- Replace feed mocks with async database results and persist add-feed submissions.
- Validate nonblank names and absolute HTTP(S) URLs; retain drafts and show errors
  when submission fails.
- Use responsive database pagination and global case-insensitive name substring
  search; expose retry for load failures.
- Run page loads and inserts through Bubble Tea commands and typed result messages.
- **Done:** all feeds are reachable/searchable; successful additions appear selected
  immediately and survive database reopen. See [implementation plan](06-feed-management.md).

### 7. Fetch feeds and display their posts
- Add an explicit asynchronous refresh action.
- Replace post mocks with persisted posts from the feed opened with `tab`; moving
  through the feed list does not change the open posts pane.
- Load posts asynchronously, retaining the existing in-memory filtering and
  pagination. Show loading, empty, and recoverable failure states without blocking
  the UI.
- Use `R` to refresh the selected feed from either pane; keep `r` for retrying the
  failed database read or refresh operation in the focused pane.
- After a successful refresh, reload posts if that feed is open; distinguish a
  failed refresh from a failed post reload so retry repeats only the failed action.
- Route operational messages through the UI rather than printing over it.
- **Done:** add, explicitly refresh, and open a feed to browse persisted posts;
  these posts remain available after database reopen. See
  [implementation plan](07-feed-posts.md).

## Milestone 3 — Version 1.0 reader

### 8. Implement the article view
- Open selected posts in a scrollable reader.
- Display title, source, date, link, and feed-provided content.
- Render feed HTML appropriately for the terminal.
- Preserve list selection when returning from the reader.
- **Done:** Enter opens a full-screen reader for persisted feed-provided content;
  scrolling, terminal HTML/XHTML rendering, resizing, and Esc return preserve
  navigation. Open articles remain stable during refresh. See
  [implementation plan](08-article-view.md).

### 9. Retrieve and cache full website articles
- Automatically fetch a post's linked public HTML page when its full article is
  not cached; show the feed preview while retrieval/extraction runs.
- Extract the main readable content, convert it to Markdown, and render it with
  the reusable independent `reader` and `reader/view` packages.
- Persist extracted Markdown and metadata by post and source URL; reuse offline,
  reload explicitly, and preserve the last successful copy after failures.
- Add reader-specific reload/retry/feed-preview controls and safe async handling.
- **Done:** public HTML articles display in the styled Markdown reader, survive
  database reopen, and fall back to feed content when retrieval fails. JavaScript
  pages are deferred. See [implementation plan](09-full-article-reader.md).

## Milestone 4 — SSH access

### 10. Add the single-owner SSH server
- Configure the listen address, persistent host keys, and authorized SSH keys.
- All accepted keys access the same owner's feeds and posts.
- Connect sessions to the TUI; handle resizing and session cancellation.
- Remove the unused user table and username integration; feed data belongs to the
  single owner of the instance.
- Verify shared-database behavior across concurrent owner sessions.
- **Done:** the owner can manage feeds and read posts through authenticated SSH
  sessions. Sessions share one database and have independent navigation; host
  identity persists across restarts. See [implementation plan](10-ssh-server.md).

## Milestone 5 — Automatic refresh

### 11. Add server-level refresh scheduling
- Refresh independently of connected SSH sessions, at a configurable interval.
- Isolate individual feed failures and prevent overlapping fetches.
- Shut down cleanly and let active readers see newly stored posts.
- **Done:** configurable sequential sweeps run independently of SSH sessions,
  share per-feed operations, isolate failures, cancel cleanly, and notify connected
  readers after successful refreshes. See
  [implementation plan](11-refresh-scheduling.md).

## Milestone 6 — Version 1.0 release

### 12. Verify and document the complete workflow
- Verify fresh and existing databases, Atom/RSS ingestion, and deduplication.
- Exercise reading, SSH reconnects, concurrent sessions, and background refresh.
- Provide a documented Docker Compose deployment and document self-hosting,
  startup, authentication, configuration, and keybindings.
- Remove obsolete mocks and placeholders after replacements are verified.
- **Done:** the complete workflow is verified and documented, including a
  locally built Linux/amd64 Docker Compose deployment with persistent storage,
  authenticated SSH, and an opt-in end-to-end workflow. See
  [implementation plan](12-release-readiness.md).

## Deferred full-article capabilities

- JavaScript-rendered sites and browser runtime support.
- Authenticated/paywalled websites and credential management.
- Site-specific extraction adapters for pages that general readability extraction
  cannot handle.

## Execution rules

- Start with step 1, then proceed through the milestones in order.
- Numbered steps are plans, not commit boundaries. Before implementation, split
  each plan into small, cohesive, independently reviewable and verified changes.
  Each commit completes one such change, including relevant tests and
  documentation. Further split work that becomes too broad or combines independent
  behavior; keep every committed state compiling and passing required checks.
- Ask about unclear requirements rather than assuming behavior.
- Add meaningful verification for the behavior being changed.
- Before each commit, run:
  - `go test ./... -timeout 30s`
  - `go vet ./...`
  - `go test ./... -run '^$'`
- Run additional applicable checks, including sqlc regeneration for SQL changes,
  and review the intended diff.
- Use short, meaningful `[feat]:`, `[fix]:`, `[misc]:`, or `[refac]:` messages.
- Keep MEMORY.md current and within 60 lines; check AGENTS.md after every task.
- Make atomic implementation commits as work proceeds; separate commit
  authorization is not required. Obtain fresh, explicit authorization every time
  before pushing to a new remote branch and opening a PR targeting `main`.

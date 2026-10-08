# Atom1c roadmap

## Product and release scope

- A personal, self-hosted CLI feed aggregator and terminal reader accessed over SSH.
- One owner per instance, with multiple authorized SSH keys accessing the same data.
- Version 1.0 supports Atom and RSS 2.0; RSS 1.0/RDF is excluded.
- The 1.0 reader displays feed-provided content, including summaries when that is
  all the feed supplies.
- Full articles retrieved from websites belong to Phase 2, after 1.0.

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

## Milestone 4 — SSH access

### 9. Add the single-owner SSH server
- Configure the listen address, persistent host keys, and authorized SSH keys.
- All accepted keys access the same owner's feeds and posts.
- Connect sessions to the TUI; handle resizing and session cancellation.
- Review whether the existing users table still serves a purpose.
- Verify shared-database behavior across concurrent owner sessions.
- **Done:** the owner can manage feeds and read posts through SSH.

## Milestone 5 — Automatic refresh

### 10. Add server-level refresh scheduling
- Refresh independently of connected SSH sessions, at a configurable interval.
- Isolate individual feed failures and prevent overlapping fetches.
- Shut down cleanly and let active readers see newly stored posts.
- **Done:** feeds continue updating while no SSH client is connected.

## Milestone 6 — Version 1.0 release

### 11. Verify and document the complete workflow
- Verify fresh and existing databases, Atom/RSS ingestion, and deduplication.
- Exercise reading, SSH reconnects, concurrent sessions, and background refresh.
- Document self-hosting, startup, authentication, configuration, and keybindings.
- Remove obsolete mocks and placeholders after replacements are verified.
- **Done:** someone can follow the README to host and use their own instance.

## Phase 2 — Full-article reader (after 1.0)

- Retrieve full articles from their websites.
- Extract main article content and render it in the terminal reader.
- Fall back to feed-provided content when retrieval or extraction fails.
- Define detailed implementation steps and acceptance checks before this phase.

## Execution rules

- Start with step 1, then proceed through the milestones in order.
- Numbered steps are plans, not commit boundaries. Before implementation, split
  each plan into small, cohesive, independently reviewable and verified changes.
  Each authorized commit completes one such change, including relevant tests and
  documentation. Further split work that becomes too broad or combines independent
  behavior; keep every committed state compiling and passing required checks.
- Ask about unclear requirements rather than assuming behavior.
- Add meaningful verification for the behavior being changed.
- Before each authorized commit, run:
  - `go test ./... -timeout 30s`
  - `go vet ./...`
  - `go test ./... -run '^$'`
- Run additional applicable checks, including sqlc regeneration for SQL changes,
  and review the intended diff.
- Use short, meaningful `[feat]:`, `[fix]:`, `[misc]:`, or `[refac]:` messages.
- Keep MEMORY.md current and within 60 lines; check AGENTS.md after every task.
- Commit only with explicit authorization. Obtain fresh explicit authorization
  every time before pushing to a new remote branch and opening a PR to main.

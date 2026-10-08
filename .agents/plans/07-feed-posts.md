# Step 7 — Fetch feeds and display their posts

Status: implemented; persisted posts load asynchronously, explicit refresh updates
the open feed, and Atom/RSS refresh-and-reopen workflows pass.
Roadmap: [Step 7](roadmap.md#7-fetch-feeds-and-display-their-posts).

## Objective

Complete the real-data workflow: add a feed, explicitly refresh it, and open its
persisted posts without blocking the TUI. The existing feed refresh transaction
and post queries are reused; this step connects them to the application.

## Confirmed behavior

- `R` refreshes the selected feed from either pane. In the posts pane, the target
  is the feed whose posts are open.
- `r` retries the failed operation in the focused pane: a feed-page read, post
  read, or refresh.
- `tab` opens the selected feed and asynchronously loads its persisted posts.
  Moving the feed-list cursor alone does not change the open posts pane.
- Load all posts for the open feed, ordered by descending database ID. Retain the
  existing in-memory post filtering and pagination.
- Post cursor movement stops at page edges, matching feeds; `h`/`l` or left/right
  changes pages and selects the first item. The page indicator is in the header,
  toggled by `P` without changing container height.
- Opening posts reads SQLite; network fetching is explicit and uses `R`.
- Show loading, empty, and failure states in the TUI. Do not print operational
  messages over the interface.
- Keep modal and list-filter key ownership, so shortcut characters remain input
  while editing.
- Successful refresh reloads posts only when that feed is open. A refresh failure
  retries the refresh; a successful refresh followed by a post-read failure retries
  only the read. Cached posts remain visible on refresh failure.
- Keep user navigation responsive during database and network work.

## Implementation boundaries

The following are reviewable work boundaries, not an instruction to create a
commit for every heading. Refine a boundary if it becomes too broad or mixes
independent behavior. Every commit must leave the repository compiling and passing
the required checks; commits still require explicit authorization.

### 1. Correct roadmap commit guidance

- State in `AGENTS.md` that a roadmap item is a plan, not a commit boundary.
- Require plans to be decomposed into cohesive, independently reviewable and
  verified changes; each authorized commit completes one such change and includes
  relevant tests and docs.
- Update roadmap guidance and historical plan wording that says to commit a whole
  step as one atomic change.

### 2. Add a feed-scoped persisted-post boundary

- Add a context-aware post handler and a UI-facing store adapter around the
  existing `GetPostsByFeed` database query.
- Cover feed isolation, descending-ID order, empty results, unavailable state, and
  error/cancellation propagation using temporary SQLite.
- Do not regenerate or hand-edit generated SQL code unless a missing database
  capability is found.

### 3. Load posts asynchronously on feed open

- Remove production post mocks and initialize the pane with no records.
- `tab` captures the selected feed, focuses the posts pane, and issues an async
  load. Cursor movement in the feeds pane does not trigger a load.
- Use typed results and request generations; ignore stale results after opening a
  different feed or retrying.
- Clear a previous feed's entries when another feed is opened. Display an initial,
  loading, empty, and recoverable failure state.
- Retain in-memory filtering and pagination. Update help and README.

### 4. Add explicit asynchronous selected-feed refresh

- Add `R` and an async adapter over the existing transactional `feed.RefreshFeed`.
- Capture the target feed ID before launching work: selected feed in the feeds
  pane, open feed in the posts pane.
- Prevent duplicate simultaneous refreshes for one feed; bound operation context;
  show progress and completion/failure in the TUI.
- Keep `r` for retry and allow navigation while work is pending.
- Test with local HTTP servers and Atom/RSS fixtures, without external networking.

### 5. Reload the open feed after refresh and distinguish retries

- On successful refresh, reload only if that feed is still open. Stale reads or
  refreshes for other feeds must not replace the current posts.
- Preserve post selection by database ID when possible and preserve filtering.
- Keep cached posts visible when refresh fails.
- Distinguish refresh errors from follow-up read errors; `r` repeats only the
  operation that failed.
- Cover upserts, newly stored posts, deduplication, selection, failure/retry, and
  stale results.

### 6. Verify workflow and update status records

- Add an integration regression using temporary SQLite and local Atom/RSS HTTP
  servers: add, open empty posts, refresh, browse, refresh updated content, and
  reopen the database to read persisted posts.
- Manually exercise the interactive TUI, including retry and navigation during
  refresh.
- Update README, mark this plan and roadmap step complete after checks pass, and
  update `MEMORY.md` with remaining issues. Check whether `AGENTS.md` needs changes.

## Acceptance criteria

- Added feeds can be opened to show only their persisted posts.
- The `R` action fetches the correct feed from either pane and never blocks the UI.
- Posts show loading, empty, and recoverable error states; `r` retries the
  appropriate failed operation only.
- Refresh updates persisted entries transactionally and open posts reload on
  success.
- A failed refresh leaves cached posts visible; a failed follow-up read does not
  trigger another network request when retried.
- Stale async responses cannot overwrite a different open feed.
- Post list operations survive database reopen and use descending ID order.
- Full tests, vet, compile check, applicable SQL generation, and diff checks pass.

## Verification completed

- Handler tests cover feed isolation, descending ID order, empty results, missing
  database configuration, and canceled contexts.
- UI tests cover async open, feed cursor independence, stale results, selected-feed
  targeting from both panes, duplicate-refresh suppression, progress/completion and
  error states, refresh/read-specific retries, cached content, post selection, and
  filter retention.
- A temporary-SQLite workflow test uses local Atom and RSS HTTP servers to verify
  empty state, refresh/upsert/deduplication, loading updated entries, and database
  reopen persistence.
- A pseudo-terminal smoke check verified the application starts, renders, and
  exits cleanly; key routing, retries, and feed workflows are covered by UI tests.
- README keybindings and usage text document the implemented workflow.
- Full test, vet, network-free compile, and diff checks passed.

## Verification before each authorized commit

```sh
go test ./... -timeout 30s
go vet ./...
go test ./... -run '^$'
git diff --check
```

Run any other applicable checks and review the intended change. Use Go 1.26.5 or
newer. Commit messages use the short `[feat]:`, `[fix]:`, `[misc]:`, or `[refac]:`
prefixes. Commit only with explicit authorization. Publishing needs fresh explicit
authorization to push to a new remote branch and open a PR targeting `main`.

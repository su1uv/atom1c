# Step 6 — Connect feed management to SQLite

Status: implemented; SQLite-backed feed pagination/search and validated async
add-feed submission are covered by database, handler, and UI workflow tests.
Roadmap: [Step 6](roadmap.md#6-connect-feed-management-to-sqlite).

## Objective

Replace feed mocks with persisted records. Let the owner browse/search the full
feed collection, add feeds without blocking the TUI, and recover from database
errors without losing an in-progress draft.

## Confirmed behavior

- Feed results use deterministic `created_at, id` ordering and SQLite `LIMIT` /
  `OFFSET` pagination; page size follows available feed-pane height.
- `/` searches feed names across the whole database using a case-insensitive
  literal substring query. `%` and `_` are ordinary search characters.
- `h`/`l` and left/right navigate database pages in the feed pane. `P` toggles
  its page indicator. `r` retries a failed page or post-save reload.
- Add-feed submission trims values, requires a nonblank name and an absolute
  HTTP(S) URL, and relies on the unique URL constraint to detect duplicates.
- The modal stays open and disables interaction while saving. A failed insert
  restores editing and preserves the draft. A successful insert closes and clears
  the modal, clears search, and selects the new feed. A later reload failure is
  reported as “saved but not loaded”; retry does not repeat the insert.
- Bubble Tea commands carry request generations so stale page/search responses
  cannot replace newer UI state. Feed page rows and their matching count are read
  in one transaction.
- Post-list mocks remain until Step 7 connects feed selection to persisted posts.

## Storage and command interfaces

- Added sqlc queries for feed pages, matching counts, and the new feed's position
  in deterministic order. Removed the former fixed 20-row query and regenerated
  code with `sqlc generate`.
- Registered a deterministic SQLite `unicode_lower` scalar for full Unicode
  lowercasing; SQLite's built-in `lower()` only handles ASCII by default.
- Added context-aware handlers for page/count reads, inserts, and feed positioning.
  Duplicate URL constraint errors map to a sentinel used by the modal.
- Added a small feed-store interface at the UI boundary for deterministic async
  command and failure tests.

## Verification

- Database tests cover all pages beyond 20 records, tied timestamp ordering,
  global/case-insensitive search, literal wildcard characters, empty queries, and
  feed positions.
- Handler tests cover page/count results, invalid pagination, duplicate URLs,
  and filtered positions.
- UI tests cover pane-sized pagination, global search routing, stale responses,
  resize selection retention, validation, saving/error states, retry without a
  duplicate insert, and a temporary-SQLite add/reopen workflow.
- Required project checks and `git diff --check` passed.

## Commit and publication

Treat queries/generated code, handlers, UI commands, tests, and documentation as
one roadmap step. Suggested commit: `[feat]: connect feed management to SQLite`.

Commit only with explicit authorization. Publishing requires fresh explicit
authorization to push a new branch and open a PR targeting `main`.

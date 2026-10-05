# Short-term project memory

Current problems; update as work resolves them. This is a snapshot, not the roadmap.

## Missing functionality
- The running app is a local TUI; SSH access is an empty placeholder.
- Feed and post lists use mocks, not database data.
- Add-feed submission only closes the modal; database handlers are not wired in.
- Selecting a feed changes focus but does not load its posts.
- The scraper is not invoked by startup/UI; no automatic refresh exists.
- Fetched entries are not persisted; no posts schema or queries exist.
- No article-reading view exists.
- User records exist, but no SSH identity/session integration is implemented.

## Existing defects and limitations
- Add-feed inputs are not initially focused.
- Global `a` handling can interrupt list filtering.
- Pagination handling runs before pane focus checks and affects both lists.

## Verification baseline
- On 2026-10-05, `go test ./... -timeout 30s`, `go vet ./...`, and
  `go test ./... -run '^$'` passed. Feed tests use Atom/RSS fixtures and local
  HTTP servers, with no external network dependency.
- Repeated `sqlc generate` was stable during the timestamp work.
- The timestamp schema is still development-only and has not been deployed.
  Existing migrations already use TEXT with UTC second-precision CURRENT_TIMESTAMP
  defaults; no legacy-data backfill migration is needed.
- Database integration tests cover fresh-schema timestamp round trips, ordering,
  nullability, and persistence.
- UI behavior and end-to-end workflows remain untested.
- Passing checks do not establish functional completeness.

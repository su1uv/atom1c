# Short-term project memory

Current problems; update as work resolves them. This is a snapshot, not the roadmap.

## Missing functionality
- The running app is a local TUI; SSH access is an empty placeholder.
- Feed and post lists use mocks, not database data.
- Add-feed submission only closes the modal; database handlers are not wired in.
- Selecting a feed changes focus but does not load its posts.
- Feed refresh now persists posts, but is not invoked by startup/UI; no automatic
  refresh exists.
- No article-reading view exists.
- User records exist, but no SSH identity/session integration is implemented.

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
- Step 5 added root-owned UI key routing and shared list-pane behavior. UI model
  tests cover modal focus/draft lifecycle, filtering shortcuts, pane-scoped async
  results, pagination scope, and pane switching; broader end-to-end workflows
  remain untested.
- PR3 corrections verified RSS namespace isolation, common date variants, full
  XML consumption, and a 10 MiB decoded response cap (including streamed/gzip
  bodies); failures precede parsing and successful-fetch marking.
- Step 4 added transactional post upserts and feed-scoped identity; full tests, vet,
  compile check, sqlc generation, and diff check passed.
- Step 5 passed full tests, vet, compile check, and diff check.
- Passing checks do not establish functional completeness.

## Local tooling
- `.opencode/` and non-plan `.agents/` files are ignored and untracked; roadmap
  plans remain versioned. The formerly tracked review command is preserved locally.

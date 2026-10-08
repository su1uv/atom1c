# Short-term project memory

Current problems; update as work resolves them. This is a snapshot, not the roadmap.

## Missing functionality
- Owner confirmed feeds stay bounded; posts use page-edge cursor navigation and
  a header page indicator. Further overflow traced to Lipgloss v2 border-inclusive
  widths: long item text wrapped inside undersized borders. Both panes now budget
  borders separately; tests check long text and repeated page keys in both dimensions.
  100 `Layout Test Feed` records remain in local `atom1c.db` for visual checks.
- The running app is a local TUI; SSH access is an empty placeholder.
- Feed management now uses SQLite with async responsive pages, global name search,
  validated add, recoverable errors, and persistence across restart.
- Feed refresh is explicit from the UI; no automatic/background refresh exists.
- Step 8 provides a full-screen persisted article reader; the next roadmap item
  is step 9, the single-owner SSH server.
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
- Step 5 added root-owned UI key routing and shared list-pane behavior. UI tests
  cover modal focus/draft lifecycle, filtering shortcuts, pane-scoped async results,
  pagination scope, and pane switching.
- PR3 corrections verified RSS namespace isolation, common date variants, full
  XML consumption, and a 10 MiB decoded response cap (including streamed/gzip
  bodies); failures precede parsing and successful-fetch marking.
- Step 4 added transactional post upserts and feed-scoped identity; full tests, vet,
  compile check, sqlc generation, and diff check passed.
- Step 5 passed full tests, vet, compile check, and diff check.
- Step 6 added SQLite feed paging/global Unicode-insensitive search and validated
  async add/retry; its temporary-database reopen workflow passed. On 2026-10-07,
  full tests, vet, compile check, sqlc generation, and diff check passed.
- Step 7 opens persisted posts asynchronously, explicitly refreshes the selected or
  open feed, reloads updated posts, and scopes retries to the failed operation.
  Atom/RSS UI workflows use local HTTP servers and temporary SQLite; full tests,
  vet, compile check, and diff check passed on 2026-10-07.
- Passing checks do not establish functional completeness.
- Step 8 passed full tests, UI race tests, vet, compile, and diff checks on
  2026-10-08. Atom/RSS temporary-SQLite workflows cover reading, refresh snapshot
  isolation, reopening updated articles, and database reopen. A PTY smoke checked
  scroll/page/jump keys, resizing, return, and clean quit; review corrections pass.
- Reader rendering serializes/coalesces work and caches the immutable document;
  HTML/XHTML regressions cover whitespace, CDATA, indentation, and size growth.

## Local tooling
- `.opencode/` and non-plan `.agents/` files are ignored and untracked; roadmap
  plans remain versioned. The formerly tracked review command is preserved locally.
- Roadmap items are implementation plans, not commit boundaries; split work into
  small, independently reviewable commits per `AGENTS.md`.

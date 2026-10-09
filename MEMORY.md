# Short-term project memory

Current problems; update as work resolves them. This is a snapshot, not the roadmap.

## Missing functionality
- Owner confirmed feeds stay bounded; posts use page-edge cursor navigation and
  a header page indicator. Further overflow traced to Lipgloss v2 border-inclusive
  widths: long item text wrapped inside undersized borders. Both panes now budget
  borders separately; tests check long text and repeated page keys in both dimensions.
  100 `Layout Test Feed` records remain in local `atom1c.db` for visual checks.
- Feed management now uses SQLite with async responsive pages, global name search,
  validated add, recoverable errors, and persistence across restart.
- Server-level automatic refresh is implemented; manual refresh remains available.
- Steps 8–10 now provide the persisted full-screen and full-article reader plus
  public-key SSH access. JavaScript/authenticated websites remain deferred.
- Step 11 defaults to 15m (`ATOM1C_REFRESH_INTERVAL`; `0` disables) and runs
  immediate sequential sweeps; successful refreshes reload connected sessions.

## Verification baseline
- Historical steps 1–5 established timestamp, Atom/RSS parsing, transactional
  post persistence, and UI input routing; details remain in roadmap plans.
- Startup migrations are embedded. The timestamp schema has not been deployed;
  existing source migrations use UTC second-precision TEXT timestamps.
- Step 6 added SQLite feed paging/global Unicode-insensitive search and validated
  async add/retry; its temporary-database reopen workflow passed. On 2026-10-07,
  full tests, vet, compile check, sqlc generation, and diff check passed.
- Step 7 opens persisted posts asynchronously, explicitly refreshes the selected or
  open feed, reloads updated posts, and scopes retries to the failed operation.
  Atom/RSS UI workflows use local HTTP servers and temporary SQLite; full tests,
  vet, compile check, and diff check passed on 2026-10-07.
- Step 8 passed full tests, UI race tests, vet, compile, and diff checks on
  2026-10-08. Atom/RSS temporary-SQLite workflows cover reading, refresh snapshot
  isolation, reopening updated articles, and database reopen. A PTY smoke checked
  scroll/page/jump keys, resizing, return, and clean quit; review corrections pass.
- Reader rendering serializes/coalesces work and caches the immutable document;
  HTML/XHTML regressions cover whitespace, CDATA, indentation, and size growth.
- Step 9 delivers independent `reader`/`reader/view` packages, public HTML
  extraction, SSRF-aware bounded HTTP, Markdown presentation, and SQLite caching.
  Full tests, UI race tests, vet, compile, sqlc, PTY smoke, and review passed on
  2026-10-08.
- Step 10 adds Wish SSH-only startup, owner public-key authentication, persistent
  Ed25519 host identity, PTY TUI sessions with resize/cancellation, shared SQLite,
  and graceful process shutdown. Local SSH integration adds a feed through the
  TUI; config, authorization, session cancellation, two-session database reload,
  concurrent-shell isolation, incomplete-handshake shutdown, and fresh-schema
  removal of unused users are covered by tests. Full tests, SSH/UI/feed/database
  race tests, vet, compile check, sqlc generation, and diff checks passed on
  2026-10-09.
- Step 11 verification on 2026-10-09: full tests, vet, compile, sqlc, and targeted
  race checks passed; temporary SQLite/local HTTP cover no-client scheduling,
  cancellation, persistence, and manual/scheduled multi-session reloads.
- Step 12 adds a non-root Linux/amd64 Compose deployment. Executable and isolated
  container workflows verify restart/persistence, Atom/RSS, SSH sessions, scheduled
  and canceled refresh, backup/restore, and cleanup; required checks pass.

## Local tooling
- `.opencode/` and non-plan `.agents/` files are ignored and untracked; roadmap
  plans remain versioned. The formerly tracked review command is preserved locally.
- Roadmap items are implementation plans, not commit boundaries; split work into
  small, independently reviewable commits per `AGENTS.md`.

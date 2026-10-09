# Step 11 — Server-level refresh scheduling

Status: implemented and verified. See
[roadmap step 11](roadmap.md#11-add-server-level-refresh-scheduling).

## Confirmed behavior

- `ATOM1C_REFRESH_INTERVAL` accepts Go duration syntax, defaults to `15m`, and
  accepts `0` to disable scheduled refresh. Negative and malformed values fail
  startup. Manual refresh remains available when scheduling is disabled.
- Run a sweep immediately after startup, then wait the full configured interval
  after that sweep completes. Never run overlapping sweeps.
- Each sweep snapshots all feeds in deterministic order and refreshes them
  sequentially. A failed feed is logged and does not prevent attempts for later
  feeds. A sweep-level enumeration error is logged and retried next interval.
- Feeds added during server operation are included in the next sweep.
- Concurrent requests for one feed share one in-flight fetch/result. Each caller
  may stop waiting independently; cancel the fetch when its last caller leaves.
  Server shutdown cancels in-flight work. Scheduled work is independent of SSH
  sessions.
- Keep the existing 15-second overall refresh operation limit and 15-second feed
  HTTP timeout.
- Publish successful refresh notifications only after the database transaction
  commits. All connected sessions automatically reload the matching open feed;
  preserve post selection/filter and the open article snapshot. Manual refreshes
  use the same notification behavior.
- Cancel refresh work and wait for it to clean up before closing SQLite.

## Design

- Add a refresh-coordination interface to `internal.State` so UI and scheduler
  callers use the same per-feed coordinator without an import cycle.
- Implement coordination alongside transactional feed refresh in `internal/feed`.
  Hide active-request sharing, caller tracking, timeout/cancellation, and success
  subscriptions behind that interface.
- Keep scheduler lifetime owned by `main`, under the same process context as SSH.
  Stop the scheduler, cancel the coordinator, and wait for cleanup before returning
  to deferred database closure.
- Enumerate a sweep's feed snapshot with a dedicated sqlc query. Do not repeatedly
  use `GetNextFeedToFetch`: a failing feed remains oldest by successful-fetch time
  and could otherwise prevent other feeds from being attempted.
- Use coalescing success subscriptions so each session can receive updates without
  blocking persistence or accumulating an unbounded event queue. Each subscription
  tracks only the session's currently open feed.

## Independently reviewable increments

Each increment must compile and pass required checks before commit. Split further
when a change becomes broad or contains independently useful behavior.

1. **Commit guidance and plan.** Update `AGENTS.md`, `MEMORY.md`, roadmap, and
   historical plan instructions so atomic implementation commits need no separate
   authorization while each push and PR still requires fresh explicit authorization.
   Add this implementation plan.
2. **Configuration.** Add a testable parser for `ATOM1C_REFRESH_INTERVAL`; test
   unset, valid positive durations, zero, malformed values, and negatives. Document
   the setting and scheduling semantics when startup wiring is added.
3. **Shared per-feed refresh coordination.** Route manual UI refreshes and later
   scheduler calls through one coordinator. Test request sharing, result/error
   propagation, different-feed independence, independent caller cancellation,
   last-caller cancellation, shutdown cancellation, cleanup, and transactional
   persistence. Keep the 15-second operation and HTTP limits.
4. **Sequential periodic sweeps.** Add a dedicated deterministic feed-enumeration
   query and regenerate sqlc output. Test immediate first sweep, post-completion
   interval, no overlap, sequential order, per-feed failure isolation, empty feeds,
   disabled scheduling, enumeration failure/recovery, and newly added feeds on the
   next sweep. Use controllable timing/synchronization, not long sleeps.
5. **Server lifecycle.** Start the scheduler independently of SSH sessions under
   the process context. Test no-client persistence, signal/SSH-error cancellation,
   canceled HTTP work, and wait-before-database-close ordering.
6. **Connected session reload.** Subscribe for each session lifetime and reload
   only the matching open feed after a successful commit. Preserve selection,
   filter, and article snapshot; ignore stale or unrelated results; coalesce events;
   retain retry behavior after reload errors. Test two sessions and both manual and
   scheduled notifications, including disconnected/slow subscribers.
7. **End-to-end verification and docs.** Verify Atom/RSS local-server workflows,
   deduplication, failure isolation, concurrent requests, session reloads, restart
   persistence, configuration, and shutdown. Update README, this verification
   record, roadmap status, and concise `MEMORY.md`.

## Verification before each commit

```sh
go test ./... -timeout 30s
go vet ./...
go test ./... -run '^$'
git diff --check
```

Use Go 1.26.5 or newer. Write and run meaningful failing behavioral tests before
implementation; use table-driven tests where they fit. Use temporary SQLite,
Atom/RSS fixtures, and local HTTP servers. Run targeted race tests for refresh,
database, SSH, and UI paths. Run `sqlc generate` after query changes and review all
generated diffs; do not edit generated database files manually.

Before each atomic commit, inspect the intended diff and run required checks. Use
`[feat]:`, `[fix]:`, `[misc]:`, or `[refac]:` prefixes. Local implementation commits
need no separate authorization. Every push and PR requires fresh explicit
authorization, uses a new remote branch, and targets `main`.

## Verification completed

- The table-driven interval parser tests cover default, positive duration, zero,
  empty, malformed input, negative input, and nil environment lookup. Full tests, vet,
  network-free compile, and diff checks passed with Go 1.27.1 on 2026-10-09.
- Coordinator tests cover same-feed sharing and result propagation, different-feed
  independence, independent and last-caller cancellation, shutdown cancellation and
  waiting, success-only notifications for the watched feed, coalescing repeated
  notifications, watch changes, and persistence before notification.
  Full tests, vet, network-free compile, diff checks, and 10 targeted race-test runs
  passed with Go 1.27.1 on 2026-10-09.
- The refresh feed query returns a complete, deterministically ordered snapshot.
  Scheduler tests cover immediate/sequential sweeps, per-feed failure isolation,
  enumeration failure logging/retry, updated feed snapshots on later sweeps, empty
  feeds, disable behavior, cancellation during a sweep, and URL credential redaction
  in failure logs. `sqlc generate`, full tests,
  vet, network-free compile, diff checks, and 10 targeted race-test runs passed
  with Go 1.27.1 on 2026-10-09.
- Server lifecycle tests verify a startup refresh with no SSH clients, manual
  refresh when scheduling is disabled, cancellation of in-flight HTTP refreshes,
  and database usability after cleanup. README and `.env.example` document the
  interval and sweep behavior. Full tests, vet, network-free compile, diff checks,
  and 10 targeted race-test runs passed with Go 1.27.1 on 2026-10-09.
- Temporary-SQLite/local-HTTP UI workflows verify that both connected models reload
  after manual and scheduled refreshes through one shared coordinator. Unit tests
  verify selection/filter and article-snapshot preservation, ignore unrelated feeds,
  and subscribe each model independently. Full tests, vet, network-free compile,
  diff checks, and 10 targeted race-test runs passed with Go 1.27.1 on 2026-10-09.
- All seven increments and the complete Step 11 workflow are implemented; all
  required checks pass.

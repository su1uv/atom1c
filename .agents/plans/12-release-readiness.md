# Step 12 — Verify and document the complete workflow

Status: in progress.

## Confirmed scope

- The project has not been deployed; verify fresh databases and reopening a
  database created by the current schema. No older deployed schema is an upgrade
  contract for this step.
- Deliver a locally built Docker Compose deployment, verified on Linux amd64.
- Run the container as a non-root account named `atom1c`; SSH login is
  `atom1c@host`.
- Persist SQLite and the SSH host key in a named volume. Mount a dedicated
  `authorized_keys` file read-only.
- Publish SSH on host loopback by default and document enabling remote access.
- Finish at a verified, documented release-ready state. Publishing a release is
  outside this step.

## Acceptance criteria

- Fresh process/container startup applies embedded migrations and serves SSH;
  repeated startup against the same storage retains feeds, posts, article cache,
  and SSH host identity.
- Deterministic Atom and RSS 2.0 workflows cover add, fetch, deduplication,
  updates, persistence, and reading.
- SSH verification covers authorized access, reconnect, PTY resize, concurrent
  independent sessions sharing data, refresh notifications, and graceful stop.
- Scheduled refresh works without connected clients; individual feed failures do
  not abort later feeds; disabled scheduling keeps manual refresh available.
- Docker Compose builds locally, runs as the configured non-root user, persists
  storage, and stops/restarts cleanly.
- README instructions work from clean storage and accurately describe setup,
  authentication, configuration, controls, capabilities, and limitations.
- Remove obsolete production mocks/placeholders only when their replacements are
  verified; retain useful test doubles.

## Independently reviewable increments

Each increment is a commit candidate, not a requirement to bundle all work into
one commit. Split broad increments further. Every implementation commit includes
its relevant verification and documentation, compiles, and passes required checks.

1. **Plan and acceptance matrix.** Link this plan from the roadmap and record
   existing test coverage and remaining integration evidence here.
2. **Executable startup/restart verification.** Exercise the built application
   with temporary storage and SSH keys: fresh embedded migrations, service startup,
   persisted state after process restart, and clean cancellation of active work.
   Add focused tests for uncovered behavior; use local fixtures, bounded waits, and
   no live network.
3. **Container image.** Add a multi-stage Dockerfile and `.dockerignore`. Use a
   Go version at least 1.26.5; run the binary in a minimal certificate-equipped
   runtime as non-root `atom1c`; configure `/data` for database and host key
   persistence and the SSH listener for container networking. Exclude local
   secrets, databases, Git and agent tooling from build context.
4. **Compose deployment.** Add a source-built Compose service with named `/data`
   volume, read-only dedicated authorized-keys mount, loopback-only published
   port by default, explicit refresh configuration, restart policy, and a stop
   grace period. Verify empty-volume ownership and useful errors for missing key
   files. Document remote access configuration.
5. **Container workflow verification.** Add a repeatable opt-in workflow isolated
   by Compose project name and temporary credentials/storage. Build and start,
   authenticate with an interactive PTY, exercise Atom/RSS ingestion and
   deduplication, verify concurrent sessions and no-client refresh persistence,
   recreate while retaining the volume, and stop during active work. Use local
   fixture services; keep Docker requirements separate from `go test ./...`.
6. **Hosting and usage documentation.** Rewrite README as an executable Compose
   walkthrough and document native Go startup, remote binding, configuration,
   storage/host-key persistence, logs, shutdown, rebuild, backup/restore, reader
   features, keybindings, and limitations. Fix contradictory website-fetch
   statements and missing reader controls. Audit obsolete mocks/placeholders.
7. **Release-readiness record.** Follow the README from clean storage, record
   acceptance evidence and platform/tool versions here, update roadmap status and
   concise `MEMORY.md`, and check whether `AGENTS.md` needs a lean Docker note.
   Request independent review of the completed deployment/integration work.

## Verification before each commit

```sh
go test ./... -timeout 30s
go vet ./...
go test ./... -run '^$'
git diff --check
```

Additionally run targeted race checks for changed lifecycle, SSH, refresh,
database, or UI paths; validate Compose and the image; and run the opt-in container
workflow where applicable. Use local fixtures and temporary storage. For any SQL
change, edit source SQL, run `sqlc generate`, and review generated output. Inspect
the intended diff before every atomic commit; use `[feat]:`, `[fix]:`, `[misc]:`,
or `[refac]:` prefixes. Local commits need no separate authorization; every push
and PR requires fresh explicit authorization, a new remote branch, and target
`main`.

## Verification completed

- Plan scope confirmed: undeployed project; current-schema reopen coverage;
  Docker Compose; Linux amd64; non-root `atom1c`; dedicated read-only key mount;
  named data volume; loopback host binding; release-ready only.
- A built-executable integration test verifies fresh embedded migrations, SSH
  authentication, persisted feed/post/article cache, retained host identity and
  permissions across restart, scheduled refresh cancellation on SIGINT, and
  database usability afterward. It passes under the race detector with repeated
  runs; full tests, vet, compile-only tests, and diff checks passed.

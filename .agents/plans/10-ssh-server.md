# Step 10 — Single-owner SSH server

Status: implemented; public-key SSH access, interactive Bubble Tea sessions,
shared SQLite state, host-key persistence, and shutdown are verified.
Roadmap: [Step 10](roadmap.md#10-add-the-single-owner-ssh-server).

## Confirmed behavior

- SSH server only; default listen address `127.0.0.1:23234`.
- Environment configuration: `ATOM1C_SSH_ADDR`, `ATOM1C_AUTHORIZED_KEYS`, and
  `ATOM1C_SSH_HOST_KEY`; `.env` is loaded at startup. `GOOSE_DBSTRING` remains the
  only database setting.
- Authenticate public keys listed in the hosting account's
  `~/.ssh/authorized_keys`; SSH username must match that operating-system account.
  Load the file at startup; changes require restart. Reject key lines with
  OpenSSH options and certificate entries rather than silently ignoring
  restrictions or accepting unvalidated certificates. Disable password
  authentication.
- Generate an Ed25519 host key if missing and retain it at
  `$XDG_DATA_HOME/atom1c/ssh_host_ed25519_key`, falling back to
  `~/.local/share/atom1c/ssh_host_ed25519_key`. Create key directories/files with
  owner-only permissions, and reject malformed, non-Ed25519, or exposed existing
  private keys without overwriting them.
- Require an interactive shell and PTY. Reject exec, SFTP/subsystems, and port
  forwarding. `q`/Ctrl+C closes only that session. SIGINT/SIGTERM stops accepting
  connections, closes accepted sockets including in-progress handshakes, cancels
  sessions and their operations, then closes SQLite. Authentication handshakes
  have a 10-second timeout.
- Each session has independent navigation and reader state over shared SQLite.
  Persist changes immediately; other sessions see them on a subsequent data load
  or reconnect. No live broadcast or user table ownership model.
- The `users` table/query and `CurrentUsername` were unused and not deployed.
  Replace migration 1 with a no-op preserving its version for existing development
  databases; fresh databases do not create a users table. Preserve existing feed,
  post, and article-cache data.
- Serialize SQLite connections in the server process and configure a busy timeout
  to prevent concurrent sessions from contending on write locks. Keep HTTP work
  outside database transactions.

## Independently reviewable increments

1. **Configuration and key provisioning.** Add validated settings, owner identity,
   strict authorized-key parsing, and safe host-key creation/loading. Table-driven
   tests cover defaults/overrides, malformed or empty key files, unsupported
   options, wrong usernames/keys, host-key reuse, permissions, and corrupt-key
   preservation.
2. **Session-scoped UI lifecycle.** Create one model per SSH session and derive
   database, feed-refresh, article-cache, and website-fetch contexts from the
   session lifetime. Tests cancel pending database/fetch operations.
3. **Wish server and TUI adapter.** Integrate Wish v2.0.3 and Bubble Tea v2 session
   I/O, PTY dimensions/resizing, public-key authentication, shell-only request
   policy, and session-local quit. Local SSH integration tests cover login, denied
   usernames, PTY enforcement, exec rejection, and interactive quit.
4. **Shared persistence and user cleanup.** Remove unused user SQL/schema/code,
   regenerate sqlc, and verify independent sessions observe persisted changes on
   reload with a shared temporary SQLite database.
5. **Startup and shutdown.** Replace the local-TUI entry point with SSH-only
   startup, embedded migrations, signal handling, connection serialization, and
   session cancellation before database close. Test cancellation and stopped
   listening.
6. **End-to-end/documentation.** Document self-hosting, authentication,
   configuration, and session behavior. Correct the stale full-article release
   scope. Verify Atom/RSS management, reading, restart/cache persistence, resizing,
   concurrent sessions, and interactive SSH behavior.

These increments are independently reviewable units, not prescribed commit
boundaries. Split further when useful; commit only with explicit authorization.

## Verification

- Use Go 1.26.5 or newer; Wish v2.0.3 is compatible with the current Charm v2
  versions and Go requirement.
- Meaningful behavior tests precede implementation. Use local TCP/SSH clients,
  temporary SQLite, table-driven configuration cases, and no external network.
- Run full tests, targeted race tests for SSH/UI/feed workflows, `go vet ./...`,
  `go test ./... -run '^$'`, and `git diff --check`.
- Run `sqlc generate` after SQL changes and review generated output. Never hand-edit
  generated database code.

## Verification completed

- SSH tests verify config defaults/overrides, parsing/comment/option/error cases,
  Ed25519 host key creation/reuse/permissions and preservation of invalid keys.
- A local SSH client verifies owner key and username authentication, rejects
  sessions without PTYs, exec commands, SFTP, and local/reverse forwarding; it adds
  a feed through a Bubble Tea session, exercises two concurrent shells, and closes
  one session with `q` while the other remains active.
- UI tests verify session cancellation reaches feed reads and article fetching, and
  separate sessions see shared SQLite changes after the next feed load.
- An SSH channel-disconnect integration test verifies a pending local HTTP feed
  refresh is canceled without marking the feed fetched while a second shell on the
  same connection remains active. Shutdown tests an accepted but incomplete SSH
  handshake. Authentication rejects certificate entries. A PTY resize is sent
  during the end-to-end add-feed workflow.
- Fresh migration tests verify the unused users table is absent; timestamp and
  existing feed/post/article schema checks continue to pass.
- On 2026-10-09 with Go 1.27.1, `go test ./... -timeout 30s`, `go vet ./...`,
  `go test ./... -run '^$'`, race tests for SSH/UI/feed/database, `sqlc generate`,
  and `git diff --check` passed. SSH disconnect and shutdown race tests also passed
  across 20 repetitions.

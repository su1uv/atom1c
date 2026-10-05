# Step 4 — Persist fetched posts

Status: implemented; posts are upserted transactionally with successful-fetch
timestamps, and required checks pass.
Roadmap: [Step 4](roadmap.md#4-persist-fetched-posts).

## Objective

Persist normalized Atom and RSS entries in SQLite, deduplicate repeated fetches,
and update stored values when feed-provided fields change. Commit all post changes
and the successful-fetch timestamp together.

## Confirmed identity and update rules

- Identify entries within one feed by non-whitespace source ID first, then by
  non-whitespace link. Preserve usable source values verbatim.
- Reject an entry with neither identity value; no changes from that refresh may
  commit.
- Prefix identity keys with `id:` or `link:` and enforce uniqueness per feed.
- Feed-provided fields replace prior fields on an existing identity, including
  values that become empty or absent. Preserve the post's database ID and
  creation time.
- Process duplicate identities in document order; the last occurrence wins.
- Keep previously stored posts that are absent from later fetches.
- A source-ID change or link-only entry gaining an ID is a new identity.

## Storage and query interface

- Added `sql/schema/0003_posts.sql` with a STRICT posts table, feed foreign key,
  feed-scoped identity constraint, bookkeeping timestamps, source ID and RSS GUID
  metadata, reader fields, and raw/normalized source dates.
- Foreign-key enforcement is enabled on application and test connections with
  modernc SQLite's `_pragma=foreign_keys(1)` DSN parameter; refresh tests verify
  the active connection setting and schema tests reject missing parent feeds.
- Timestamps use UTC TEXT at second precision, `YYYY-MM-DD HH:MM:SS`. Raw source
  date text is preserved; missing or unparseable normalized values remain NULL.
- Added sqlc queries for upserting posts, listing a feed's posts in descending
  database-ID order, and reading a post by ID. Generated code is committed from
  `sqlc generate`, not hand-edited.

## Refresh behavior

- `feed.RefreshFeed` fetches and parses before starting a transaction, validates
  every identity, upserts entries in document order, and marks the feed fetched
  through the same transaction.
- Refreshes for one feed are serialized within the application process so an
  older in-flight response cannot overwrite a later response. Waiting callers
  can cancel through their context.
- Any fetch, parse, validation, upsert, fetch-mark, or commit failure returns an
  error and cannot partially persist posts or advance the fetch timestamp.
- Empty feeds are successful and update their fetch timestamp.
- `ScrapeFeeds` still selects the next feed, then calls the transactional refresh.
  Application state exposes the SQL handle for transaction creation.

## Verification completed

- Added database and HTTP-backed tests for schema constraints, Atom/RSS metadata,
  ID precedence and link fallback, verbatim RSS identifiers with whitespace-
  differing GUIDs/links, whitespace/missing identity, duplicate identities,
  empty-field and invalid-date replacement, retained missing entries, empty feeds,
  persistence after database reopen, and database failure rollback.
- Added coverage for per-feed refresh serialization and cancellation while
  waiting, and asserted normalized entry update dates.
- Added checks that HTTP, malformed XML, canceled contexts, and invalid identities
  do not create posts or mark feeds fetched.
- Ran `sqlc generate` repeatedly and confirmed stable generated output.
- Passed `go test ./... -timeout 30s`.
- Passed `go vet ./...`.
- Passed `go test ./... -run '^$'`.
- Passed `git diff --check`.
- Checked `AGENTS.md`; existing migration, verification, and testing guidance
  remains applicable.

## Commit and publication

Treat the migration, SQL queries/generated code, refresh logic, tests, and records
as one atomic roadmap step. Suggested commit: `[feat]: persist fetched posts`.

Commit only with explicit authorization. Publishing requires fresh explicit
authorization to push a new branch and open a PR targeting `main`.

# Step 1 — Reconcile database timestamps

Status: completed; second-level precision implemented and verified.
Roadmap: [Step 1](roadmap.md#1-reconcile-database-timestamps).

## Objective

Make SQLite timestamp storage, sqlc-generated types, and Go callers consistent.
Verify fresh and existing databases, including timestamp scanning and ordering.

## Original mismatch (resolved)

- Users and feeds declare timestamps as SQLite TEXT, while the original generated
  models and callers used time.Time/sql.NullTime.
- The existing database contained Go-formatted local timestamps with offsets,
  fractional seconds, and monotonic suffixes.

## Timestamp convention

- Retain SQLite TEXT storage.
- Use string for required timestamps and sql.NullString for nullable timestamps.
- Store UTC timestamps as `YYYY-MM-DD HH:MM:SS` with second-level precision.
- Let SQL assign bookkeeping timestamps, matching CURRENT_TIMESTAMP defaults.
- Preserve NULL last_fetched_at as “never fetched.”
- Article publication dates are outside this step and will be addressed with posts.

## Implementation checklist

### A. Inspect compatibility

- [x] Check the installed sqlc version (v1.31.1).
- [x] Inspect existing table definitions and timestamp formats read-only.
- [x] Identify SQLite-default and Go-formatted values in the existing database.
- [x] Confirm the generated-type/schema mismatch and choose a compatible format.

### B. Align SQL timestamp handling

- [x] Use database defaults for feed creation timestamps.
- [x] Assign fetch/update timestamps consistently in the fetch-update query.
- [x] Preserve NULL fetch timestamps.
- [x] Ensure chronological ordering works under the canonical representation.
- [x] Add a new migration with verified conversion behavior for legacy values;
      previously applied migrations were not rewritten.
- [x] Preserve identifiers and all non-timestamp data.
- [x] Fail migration on unrecognized timestamp formats instead of silently
      retaining mixed representations.

### C. Regenerate and update callers

- [x] Run `sqlc generate` and review generated types and query parameters.
- [x] Update internal/handlers/handle_feeds.go and internal/atom/atom.go.
- [x] Remove obsolete timestamp parameters and imports.
- [x] Regenerate rather than hand-edit generated internal/database files.

### D. Add database integration tests

Use temporary SQLite databases and production migrations. Verify:

- [x] Feed creation returns readable canonical timestamps.
- [x] Feed listing scans timestamps successfully.
- [x] Newly created feeds have NULL last_fetched_at.
- [x] Marking a feed fetched stores readable fetch/update timestamps.
- [x] Fetch selection prioritizes never-fetched feeds, then oldest fetched feeds.
- [x] User lookup reads timestamps correctly.
- [x] Closing and reopening the database preserves values.
- [x] Compatibility migration handles representative legacy values without
      losing records, including relevant timezone and fractional-second cases.
- [x] Unknown formats cause a migration failure and rollback without data loss.

Avoid relying on sleeps or assuming second-level timestamps are unique.

### E. Run checks

- [x] `sqlc generate`
- [x] Confirm repeated generation produces no additional changes.
- [x] `go test ./... -timeout 30s`
- [x] `go vet ./...`
- [x] `go test ./... -run '^$'`
- [x] `git diff --check`
- [x] Review the intended diff and applicable checks.

### F. Update memory and guidance

- [x] Remove the resolved mismatch from MEMORY.md, keeping it within 60 lines.
- [x] Record remaining unrelated issues in MEMORY.md.
- [x] Update AGENTS.md to document migration 0003 and remove obsolete mismatch guidance.

## Acceptance criteria

- [x] Regeneration produces compiling code without manual corrections.
- [x] Feed and user timestamp reads work against SQLite.
- [x] Nullability and chronological fetch ordering behave correctly.
- [x] Existing records remain usable after migration.
- [x] All required checks pass.

## Commit and publication

The SQL changes, generated code, caller updates, tests, and memory/guidance updates
form one atomic fix. Commit only when explicitly authorized.

Suggested message: `[fix]: reconcile SQLite timestamps`.

Publishing requires fresh explicit authorization to push a new remote branch and
open a PR targeting main, following AGENTS.md.

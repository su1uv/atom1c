# Repository guidance

## Purpose
- Atom1c is a self-hosted CLI Atom/RSS aggregator with an included reader,
  accessed over SSH. The intended product is not a local-only application.
- Each user self-hosts their own instance for personal use, including the author.

## Working agreements and memory
- Read `MEMORY.md` as the agent's short-term memory of current project problems.
  Keep it concise (at most 60 lines) and update it as problems change or resolve.
- Use TDD whenever applicable: write and run a meaningful failing test before
  implementing the behavior, then make it pass and refactor. UI-only styling and
  visual presentation tasks are exempt.
- Use table-driven tests whenever they make sense for the behavior being tested.
- After every completed task, check whether `AGENTS.md` needs updating and update
  it when necessary. Keep guidance lean; structure is expected to change often.
- Ask questions when requirements are unclear; do not make assumptions.

## Commit and publication rules
- Only atomic commits, with short, meaningful messages using one of these prefixes:
  `[feat]: ...`, `[fix]: ...`, `[misc]: ...`, `[refac]: ...`.
- For multi-step plans, make one atomic commit per step.
- Before each commit: tests must pass, run `go vet ./...`, verify compilation,
  and run any other checks applicable to the changes. Review the intended diff.
- Commit only when explicitly requested or authorized.
- Publishing must ALWAYS push to a new remote branch and open a PR targeting
  `main`. Obtain prior, explicit authorization for the push and PR every time;
  never infer it from earlier authorization or push directly to `main`.

## Run and verify
- Use Go 1.26.5 or newer, as required by `go.mod`.
- Run from the root: `GOOSE_DBSTRING=./atom1c.db go run .`; startup also loads `.env`.
  The TUI requires an interactive terminal.
- Startup automatically applies embedded Goose migrations. Only `GOOSE_DBSTRING`
  configures the database; other Goose variables in `.env.example` are unused.
- Tests: `go test ./... -timeout 30s`; feed tests use Atom/RSS fixtures and local
  HTTP servers.
- Network-free compile check: `go test ./... -run '^$'`; static check: `go vet ./...`.

## Implementation guidance
- Use Charm v2 APIs (`charm.land/...`, `tea.KeyPressMsg`, `View() tea.View`).
- SQL sources live in `sql/schema` and `sql/queries`; regenerate with `sqlc generate`.
  Do not hand-edit generated `internal/database` files. Review generated diffs
  before regeneration. Before the first deployment, update existing migrations
  directly when needed. After deployment, preserve applied migrations and add a
  new migration for compatibility changes.
- Migrations are embedded at build time; rebuild/re-run after changing SQL.

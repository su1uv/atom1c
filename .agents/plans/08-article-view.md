# Step 8 — Article view

Status: implemented; full-screen persisted article reading, responsive reflow,
snapshot isolation, and Atom/RSS refresh/reopen workflows verified.
Roadmap: [Step 8](roadmap.md#8-implement-the-article-view).

## Confirmed behavior

- Enter opens the selected persisted post in a full-screen reader; Esc returns
  with posts focus, filter, selection, and page preserved.
- j/k and arrows scroll; PgUp/PgDown page; Home/End jump; q/Ctrl+C quit.
- Show title, saved feed name, publication date, link, and feed-provided content.
- Dates use publication time in UTC, then raw publication date, otherwise Unknown.
- Render HTML/XHTML headings, lists, quotes, code, and link destinations; images
  become text labels. Maintained rendering/parser dependencies are permitted.
- Blank content shows No feed-provided content. Unsupported types show a notice.
- Keep an immutable article snapshot during an already-running refresh. Update
  underlying posts normally; reopening reads the updated records.
- Reader keys own input; preserve existing modal/filter key ownership.

## Independently reviewable increments

1. Retain complete loaded posts indexed by ID and create immutable snapshots.
   Test accepted/stale results, feed switching, empty selection, and snapshots.
2. Select maintained, compatible dependencies and implement a small content
   rendering interface. Table-test text, HTML/XHTML structure, metadata fallbacks,
   malformed markup, Unicode, and long lines.
3. Integrate a Charm v2 full-screen viewport, reader-specific keys/help, async
   rendering with request generations, and return-state preservation. Test
   routing, stale results, scrolling, and refresh completion during reading.
4. Reflow on resize, preserve reading position where possible, and clamp offsets.
   Test terminal dimensions and persisted Atom/RSS read/refresh/reopen workflows;
   manually exercise the interactive TUI.
5. Update README, roadmap, this plan, and concise MEMORY.md after verification.
   Check whether AGENTS.md needs updating.

These are work increments, not fixed commit boundaries. Split further when needed;
each authorized atomic commit must compile and pass applicable checks and include
its tests/documentation. Implementation commits are authorized on one new local
step-8 branch. Publishing still requires fresh authorization.

## Verification

Use Go 1.26.5 or newer. Write and run meaningful failing behavioral tests first;
UI-only styling is exempt. Use temporary SQLite and local HTTP fixtures, not
external network tests. Budget Lipgloss v2 border-inclusive dimensions.

Before every authorized commit, review the diff and run:

```sh
go test ./... -timeout 30s
go vet ./...
go test ./... -run '^$'
git diff --check
```

Review dependency changes; regenerate SQL only if SQL changes are needed, never
hand-edit generated database files. Use short repository commit prefixes.

## Acceptance criteria

- Persisted Atom/RSS articles display their supplied content in the reader.
- Scrolling and resizing stay within the terminal, including long metadata/text.
- Returning preserves list navigation; async results cannot replace the snapshot
  or reopen a closed reader.
- Reopening after refresh or database reopen shows persisted current content.
- Required checks, workflow verification, and code review pass.

## Verification completed

- Behavioral tests were written and run failing before snapshot retention,
  rendering, reader routing, and review-driven corrections were implemented.
- Table-driven rendering tests cover text, HTML/XHTML, CDATA, malformed markup,
  metadata fallbacks, blank/unsupported content, lists, inline whitespace, code
  indentation, terminal controls, Unicode, and wide/long text.
- Regression tests prevent multiline styling allocation expansion and excessive
  nested quote expansion; rendering output is bounded and deep markup simplified.
- Reader tests cover selection/filter/page retention, keyboard ownership, scroll
  limits, quit, stale results, narrow terminals, repeated resize, and navigation
  during initial rendering/reflow. A blocked-render test verifies serialized,
  coalesced work and cached width-independent documents.
- Temporary-SQLite/local-HTTP Atom and RSS workflows verify reading persisted
  content, stable snapshots during refresh, updated content on reopening, and
  reading after database reopen.
- A real pseudo-terminal smoke check exercised startup, opening persisted content,
  scrolling, PgDown/End/Home, resizing from 100x24 to 50x12, Esc, and clean quit.
- Full tests, UI race tests, vet, compile check, and diff checks passed on
  2026-10-08. Read-only code review findings were fixed and rechecked.
- README documents controls and rendering behavior. AGENTS.md was checked and
  needs no additional guidance for this step.

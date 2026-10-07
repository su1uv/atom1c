# Step 5 — Fix UI focus and input routing

Status: implemented; focused input routing and modal lifecycle are covered by UI
model tests, and required checks pass.
Roadmap: [Step 5](roadmap.md#5-fix-ui-focus-and-input-routing).

## Objective

Route every key to its intended owner: the open modal, the active filter input,
or the selected list pane. Use the step to replace experimental shared mutable
focus state with a clearer ownership structure.

## Design and behavior

- The root model owns active-pane focus, modal visibility, dimensions, and key
  routing. A key that opens the modal or switches panes is consumed there.
- The modal owns its inputs and cursor mode, and reports close or submit actions
  to the root. Opening focuses the first field; closing blurs inputs; draft values
  survive reopening.
- Feed and post panes share one list implementation for list updates, sizing, and
  rendering. Each pane keeps its own list state and presentation width.
- While the active list is filtering, it exclusively receives key presses so
  shortcuts such as `a`, `P`, and `q` remain filter text.
- Pagination toggles and regular key presses are routed only to the active pane.
- List-pane commands tag asynchronous results with their owner and the root routes
  results only to that pane, preventing late filter results from crossing panes.
- Unowned non-key messages are sent to both lists when the modal is closed;
  window size is handled by the root and applied directly to both panes.

## Verification

- Added regression tests for modal opening, draft retention, modal-only input,
  field navigation and submit, filtering shortcuts in both panes, pagination
  scope, pane switching, and pane-scoped asynchronous results.
- Ran targeted tests against the prior implementation; initial input focus,
  filter shortcut handling, pagination focus, and modal reopening failed as
  expected.
- Passed `go test ./... -timeout 30s`.
- Passed `go vet ./...`.
- Passed `go test ./... -run '^$'`.
- Passed `git diff --check`.
- UI model behavior is tested; broader end-to-end workflows remain untested.

## Commit and publication

The original commit guidance grouped the complete roadmap step as one change; that
guidance is superseded. A plan is not a commit boundary: split future work into
small, independently reviewable and verified changes as required by
[AGENTS.md](../../AGENTS.md).

Commit only with explicit authorization. Publishing requires fresh explicit
authorization to push a new branch and open a PR targeting `main`.

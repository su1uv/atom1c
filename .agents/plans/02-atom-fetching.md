# Step 2 — Fix and verify Atom fetching

Status: completed; deterministic Atom parsing and HTTP tests pass.
Roadmap: [Step 2](roadmap.md#2-fix-and-verify-atom-fetching).

## Objective

Parse representative Atom feeds correctly and make HTTP fetching bounded,
status-aware, and cancellable. Keep verification independent of external services.

## Decisions

- Parse Atom elements in the Atom namespace, with either default or prefixed XML
  namespaces.
- Select the first link with `rel="alternate"` or omitted `rel`; skip other
  relations such as `self` and `enclosure`.
- Let XML decoding handle entity references once. Preserve HTML text constructs
  as HTML and retain XHTML inner markup.
- Use an entry summary only when the content element is absent.
- Use a 15-second default HTTP client timeout.
- Preserve feed-updated and entry-published timestamps as source strings.
- Keep RSS support out of this step.

## Implementation checklist

### A. Add failing tests first

- [x] Replace the live-network test with local HTTP servers and XML fixtures.
- [x] Add table-driven fixture checks for default/prefixed namespaces, titles,
  links, HTML/XHTML content, dates, multiple entries, summary fallback, and an
  empty feed.
- [x] Add table-driven malformed/non-Atom document and HTTP status checks.
- [x] Verify user-agent, redirects, cancellation, client timeout, invalid URLs,
  and response-body read errors/closure.
- [x] Run the new tests against the original implementation and confirm meaningful
  failures for links, summary fallback, root validation, and HTTP status handling.

### B. Correct Atom parsing

- [x] Decode link `href` and `rel` attributes and select alternate links.
- [x] Preserve transformed entry values in the returned entries slice.
- [x] Decode Atom text constructs without double-unescaping and preserve XHTML.
- [x] Use summary when content is absent and validate the Atom root namespace.

### C. Harden HTTP fetching and context flow

- [x] Use a reusable HTTP client with a 15-second timeout.
- [x] Reject non-2xx responses and wrap request, read, and XML parse errors.
- [x] Propagate caller context through feed selection, fetching, and fetch marking.
- [x] Close response bodies on success and error paths.

### D. Verify and update project records

- [x] `go test ./... -timeout 30s`
- [x] `go test ./... -run '^$'`
- [x] `go vet ./...`
- [x] `git diff --check`
- [x] Update `MEMORY.md` and the roadmap status.
- [x] Check whether `AGENTS.md` needs updating; TDD and table-driven test guidance
  are recorded there.

## Acceptance criteria

- [x] Representative Atom fields parse correctly with deterministic tests.
- [x] HTTP fetching is bounded, status-aware, cancellable, and closes response
  bodies.
- [x] Tests require no external network access.
- [x] All required checks pass.

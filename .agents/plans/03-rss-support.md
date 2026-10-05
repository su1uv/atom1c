# Step 3 — Add RSS 2.0 support

Status: completed; Atom and RSS 2.0 normalize into a shared representation and
all required checks pass.
Roadmap: [Step 3](roadmap.md#3-add-rss-20-support).

## Objective

Detect Atom and RSS 2.0 documents and normalize both into a shared feed/entry
representation. Keep format-specific parsing inside `internal/feed`, allowing
future storage and UI callers to use the same interface.

Step 3 establishes this shared contract. Persistence and real-data UI integration
remain scheduled for roadmap steps 4–7.

## Confirmed decisions

### Shared module and representation

- Move fetching, parsing, fixtures, tests, and the scraper from `internal/atom`
  into `internal/feed`.
- Replace Atom-specific exported types with shared `Feed` and `Entry` types.
- Provide one format-neutral fetch interface; keep XML document types private.
- Preserve source IDs, optional RSS GUID permalink metadata, and content kind.
- Preserve entry publication and update dates separately.
- Define deduplication in roadmap step 4.

### RSS content and links

- Prefer `content:encoded` in the standard content namespace:
  `http://purl.org/rss/1.0/modules/content/`.
- Fall back to `description` only when `content:encoded` is absent. A present but
  empty `content:encoded` remains empty.
- Treat description and encoded content as HTML-capable, preserve markup, and let
  XML decoding resolve entities once.
- Prefer an item's explicit `link`. If absent, use `guid` only when
  `isPermaLink` is true or omitted. A false GUID remains an opaque source ID.

### Dates

- Represent source dates with their raw value and an optional normalized UTC
  timestamp.
- Parse Atom RFC3339/RFC3339Nano and common RSS RFC822/RFC1123 variants, including
  numeric timezone offsets.
- Missing or unparseable dates have no normalized timestamp; retain raw values,
  and do not reject an otherwise parseable feed.
- Keep Atom entry `published` and `updated` distinct. RSS item `pubDate` supplies
  publication time only.
- Feed update time comes from Atom `updated`, or RSS `lastBuildDate`, falling
  back to channel `pubDate` when `lastBuildDate` is absent. Never substitute
  fetch time.

## Proposed shared representation

- `Feed`: title, link, updated date, entries.
- `Entry`: source ID, optional RSS GUID permalink flag, title, link, content,
  content kind, published date, updated date.
- `SourceDate`: raw value and optional `time.Time` normalized to UTC.
- Content kind: text, HTML, or XHTML.

Expose a format-neutral `Fetch(ctx, feedURL)` function. Keep HTTP-client injection
private to the package's tests.

## Implementation checklist

### A. Baseline and failing tests

- [x] Confirm Go version and clean working tree.
- [x] Run the existing test suite before changes.
- [x] Preserve existing Atom fixture and HTTP behavior coverage.
- [x] Add RSS fixtures for channel metadata/dates, multiple and empty channels,
  item metadata, GUIDs, CDATA, escaped HTML, and one-time entity decoding.
- [x] Cover content precedence, empty encoded content, namespace URI matching,
  explicit links, GUID fallback flags, and missing optional fields.
- [x] Add format detection and rejection cases for malformed XML, unrelated roots,
  wrong Atom namespace, unsupported RSS versions, RSS 1.0/RDF, and missing
  channels.
- [x] Add Atom and RSS date cases for supported layouts, UTC normalization,
  missing/invalid raw values, separate entry dates, and feed date fallback.
- [x] Verify both formats produce the same shared representation through the
  fetch interface, including misleading content-type headers.
- [x] Run new tests against the pre-support implementation and confirm meaningful
  failures before implementation.

### B. Introduce the shared module and normalize formats

- [x] Move the Atom implementation, fixtures, and tests into `internal/feed`.
- [x] Add shared types without format-specific XML tags or document fields.
- [x] Add format detection by XML root and dispatch to private Atom/RSS parsers.
- [x] Preserve existing Atom namespace, link, text/XHTML, and summary behavior;
  additionally retain Atom IDs, content kinds, and entry update dates.
- [x] Parse RSS 2.0 channel/items, content, links, GUIDs, and source dates by the
  confirmed rules.
- [x] Return descriptive errors for malformed and unsupported documents and an
  empty entries collection for an empty feed.
- [x] Preserve HTTP timeout, status validation, context propagation, user-agent,
  redirect behavior, error wrapping, and body closure.
- [x] Move the scraper to `internal/feed` and update it to consume the shared
  representation.
- [x] Remove the superseded `internal/atom` package after migration is verified.

### C. Verify and update records

- [x] Run targeted tests, then `go test ./... -timeout 30s`.
- [x] Run `go vet ./...`, `go test ./... -run '^$'`, and `git diff --check`.
- [x] Review the intended diff and request code review; preserve raw date text
  verbatim based on review feedback.
- [x] Update the roadmap to state shared-contract readiness; actual storage/UI
  integrations remain in steps 4–7.
- [x] Update `MEMORY.md` and mark this plan complete after all checks pass.
- [x] Check whether `AGENTS.md` needs an update; broaden Atom-specific fixture
  guidance to Atom/RSS if appropriate.

## Acceptance criteria

- [x] Representative Atom and RSS 2.0 feeds return the same shared types and
  callers do not choose parsers or inspect format-specific XML.
- [x] Existing Atom behavior remains covered and passes.
- [x] RSS content precedence, GUID/link behavior, namespaces, dates, and empty
  feeds follow the confirmed rules.
- [x] Source IDs, content kinds, raw dates, and normalized dates remain available
  for later persistence and reader work.
- [x] Unsupported formats and malformed documents return errors.
- [x] HTTP fetching remains bounded and cancellable; response bodies are closed.
- [x] Tests require no external network access and all required checks pass.

## PR3 review corrections (2026-10-05)

- Added regression tables and ran them before implementation. All four targeted
  tests failed: RSS namespace collisions replaced core values, common date
  variants stayed unnormalized, trailing XML was accepted, and oversize bodies
  were accepted (including streamed and gzip-decoded responses).
- RSS core elements/attributes now require an empty namespace; standard
  content namespace matching remains independent of prefix.
- Common RSS date layouts support single-digit days, short years, optional
  weekdays/seconds, and numeric offsets without guessing unknown zones.
- Validate the entire XML document through EOF, requiring exactly one root and
  allowing surrounding whitespace, comments, and processing instructions.
- Fetch limits decoded body data to 10 MiB (10,485,760 bytes), reading at most
  limit+1 bytes regardless of Content-Length, before parsing or marking fetched.
- Targeted tests and the full suite, vet, compile check, and diff check passed.
- Checked repository guidance; no feed-specific guidance change was needed.

## Commit and publication

Treat the shared module, RSS support, date normalization, tests, and associated
record updates as one atomic roadmap step. Split the step before implementation if
it becomes too large for one independently verified change.

Suggested implementation commit: `[feat]: normalize Atom and RSS feeds`.

Commit only with explicit authorization. Publishing requires fresh explicit
authorization to push a new remote branch and open a PR targeting `main`.

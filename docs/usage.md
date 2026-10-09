# Usage

Connect with an interactive SSH terminal. Each connection gets an independent
reader UI over the same feeds and posts. `q` or `ctrl+c` closes only that session.

## Feeds and posts

| Key | Action |
| --- | --- |
| `a` | Add a feed |
| `j` / `k`, up / down | Move within the current page |
| `h` / `l`, left / right | Previous / next page in the focused pane |
| `/` | Search feed names, or filter titles in the open feed's posts |
| `tab` | Open the selected feed's saved posts |
| `shift+tab` | Return focus to feeds |
| `enter` in posts | Read the selected post |
| `P` | Toggle the focused pane's page indicator |
| `R` | Refresh the selected or open feed |
| `r` | Retry the failed operation |
| `q` / `ctrl+c` | Quit |

Feed search is a case-insensitive substring search across all saved feeds. Post
search covers the loaded feed's posts. Search updates as you type; `enter` or
`esc` ends editing, and backspace changes or clears the query.

Lists fit the terminal and stop up/down navigation at page edges. Page changes
select the first item. Moving through feeds does not change the open posts pane;
press `tab` to open another feed.

Reads, additions, and refreshes run asynchronously. Loading, empty, and recoverable
error states appear in the relevant pane. A successful refresh updates connected
sessions that have the feed open. Feed refresh does not reload website articles.

### Adding a feed

| Key | Action |
| --- | --- |
| `tab` / `shift+tab`, up / down | Move between fields |
| `enter` | Advance focus; save when Submit is focused |
| `ctrl+r` | Change cursor style |
| `esc` | Close before saving |

Names must be nonblank and URLs must be absolute HTTP(S) URLs. Failed saves keep
the modal open and preserve the draft for correction or retry.

## Article reader

| Key | Action |
| --- | --- |
| `j` / `k`, down / up | Scroll one line |
| `PgDown` / `PgUp` | Scroll a page |
| `Home` / `End` | Jump to beginning / end |
| `R` | Reload the full website article |
| `r` | Retry failed article retrieval |
| `f` | Toggle full article / feed preview |
| `esc` | Return to posts, preserving navigation |
| `q` / `ctrl+c` | Quit |

The reader shows a cached full article when available. Otherwise it displays the
feed preview while retrieving and extracting the linked public HTML page.
Headings, lists, quotes, code, emphasis, and links render for the terminal; images
appear as text labels. Content reflows on resize, preserving relative reading
position where possible. Publication dates display in UTC, with supplied text or
`Unknown` as fallback.

Successful extractions are cached as Markdown in SQLite and survive restart for
offline reading. Failed reloads preserve the last successful cache. The cache is
tied to the post's link and removed when its post/feed is deleted.

Website extraction does not run JavaScript or sign in. Blocked or unextractable
pages fall back to feed previews. Local and private network destinations are
rejected. Feed previews render saved content without fetching embedded links.
Empty or unsupported content shows a notice; deep markup and oversized rendered
output are bounded.

Open articles remain stable snapshots during feed refresh. Return and reopen a
post to see updated content. List shortcuts are inactive while reading.

[Back to README](../README.md)

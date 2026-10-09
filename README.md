# Atom1c

> On Development

A self-hosted Atom/RSS feed aggregator and terminal reader accessed over SSH.

<p align="center">
  <img src="demo.gif" alt="Demo">
</p>

## Motivation

It's a learning project. I'm using it to get properly comfortable with the Charm TUI stack (Bubble Tea, Bubbles, Lipgloss).

## Quick Start

Requires Go 1.26.5 or newer.

```sh
git clone https://github.com/su1uv/atom1c.git
cd atom1c
echo 'GOOSE_DBSTRING=./atom1c.db' > .env
go run .
```

Startup applies database migrations and launches the SSH server. Go 1.26.5 or newer
and an authorized key are required. By default, the server listens on
`127.0.0.1:23234`; set `ATOM1C_SSH_ADDR=0.0.0.0:23234` in `.env` to accept
connections on all IPv4 interfaces.

Before starting, ensure the server account's `~/.ssh/authorized_keys` exists and
contains the public key for each client that should connect. Create the directory
with mode `700` and the key file with mode `600` if needed.

Atom1c authenticates public keys from the server account's `~/.ssh/authorized_keys`.
The SSH login name must match the operating-system account running Atom1c. Add one
plain public-key line per client; entries with OpenSSH restrictions/options are
rejected because Atom1c does not enforce those restrictions, and SSH certificate
entries are not supported. Password login is disabled. Authorized keys are loaded
at startup, so key-file changes require a restart.

The Ed25519 SSH host key is generated on first startup and retained at
`$XDG_DATA_HOME/atom1c/ssh_host_ed25519_key`, or
`~/.local/share/atom1c/ssh_host_ed25519_key` when `XDG_DATA_HOME` is unset. Keep
this file to preserve the server identity across restarts.

Connect with an interactive terminal (replace `server-account` and `host`):

```sh
ssh -p 23234 server-account@host
```

Atom1c accepts interactive PTY shells only; remote commands, SFTP, and port
forwarding are not enabled. Each SSH connection gets an independent reader UI
over the same database. Sessions see shared changes on their next data load or
after reconnecting, while successful feed refreshes automatically update sessions
that have that feed open. `q` or `ctrl+c` ends only the current reader session.
`SIGINT`/`SIGTERM` stops the server and active sessions.

## Usage

Main view:

| key | action |
| --- | --- |
| `a` | add a feed |
| `j` / `k` (or up / down) | move within the current list page |
| `h` / `l` (or left / right) | previous / next page in the focused pane |
| `/` | search all feeds by name (case-insensitive substring) |
| `tab` | open the selected feed and load its persisted posts |
| `shift+tab` | back to feeds |
| `enter` (posts pane) | read the selected post |
| `P` | toggle the pagination indicator for the focused pane |
| `R` | refresh the selected feed (or the feed open in the posts pane) |
| `r` | retry a failed database read or feed refresh |
| `q` / `ctrl+c` | quit |

Feed search updates as you type. Press `enter` to stop editing, `esc` to leave
search editing, and backspace to change or clear the query. Feed pages fit the
current pane size; the list is loaded from SQLite rather than limited to a fixed
number of feeds.

Open a feed with `tab` to load its saved posts. Moving the feed cursor does not
change which feed is open in the posts pane. Use `R` to fetch and store the
selected/open feed's latest entries; a successful refresh reloads the open posts.
Database reads and refreshes run asynchronously. Empty feeds, loading, and
recoverable errors are shown in the relevant pane; `r` retries the failed
operation.

Both lists stop up/down navigation at the current page's edges. Use `h`/`l` or
left/right to change pages; each page change selects its first item. Post pages
fit the pane height, with their page indicator in the header so additional posts
do not expand the container. `P` hides or shows that indicator without resizing
the pane. `/` in the posts pane filters titles across the loaded feed's posts.

Article reader:

| key | action |
| --- | --- |
| `j` / `k` (or down / up) | scroll one line |
| `PgDown` / `PgUp` | scroll a page |
| `Home` / `End` | jump to beginning / end |
| `esc` | return to posts, preserving selection, page, and filter |
| `q` / `ctrl+c` | quit the application |

The full-screen reader displays the saved title, feed name, publication date,
link, and feed-provided content. Publication times display in UTC; unparsed dates
display as supplied by the feed, and missing dates display as `Unknown`.
HTML/XHTML headings, lists, quotes, code, and links are rendered for the terminal;
images appear as text labels. Plain text stays literal. Empty content and
unsupported content types show notices. Websites and images are not fetched.

Content reflows when the terminal width changes, preserving relative reading
position where possible. Returning applies the current terminal dimensions to
the lists. An already-running refresh updates the posts behind the reader, while
the open article remains a stable snapshot; reopen it to see updated content.
List shortcuts are inactive while reading. Unusually deep markup is simplified,
and oversized rendered output is capped with a notice.

The reader first shows a saved extracted article when available. Otherwise it
shows the feed preview immediately and asynchronously retrieves the linked public
HTML page. Readable main content is extracted, converted to Markdown, and shown
in a centered, styled column; headings, nested lists, quotes, emphasis, links,
and code blocks are preserved. The website's title, author, and publication date
are used when available, with feed metadata as fallback. `R` explicitly reloads
the full article, `r` retries a failed retrieval, and `f` toggles between full
article and feed preview. Website reloads do not happen during feed refresh.

Successful extractions are cached in SQLite as Markdown and are available offline
after restart. A failed reload keeps the last successful cached article. The
cache is tied to the post's current link and is removed when its post/feed is
deleted. Website extraction supports public HTTP(S) HTML pages; it does not run
JavaScript or sign in to websites. Pages that block requests or cannot be
extracted remain readable through their feed preview. Fetching rejects local and
private network destinations to avoid using article links to access host-private
services.

The reusable [`reader`](reader) and [`reader/view`](reader/view) Go packages own
article retrieval/extraction, HTML-to-Markdown conversion, terminal Markdown
rendering, the focused column, scrolling, and responsive layout without importing
Atom1c application or database packages. Run the standalone reader example with:

```sh
go run ./examples/reader https://example.com/article
```

## Configuration

`GOOSE_DBSTRING` is required and selects the SQLite database. SSH settings are
optional; defaults are shown below:

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `ATOM1C_SSH_ADDR` | `127.0.0.1:23234` | SSH listen address |
| `ATOM1C_AUTHORIZED_KEYS` | `~/.ssh/authorized_keys` | Public keys allowed to log in |
| `ATOM1C_SSH_HOST_KEY` | `$XDG_DATA_HOME/atom1c/ssh_host_ed25519_key` or `~/.local/share/atom1c/ssh_host_ed25519_key` | Persistent Ed25519 host key |
| `ATOM1C_REFRESH_INTERVAL` | `15m` | Automatic feed refresh interval; `0` disables scheduled refresh |

Automatic refresh runs one sweep at startup, then waits the configured interval
after each sweep completes. Feeds refresh sequentially; individual failures are
logged and do not stop the rest of a sweep. Manual refresh remains available when
scheduling is disabled. Set a Go duration such as `30m`; malformed and negative
values are rejected at startup.

See `.env.example` for a minimal configuration template.

Add feed modal:

| key | action |
| --- | --- |
| `tab` / `shift+tab` (or `up` / `down`) | move between fields |
| `enter` | submit |
| `ctrl+r` | change cursor style |
| `esc` | close before saving |

Feed names must be nonblank and URLs must be absolute HTTP(S) URLs. The modal
remains open while saving; failed saves preserve the draft so it can be corrected
or retried.

## Contributing

I'm not accepting pull requests for now.

That said, if you spot something wrong or weird, or just have an opinion, please open an issue; I'd like to hear it.

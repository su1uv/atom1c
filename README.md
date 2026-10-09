# Atom1c

> On Development

A self-hosted Atom/RSS feed aggregator and terminal reader accessed over SSH.

<p align="center">
  <img src="demo.gif" alt="Demo">
</p>

## Motivation

It's a learning project. I'm using it to get properly comfortable with the Charm TUI stack (Bubble Tea, Bubbles, Lipgloss).

## Quick Start — Docker Compose

Docker Engine and the Docker Compose plugin are required. Compose builds the
Linux/amd64 image locally; the container runs as the unprivileged `atom1c` account.

1. Clone the repository and prepare a dedicated public-key file on the server:

   ```sh
   git clone https://github.com/su1uv/atom1c.git
   cd atom1c
   # If you don't already have an SSH client key, create one with ssh-keygen.
   mkdir -p ~/.config/atom1c
   cp ~/.ssh/id_ed25519.pub ~/.config/atom1c/authorized_keys
   chmod 644 ~/.config/atom1c/authorized_keys
   ```

   Add one plain SSH public-key line per client. The file contains public keys,
   not private keys. It must be readable by the container's `atom1c` user; Compose
   mounts it read-only.

2. Configure Compose and start the service:

   ```sh
   cp .env.example .env
   ```

   Edit `.env` and set `ATOM1C_AUTHORIZED_KEYS` to the absolute path of the file
   prepared above, for example `/home/alice/.config/atom1c/authorized_keys`.
   Then build and start:

   ```sh
   docker compose up --build -d
   docker compose logs -f atom1c
   ```

   Press `ctrl+c` to stop following logs; the server keeps running.

3. Connect from the same machine:

   ```sh
   ssh -p 23234 atom1c@localhost
   ```

   The default host binding is loopback-only. For remote connections, set
   `ATOM1C_SSH_BIND_ADDRESS=0.0.0.0` in `.env`, open TCP port 23234 in the host
   firewall, and connect as `atom1c@server-host`. Keep the host firewall limited
   to the clients that should reach SSH.

Compose applies embedded SQLite migrations on startup. Its named `atom1c-data`
volume stores the database and persistent Ed25519 SSH host key; rebuilding or
recreating the container retains both. `docker compose down` also preserves the
volume. **`docker compose down --volumes` deletes the database and host identity.**

Stop and restart the service with:

```sh
docker compose stop
docker compose start
```

Rebuild after changing source with `docker compose up --build -d`. For a stopped
instance, copy the database and host identity out of the container:

```sh
docker compose stop
docker compose cp atom1c:/data/atom1c.db ./atom1c.db.backup
docker compose cp atom1c:/data/ssh_host_ed25519_key ./ssh_host_ed25519_key.backup
chmod 600 ./atom1c.db.backup ./ssh_host_ed25519_key.backup
```

Keep both backup files together and protect the host-key backup as a private key.
To restore them, stop the service and use a short-lived helper container to copy
the files into the Compose volume with the required owner and host-key permissions:

```sh
container_id="$(docker compose ps --all --quiet atom1c)"
data_volume="$(docker inspect "$container_id" --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Name}}{{end}}{{end}}')"
docker run --rm --network none \
  --volume "$data_volume:/data" \
  --volume "$PWD:/backup:ro" \
  busybox:1.37.0 sh -ec '
    cp /backup/atom1c.db.backup /data/atom1c.db
    cp /backup/ssh_host_ed25519_key.backup /data/ssh_host_ed25519_key
    rm -f /data/atom1c.db-wal /data/atom1c.db-shm
    chown 10001:10001 /data/atom1c.db /data/ssh_host_ed25519_key
    chmod 600 /data/atom1c.db /data/ssh_host_ed25519_key
  '
docker compose start
```

Run the restore command from the directory containing the two backup files.
Restore only a backup made after a clean stop.

The database contains feeds, posts, and extracted article caches. The host key
preserves the SSH server identity across restore; clients otherwise see a changed
host-key warning.

The container accepts interactive PTY shells only. Remote commands, SFTP,
password authentication, and port forwarding are disabled. Each SSH connection
gets an independent reader UI over shared data. Successful feed refreshes update
connected sessions that have that feed open. `q` or `ctrl+c` ends only the current
reader session. Stopping the service cancels active feed work and sessions.

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
| `R` | reload the full website article |
| `r` | retry a failed article retrieval |
| `f` | toggle full article / feed preview |
| `esc` | return to posts, preserving selection, page, and filter |
| `q` / `ctrl+c` | quit the application |

The full-screen reader displays the saved title, feed name, publication date,
link, and feed-provided content. Publication times display in UTC; unparsed dates
display as supplied by the feed, and missing dates display as `Unknown`.
HTML/XHTML headings, lists, quotes, code, and links are rendered for the terminal;
images appear as text labels. Plain text stays literal. Empty content and
unsupported content types show notices. Article links embedded in feed content
are displayed but not fetched by this renderer.

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

Docker Compose reads these values from the project `.env` file or the shell
environment:

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `ATOM1C_AUTHORIZED_KEYS` | required | Host path to the dedicated public-key file mounted read-only into the container |
| `ATOM1C_SSH_BIND_ADDRESS` | `127.0.0.1` | Host interface for the published SSH port; set `0.0.0.0` for remote connections |
| `ATOM1C_SSH_PORT` | `23234` | Host port published for SSH |
| `ATOM1C_REFRESH_INTERVAL` | `15m` | Automatic feed refresh interval; `0` disables scheduled refresh |

The container listens on `0.0.0.0:23234` internally and logs in as `atom1c`.
Compose persists its database and Ed25519 host key in the `atom1c-data` named
volume. Keep that volume when rebuilding or replacing the container.

For native Go startup, `GOOSE_DBSTRING` is required. Native SSH defaults are:

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `ATOM1C_SSH_ADDR` | `127.0.0.1:23234` | SSH listen address |
| `ATOM1C_AUTHORIZED_KEYS` | `~/.ssh/authorized_keys` | Public keys allowed to log in |
| `ATOM1C_SSH_HOST_KEY` | `$XDG_DATA_HOME/atom1c/ssh_host_ed25519_key` or `~/.local/share/atom1c/ssh_host_ed25519_key` | Persistent Ed25519 host key |
| `ATOM1C_REFRESH_INTERVAL` | `15m` | Automatic feed refresh interval; `0` disables scheduled refresh |

To run natively, use Go 1.26.5 or newer, set `GOOSE_DBSTRING` in `.env` or the
environment, and ensure the current OS account's `~/.ssh/authorized_keys` contains
plain public-key lines. The SSH login name must match the OS account running the
server. Start it from the repository root:

```sh
GOOSE_DBSTRING=./atom1c.db go run .
```

The first startup applies database migrations and creates the Ed25519 host key.
Keep that key file to preserve server identity. Authorized keys load at startup,
so key-file changes require a restart. OpenSSH key restrictions/options and
certificate entries are rejected; password login is disabled.

Automatic refresh runs one sweep at startup, then waits the configured interval
after each sweep completes. Feeds refresh sequentially; individual failures are
logged and do not stop the rest of a sweep. Manual refresh remains available when
scheduling is disabled. Set a Go duration such as `30m`; `0` disables scheduled
refresh, while malformed and negative values are rejected at startup.

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

Run the Go checks from the repository root:

```sh
go test ./... -timeout 30s
go vet ./...
go test ./... -run '^$'
```

The Docker Compose end-to-end workflow requires Docker Engine and Compose. It
builds isolated Linux/amd64 images, creates a temporary key and named volume,
exercises interactive SSH/feed workflows, and removes its test containers,
network, and volume when finished:

```sh
go run ./scripts/compose-smoke
```

That workflow also uses the `busybox:1.37.0` image to verify the documented
database/host-key backup and restore path. It does not touch the normal Compose
project or its data volume.

If you spot something wrong or weird, or just have an opinion, please open an
issue; I'd like to hear it.

# Deployment

## Docker Compose

Follow the [quick start](../README.md#quick-start) to prepare a dedicated file with
one plain SSH public-key line per client. The file must be readable by the
container's unprivileged `atom1c` account and is mounted read-only. Never put a
private key in this file.

Compose builds a Linux/amd64 image and applies embedded SQLite migrations on
startup. It listens internally on `0.0.0.0:23234`; the SSH login is `atom1c`.

Settings come from the project `.env` file or shell environment:

| Variable | Default | Purpose |
| --- | --- | --- |
| `ATOM1C_AUTHORIZED_KEYS` | Required | Absolute host path to the public-key file |
| `ATOM1C_SSH_BIND_ADDRESS` | `127.0.0.1` | Published SSH interface; use `0.0.0.0` for remote access |
| `ATOM1C_SSH_PORT` | `23234` | Published SSH port |
| `ATOM1C_REFRESH_INTERVAL` | `15m` | Automatic refresh interval; `0` disables it |

For remote access, allow the chosen TCP port in the host firewall for your
clients, then connect with `ssh -p 23234 atom1c@server-host`.

```sh
docker compose logs -f atom1c    # ctrl+c stops following logs
docker compose stop
docker compose start
docker compose up --build -d    # rebuild after source changes
```

The `atom1c-data` named volume stores the database and persistent Ed25519 host
key. Rebuilds, container recreation, and `docker compose down` preserve it.
**`docker compose down --volumes` deletes the data and host identity.**

## Backup and restore

Stop the service before copying the database and host key:

```sh
docker compose stop
docker compose cp atom1c:/data/atom1c.db ./atom1c.db.backup
docker compose cp atom1c:/data/ssh_host_ed25519_key ./ssh_host_ed25519_key.backup
chmod 600 ./atom1c.db.backup ./ssh_host_ed25519_key.backup
```

Keep both files together and protect the host-key backup as a private key. The
database includes feeds, posts, and article caches; the host key preserves the
server identity trusted by SSH clients.

To restore a backup made after a clean stop, run from the directory containing
the two files. Keep the Compose container available to locate its named volume:

```sh
docker compose stop
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

## Native Go startup

Use Go 1.26.5 or newer. Startup loads `.env`; `GOOSE_DBSTRING` is required.
The SSH login must match the OS account running the server.

| Variable | Default | Purpose |
| --- | --- | --- |
| `GOOSE_DBSTRING` | Required | SQLite database path |
| `ATOM1C_SSH_ADDR` | `127.0.0.1:23234` | SSH listen address |
| `ATOM1C_AUTHORIZED_KEYS` | `~/.ssh/authorized_keys` | Allowed public-key file |
| `ATOM1C_SSH_HOST_KEY` | `$XDG_DATA_HOME/atom1c/ssh_host_ed25519_key`, falling back to `~/.local/share/atom1c/ssh_host_ed25519_key` | Persistent host key |
| `ATOM1C_REFRESH_INTERVAL` | `15m` | Refresh interval; `0` disables it |

Ensure the authorized-key file contains plain public-key lines, then run from
the repository root:

```sh
GOOSE_DBSTRING=./atom1c.db go run .
```

In another terminal, connect to the running server:

```sh
ssh -p 23234 "$(whoami)@localhost"
```

The first startup applies migrations and creates the host key. Preserve that key
to retain server identity. Authorized keys load at startup; changes require a
restart. Key options/restrictions and certificates are rejected.

## Refresh and sessions

Automatic refresh runs a sequential sweep at startup, then waits the configured
interval after each sweep. Individual failures are logged without stopping the
remaining feeds. Use a Go duration such as `30m`; malformed and negative intervals
are rejected. `0` disables scheduled refresh; manual refresh remains available.

Only interactive PTY shells are accepted. Password authentication, remote
commands, SFTP, and port forwarding are disabled. Each session has its own UI
over shared data. Stopping the service cancels active refresh work and sessions.

See [`.env.example`](../.env.example) for a configuration template.

[Back to README](../README.md)

# Atom1c

A personal, self-hosted Atom/RSS aggregator and terminal reader over SSH.
Your feeds, your server, any SSH terminal.

![SSH login, feed search, and full-article reading in Atom1c](demo.gif)

- Atom and RSS 2.0 feeds with search, pagination, and automatic refresh.
- Full-screen reader with website article extraction and offline article caching.
- SQLite persistence, public-key SSH access, and Docker Compose deployment.

## Quick start

Requires Docker Engine and the Docker Compose plugin. The image targets Linux/amd64.

```sh
git clone https://github.com/su1uv/atom1c.git
cd atom1c
mkdir -p ~/.config/atom1c
cp ~/.ssh/id_ed25519.pub ~/.config/atom1c/authorized_keys
chmod 644 ~/.config/atom1c/authorized_keys
cp .env.example .env
```

Use an existing SSH public key, or create one with `ssh-keygen` first.
Set `ATOM1C_AUTHORIZED_KEYS` in `.env` to the **absolute path** of the file above,
then start and connect:

```sh
docker compose up --build -d
ssh -p 23234 atom1c@localhost
```

Press `a` to add a feed, `R` to refresh it, `tab` to open its posts, and `enter`
to read. Use `/` to search and `q` to quit.

SSH binds to localhost by default. For remote access, set
`ATOM1C_SSH_BIND_ADDRESS=0.0.0.0` in `.env` and allow TCP port 23234 through your
server's firewall. Data and SSH host identity persist in a Docker volume.

## Documentation

- [Usage and keyboard shortcuts](docs/usage.md)
- [Deployment, configuration, and backups](docs/deployment.md)
- [Development and verification](docs/development.md)

Public HTML articles are supported; JavaScript-rendered and authenticated sites
are not. RSS 1.0/RDF is not supported.

Feedback and bug reports are welcome through issues. Pull requests are currently
not accepted.

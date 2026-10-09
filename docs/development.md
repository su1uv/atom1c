# Development

Use Go 1.26.5 or newer. Run checks from the repository root:

```sh
go test ./... -timeout 30s
go vet ./...
go test ./... -run '^$'
```

SQL sources live in `sql/schema` and `sql/queries`. Regenerate database code with
`sqlc generate`; migrations are embedded, so rebuild after changing them.

## Deployment smoke check

Requires Docker Engine and Docker Compose:

```sh
go run ./scripts/compose-smoke
```

The workflow builds isolated Linux/amd64 images and creates temporary SSH keys
and a named volume. It verifies SSH/feed workflows, restart persistence,
scheduled refresh, and backup/restore using `busybox:1.37.0`. Test containers,
network, and volume are removed afterward; the normal Compose data is untouched.

## Standalone article reader

The reusable [`reader`](../reader) and [`reader/view`](../reader/view) packages
provide retrieval, extraction, HTML-to-Markdown conversion, terminal rendering,
scrolling, and responsive layout independently of the application and database.

```sh
go run ./examples/reader https://example.com/article
```

Feedback and bug reports are welcome through issues. Pull requests are currently
not accepted.

[Back to README](../README.md)

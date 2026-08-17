-- name: GetFeeds :many
SELECT
    id,
    created_at,
    updated_at,
    name,
    url,
    last_fetched_at
FROM feeds
ORDER BY created_at
LIMIT 20;

-- name: CreateFeed :one
INSERT INTO feeds (
    created_at, updated_at, name, url
) VALUES (
    ?, ?, ?, ?
) RETURNING *;

-- name: MarkFeedAsFetched :exec
UPDATE feeds
SET last_fetched_at = ?, updated_at = ?
WHERE id = ?;

-- name: GetNextFeedToFetch :one
SELECT
    id, created_at, updated_at, name, url, last_fetched_at
FROM feeds
ORDER BY last_fetched_at ASC
LIMIT 1;

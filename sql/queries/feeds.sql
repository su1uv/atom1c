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
    name, url
) VALUES (
    ?, ?
) RETURNING *;

-- name: MarkFeedAsFetched :execrows
UPDATE feeds
SET last_fetched_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: GetNextFeedToFetch :one
SELECT
    id, created_at, updated_at, name, url, last_fetched_at
FROM feeds
ORDER BY last_fetched_at ASC
LIMIT 1;

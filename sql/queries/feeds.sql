-- name: GetFeedsPage :many
SELECT
    id,
    created_at,
    updated_at,
    name,
    url,
    last_fetched_at
FROM feeds
WHERE instr(unicode_lower(name), unicode_lower(CAST(sqlc.arg(search) AS TEXT))) > 0
ORDER BY created_at, id
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountFeeds :one
SELECT COUNT(*)
FROM feeds
WHERE instr(unicode_lower(name), unicode_lower(CAST(sqlc.arg(search) AS TEXT))) > 0;

-- name: GetFeedPosition :one
WITH target AS (
    SELECT feed.id, feed.created_at
    FROM feeds AS feed
    WHERE feed.id = sqlc.arg(id)
)
SELECT COUNT(*)
FROM feeds, target
WHERE instr(unicode_lower(feeds.name), unicode_lower(CAST(sqlc.arg(search) AS TEXT))) > 0
  AND (
      feeds.created_at < target.created_at
      OR (feeds.created_at = target.created_at AND feeds.id < target.id)
  );

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

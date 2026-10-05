-- name: UpsertPost :one
INSERT INTO posts (
    feed_id,
    identity_key,
    source_id,
    guid_is_permalink,
    title,
    link,
    content,
    content_kind,
    published_raw,
    published_at,
    updated_raw,
    source_updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
ON CONFLICT (feed_id, identity_key) DO UPDATE SET
    source_id = excluded.source_id,
    guid_is_permalink = excluded.guid_is_permalink,
    title = excluded.title,
    link = excluded.link,
    content = excluded.content,
    content_kind = excluded.content_kind,
    published_raw = excluded.published_raw,
    published_at = excluded.published_at,
    updated_raw = excluded.updated_raw,
    source_updated_at = excluded.source_updated_at,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetPostsByFeed :many
SELECT * FROM posts
WHERE feed_id = ?
ORDER BY id DESC;

-- name: GetPost :one
SELECT * FROM posts
WHERE id = ?;

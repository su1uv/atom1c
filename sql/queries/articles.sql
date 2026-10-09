-- name: GetArticleCache :one
SELECT * FROM article_cache
WHERE post_id = ? AND source_url = ?;

-- name: UpsertArticleCache :one
INSERT INTO article_cache (
    post_id, source_url, final_url, markdown, title, author, site_name, published_at
)
SELECT id, link, ?, ?, ?, ?, ?, ?
FROM posts
WHERE id = ? AND link = ?
ON CONFLICT (post_id) DO UPDATE SET
    source_url = excluded.source_url,
    final_url = excluded.final_url,
    markdown = excluded.markdown,
    title = excluded.title,
    author = excluded.author,
    site_name = excluded.site_name,
    published_at = excluded.published_at,
    fetched_at = CURRENT_TIMESTAMP
RETURNING *;

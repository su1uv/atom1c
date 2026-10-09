-- +goose Up
CREATE TABLE article_cache (
    post_id INTEGER PRIMARY KEY REFERENCES posts(id) ON DELETE CASCADE,
    source_url TEXT NOT NULL,
    final_url TEXT NOT NULL,
    markdown TEXT NOT NULL,
    title TEXT NOT NULL,
    author TEXT NOT NULL,
    site_name TEXT NOT NULL,
    published_at TEXT NOT NULL,
    fetched_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;

CREATE INDEX article_cache_fetched_at_idx ON article_cache (fetched_at);

-- +goose Down
DROP TABLE article_cache;

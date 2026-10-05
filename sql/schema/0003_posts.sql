-- +goose Up
CREATE TABLE posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_id INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
    identity_key TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    source_id TEXT NOT NULL,
    guid_is_permalink INTEGER,
    title TEXT NOT NULL,
    link TEXT NOT NULL,
    content TEXT NOT NULL,
    content_kind TEXT NOT NULL,
    published_raw TEXT NOT NULL,
    published_at TEXT,
    updated_raw TEXT NOT NULL,
    source_updated_at TEXT,
    UNIQUE (feed_id, identity_key)
) STRICT;

CREATE INDEX posts_feed_id_id_idx ON posts (feed_id, id DESC);

-- +goose Down
DROP TABLE posts;

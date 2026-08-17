-- +goose Up
CREATE TABLE feeds (
    id INT PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    name TEXT NOT NULL,
    url TEXT UNIQUE NOT NULL,
    last_fetched_at TIMESTAMP
) STRICT;

-- +goose Down
DROP TABLE feeds;

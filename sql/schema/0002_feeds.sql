-- +goose Up
create table if not exists feeds (
    id uuid primary key,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    name text not null,
    url text not null,
    last_fetched_at timestamptz,
    unique(url)
);

-- +goose Down
drop table if exists feeds;

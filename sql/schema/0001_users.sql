-- +goose Up
create table if not exists users (
    id uuid primary key,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    username text not null,
    unique(username)
);

-- +goose Down
drop table if exists users;

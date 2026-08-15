-- +goose Up
create table users (
    id uuid primary key,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    username text not null,
    unique(username)
);

-- +goose Down
drop table users;

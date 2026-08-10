-- +goose Up
create table if not exists feeds (
    id uuid primary key,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    name text not null,
    url text not null,
    unique(url),
    user_id uuid not null,
    foreign key (user_id) references users(id)
);

-- +goose Down
drop table if exists feeds;

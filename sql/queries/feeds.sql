-- name: GetFeeds :many
select
    id,
    created_at,
    updated_at,
    name,
    url,
    last_fetched_at
from feeds
order by created_at
limit 20;

-- name: CreateFeed :one
insert into feeds (
    id, created_at, updated_at, name, url
) values (
    $1, $2, $3, $4, $5
) returning *;

-- name: MarkFeedAsFetched :exec
update feeds
set last_fetched_at = $1, updated_at = $2
where id = $3;

-- name: GetNextFeedToFetch :one
select
    id, created_at, updated_at, name, url, last_fetched_at
from feeds
order by last_fetched_at asc nulls first
limit 1;

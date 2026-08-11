-- name: GetFeeds :many
select
    id,
    created_at,
    updated_at,
    name,
    url
from feeds
order by created_at;

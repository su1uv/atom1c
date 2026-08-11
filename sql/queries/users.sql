-- name: GetUserByUsername :one
select
    id,
    created_at,
    updated_at,
    username
from users
where username = $1;

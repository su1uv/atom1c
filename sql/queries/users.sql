-- name: GetUserByUsername :one
SELECT
    id,
    created_at,
    updated_at,
    username
FROM users
WHERE username = ?;

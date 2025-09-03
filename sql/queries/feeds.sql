-- name: CreateFeed :one
INSERT INTO feeds(name ,url ,user_id)
VALUES(
    $1,
    $2,
    $3
)
RETURNING *;

-- name: GetFeeds :many
SELECT * FROM feeds;

-- name: GetFeedId :one
SELECT id FROM feeds Where url = $1;

-- name: MarkFeedFetched :exec
UPDATE feeds
SET last_fetched_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT * FROM feeds ORDER BY last_fetched_at ASC NULLS FIRST LIMIT 2;

-- name: GetFeedForID :one
SELECT * FROM feeds WHERE id = $1;
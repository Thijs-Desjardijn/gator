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
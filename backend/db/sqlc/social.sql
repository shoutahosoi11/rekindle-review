-- name: FollowUser :execrows
INSERT INTO follows (follower_id, followee_id)
VALUES ($1, $2)
ON CONFLICT (follower_id, followee_id) DO NOTHING;

-- name: UnfollowUser :execrows
DELETE FROM follows
WHERE follower_id = $1 AND followee_id = $2;

-- name: ListCommentsByPostID :many
SELECT
    c.id,
    c.post_id,
    c.user_id,
    u.username,
    u.display_name,
    u.avatar_url,
    c.content,
    c.created_at
FROM comments c
JOIN users u ON u.id = c.user_id
WHERE c.post_id = $1
ORDER BY c.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetTimelinePostByID :one
SELECT
    p.id,
    p.user_id,
    p.question_id,
    p.book_id,
    p.field_id,
    p.body,
    COALESCE(p.book_title, b.title)::text AS book_title,
    p.question_count,
    p.type,
    p.repost_count,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at,
    0::integer AS score,
    u.username,
    u.display_name,
    u.avatar_url,
    fi.name AS field_name
FROM posts p
JOIN users u ON p.user_id = u.id
LEFT JOIN books b ON p.book_id = b.id
LEFT JOIN fields fi ON p.field_id = fi.id
WHERE p.id = $1
LIMIT 1;

-- name: ListPostedQuestionsByPostID :many
SELECT
    q.id,
    q.question_type,
    q.body,
    q.options,
    q.correct_answer,
    q.explanation,
    pq.note,
    pq.sort_order
FROM post_questions pq
JOIN questions q ON q.id = pq.question_id
WHERE pq.post_id = $1
ORDER BY pq.sort_order ASC;

-- name: PostExists :one
SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1);

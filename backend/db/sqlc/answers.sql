-- name: UpsertAnswer :one
INSERT INTO answers (
    user_id,
    question_id,
    user_answer,
    is_correct
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (user_id, question_id) DO UPDATE SET
    user_answer  = EXCLUDED.user_answer,
    is_correct   = EXCLUDED.is_correct,
    updated_at   = NOW()
RETURNING id, user_id, question_id, user_answer, is_correct, created_at, updated_at;

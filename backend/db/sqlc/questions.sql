-- name: CreateQuestion :exec
INSERT INTO questions (
    id,
    user_id,
    source_type,
    question_type,
    body,
    options,
    correct_answer,
    explanation,
    is_ai_generated,
    generation_id,
    highlight_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);

-- name: ListQuestionsByCreatorID :many
SELECT id, question_type, body, options, correct_answer, explanation
FROM questions
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: FindQuestionByID :one
SELECT
    id,
    user_id,
    source_type,
    question_type,
    body,
    options,
    correct_answer,
    explanation,
    is_ai_generated,
    generation_id,
    highlight_id,
    answer_count,
    correct_count
FROM questions
WHERE id = $1
LIMIT 1;

-- name: GetQuestionByID :one
SELECT id, question_type, body, options, correct_answer, explanation
FROM questions
WHERE id = $1
LIMIT 1;

-- name: UpdateQuestionStats :exec
UPDATE questions
SET
    answer_count  = answer_count + 1,
    correct_count = correct_count + CASE WHEN sqlc.arg(is_correct)::boolean THEN 1 ELSE 0 END,
    updated_at    = NOW()
WHERE id = $1;

-- name: SaveQuestionForUser :exec
INSERT INTO saved_questions (user_id, question_id, note, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (user_id, question_id)
DO UPDATE SET note = EXCLUDED.note, updated_at = NOW();

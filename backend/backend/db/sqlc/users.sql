-- name: GetUserByFirebaseUID :one
SELECT * FROM users
WHERE firebase_uid = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1
LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (
    firebase_uid,
    username,
    display_name,
    avatar_url,
    bio,
    university,
    faculty,
    grade,
    country,
    plan
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, 'free'
)
RETURNING *;

-- name: UpdateUser :one
UPDATE users SET
    username = $2,
    display_name = $3,
    avatar_url = $4,
    bio = $5,
    university = $6,
    faculty = $7,
    grade = $8,
    country = $9,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserQuestionSettings :one
UPDATE users SET
    default_question_count = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

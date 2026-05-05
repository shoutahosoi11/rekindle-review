-- name: ListHighlightsByUserIDAndASIN :many
SELECT
    id, user_id, book_id, content, location, created_at,
    book_title, book_author, asin, highlighted_at, source, updated_at,
    content_hash, explanation, source_app, source_url, status, retry_count,
    generation_requested_at, processing_started_at, completed_at, failed_at, last_error, book_order_index
FROM highlights
WHERE user_id = $1
  AND asin = sqlc.arg(asin)
  AND source = sqlc.arg(source)
ORDER BY highlighted_at ASC NULLS LAST, created_at ASC;

-- name: ListHighlightsByUserIDAndBookMetadata :many
SELECT
    id, user_id, book_id, content, location, created_at,
    book_title, book_author, asin, highlighted_at, source, updated_at,
    content_hash, explanation, source_app, source_url, status, retry_count,
    generation_requested_at, processing_started_at, completed_at, failed_at, last_error, book_order_index
FROM highlights
WHERE user_id = $1
  AND source = sqlc.arg(source)
  AND lower(trim(coalesce(book_title, ''))) = lower(trim(sqlc.arg(book_title)))
  AND lower(trim(coalesce(book_author, ''))) = lower(trim(sqlc.arg(book_author)))
ORDER BY highlighted_at ASC NULLS LAST, created_at ASC;

-- name: ListHighlightsByUserIDAndBookTitle :many
SELECT
    id, user_id, book_id, content, location, created_at,
    book_title, book_author, asin, highlighted_at, source, updated_at,
    content_hash, explanation, source_app, source_url, status, retry_count,
    generation_requested_at, processing_started_at, completed_at, failed_at, last_error, book_order_index
FROM highlights
WHERE user_id = $1
  AND source = sqlc.arg(source)
  AND lower(trim(coalesce(book_title, ''))) = lower(trim(sqlc.arg(book_title)))
ORDER BY highlighted_at ASC NULLS LAST, created_at ASC;

-- name: ListHighlightBooksByUserID :many
SELECT
    asin,
    COALESCE(MAX(book_title), '')::text AS book_title,
    COALESCE(MAX(book_author), '')::text AS book_author,
    COUNT(*) AS highlight_count,
    source
FROM highlights
WHERE user_id = $1
  AND asin IS NOT NULL
  AND source = $2
GROUP BY asin, source
ORDER BY MAX(highlighted_at) DESC NULLS LAST, MAX(created_at) DESC;

-- name: CreateHighlight :one
INSERT INTO highlights (
    user_id,
    book_id,
    book_title,
    book_author,
    asin,
    content,
    explanation,
    location,
    highlighted_at,
    source,
    content_hash,
    source_app,
    source_url
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING
    id, user_id, book_id, content, location, created_at,
    book_title, book_author, asin, highlighted_at, source, updated_at,
    content_hash, explanation, source_app, source_url, status, retry_count,
    generation_requested_at, processing_started_at, completed_at, failed_at, last_error, book_order_index;

-- name: GetHighlightByUserIDAndContentHash :one
SELECT
    id, user_id, book_id, content, location, created_at,
    book_title, book_author, asin, highlighted_at, source, updated_at,
    content_hash, explanation, source_app, source_url, status, retry_count,
    generation_requested_at, processing_started_at, completed_at, failed_at, last_error, book_order_index
FROM highlights
WHERE user_id = $1
  AND content_hash = $2
ORDER BY created_at ASC, id ASC
LIMIT 1;

-- name: UpdateHighlightExplanation :one
UPDATE highlights
SET explanation = $3, updated_at = NOW()
WHERE id = $1 AND user_id = $2
RETURNING
    id, user_id, book_id, content, location, created_at,
    book_title, book_author, asin, highlighted_at, source, updated_at,
    content_hash, explanation, source_app, source_url, status, retry_count,
    generation_requested_at, processing_started_at, completed_at, failed_at, last_error, book_order_index;

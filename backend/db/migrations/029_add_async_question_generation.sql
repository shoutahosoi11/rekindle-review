CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE highlights
    ADD COLUMN IF NOT EXISTS book_title TEXT,
    ADD COLUMN IF NOT EXISTS book_author TEXT,
    ADD COLUMN IF NOT EXISTS asin TEXT,
    ADD COLUMN IF NOT EXISTS highlighted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS source VARCHAR(20) NOT NULL DEFAULT 'manual',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS content_hash TEXT,
    ADD COLUMN IF NOT EXISTS explanation TEXT,
    ADD COLUMN IF NOT EXISTS source_app TEXT,
    ADD COLUMN IF NOT EXISTS source_url TEXT;

ALTER TABLE highlights
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS generation_requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS processing_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS failed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_error TEXT;

CREATE INDEX IF NOT EXISTS idx_highlights_status_user_requested_at
    ON highlights (status, user_id, generation_requested_at);

DROP INDEX IF EXISTS highlights_user_id_content_hash_idx;

UPDATE highlights
SET content_hash = encode(
        digest(
            CASE
                WHEN source = 'kindle' THEN format(
                    'source:%s:asin:%s:loc:%s:content:%s',
                    coalesce(source, ''),
                    coalesce(trim(asin), ''),
                    coalesce(trim(location), ''),
                    regexp_replace(lower(trim(content)), '\s+', ' ', 'g')
                )
                ELSE format(
                    'source:%s:app:%s:url:%s:title:%s:author:%s:content:%s',
                    coalesce(source, ''),
                    coalesce(trim(source_app), ''),
                    coalesce(trim(source_url), ''),
                    coalesce(trim(book_title), ''),
                    coalesce(trim(book_author), ''),
                    regexp_replace(lower(trim(content)), '\s+', ' ', 'g')
                )
            END,
            'sha256'
        ),
        'hex'
    )
WHERE content_hash IS NULL
  AND content IS NOT NULL;

WITH ranked AS (
    SELECT
        id,
        FIRST_VALUE(id) OVER (
            PARTITION BY user_id, content_hash
            ORDER BY created_at ASC, id ASC
        ) AS keep_id,
        ROW_NUMBER() OVER (
            PARTITION BY user_id, content_hash
            ORDER BY created_at ASC, id ASC
        ) AS rn
    FROM highlights
    WHERE content_hash IS NOT NULL
),
duplicate_map AS (
    SELECT id AS delete_id, keep_id
    FROM ranked
    WHERE rn > 1
)
UPDATE questions q
SET highlight_id = duplicate_map.keep_id
FROM duplicate_map
WHERE q.highlight_id = duplicate_map.delete_id;

WITH ranked AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY user_id, content_hash
            ORDER BY created_at ASC, id ASC
        ) AS rn
    FROM highlights
    WHERE content_hash IS NOT NULL
)
DELETE FROM highlights h
USING ranked r
WHERE h.id = r.id
  AND r.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS highlights_user_id_content_hash_idx
    ON highlights (user_id, content_hash)
    WHERE content_hash IS NOT NULL;

ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS perspective VARCHAR(32) NOT NULL DEFAULT 'definition',
    ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_questions_highlight_id_perspective
    ON questions (highlight_id, perspective, created_at DESC)
    WHERE highlight_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS regeneration_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    highlight_id UUID NOT NULL REFERENCES highlights(id) ON DELETE CASCADE,
    requested_from_question_id UUID REFERENCES questions(id) ON DELETE SET NULL,
    reason VARCHAR(50) NOT NULL DEFAULT 'answer_submitted',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processing_started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_regeneration_queue_status_requested_at
    ON regeneration_queue (status, requested_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_regeneration_queue_active_highlight
    ON regeneration_queue (highlight_id)
    WHERE status IN ('pending', 'processing');

UPDATE highlights h
SET
    status = 'completed',
    completed_at = COALESCE(h.completed_at, q.last_question_at),
    generation_requested_at = COALESCE(h.generation_requested_at, h.created_at)
FROM (
    SELECT highlight_id, MAX(created_at) AS last_question_at
    FROM questions
    WHERE highlight_id IS NOT NULL
    GROUP BY highlight_id
) q
WHERE h.id = q.highlight_id;

UPDATE highlights
SET generation_requested_at = COALESCE(generation_requested_at, created_at)
WHERE generation_requested_at IS NULL;

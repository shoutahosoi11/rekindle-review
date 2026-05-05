CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE highlights
    ADD COLUMN IF NOT EXISTS content_hash TEXT,
    ADD COLUMN IF NOT EXISTS source TEXT;

ALTER TABLE highlights
    ALTER COLUMN source TYPE TEXT,
    ALTER COLUMN source DROP DEFAULT,
    ALTER COLUMN source DROP NOT NULL;

DROP INDEX IF EXISTS highlights_user_id_content_hash_idx;
DROP INDEX IF EXISTS idx_highlights_user_content;

UPDATE highlights
SET source = 'share'
WHERE source = 'mobile_share';

UPDATE highlights
SET source = 'extension'
WHERE source = 'kindle';

UPDATE highlights
SET content_hash = encode(digest(regexp_replace(trim(content), '\s+', ' ', 'g'), 'sha256'), 'hex')
WHERE content IS NOT NULL
  AND trim(content) <> '';

UPDATE highlights
SET content_hash = NULL
WHERE content IS NULL
   OR trim(content) = '';

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
UPDATE highlights h
SET content_hash = NULL
FROM ranked r
WHERE h.id = r.id
  AND r.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS idx_highlights_user_content
    ON highlights (user_id, content_hash)
    WHERE content_hash IS NOT NULL;

CREATE TABLE IF NOT EXISTS rate_limit_counters (
    user_id    TEXT NOT NULL,
    bucket     TEXT NOT NULL,
    period     DATE NOT NULL,
    count      BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, bucket, period)
);

CREATE INDEX IF NOT EXISTS idx_rate_limit_period
    ON rate_limit_counters (period);

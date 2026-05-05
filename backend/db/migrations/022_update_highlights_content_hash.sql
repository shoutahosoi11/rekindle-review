DROP INDEX IF EXISTS highlights_user_id_content_hash_idx;

UPDATE highlights
SET content_hash = NULL
WHERE content_hash IS NOT NULL;

CREATE UNIQUE INDEX highlights_user_id_content_hash_idx
    ON highlights (user_id, content_hash)
    WHERE content_hash IS NOT NULL;

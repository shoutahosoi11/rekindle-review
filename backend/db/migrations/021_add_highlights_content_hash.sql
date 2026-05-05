ALTER TABLE highlights
    ADD COLUMN IF NOT EXISTS content_hash TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS highlights_user_id_content_hash_idx
    ON highlights (user_id, content_hash)
    WHERE content_hash IS NOT NULL;

ALTER TABLE highlights
    ADD COLUMN IF NOT EXISTS book_key TEXT;

CREATE INDEX IF NOT EXISTS idx_highlights_user_book_status
    ON highlights(user_id, book_key, status);

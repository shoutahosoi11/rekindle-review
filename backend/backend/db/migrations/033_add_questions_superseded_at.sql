ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS superseded_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_questions_active_by_highlight
    ON questions(highlight_id)
    WHERE superseded_at IS NULL;

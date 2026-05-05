ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS highlight_id UUID REFERENCES highlights(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_questions_user_id_highlight_id
    ON questions(user_id, highlight_id)
    WHERE highlight_id IS NOT NULL;

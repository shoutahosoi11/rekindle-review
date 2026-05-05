ALTER TABLE answers
    ALTER COLUMN is_correct SET DEFAULT FALSE;

ALTER TABLE answers
    ADD COLUMN IF NOT EXISTS score INTEGER,
    ADD COLUMN IF NOT EXISTS feedback TEXT,
    ADD COLUMN IF NOT EXISTS grader_model VARCHAR(50),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'answers_user_id_question_id_key'
    ) THEN
        ALTER TABLE answers
            ADD CONSTRAINT answers_user_id_question_id_key UNIQUE (user_id, question_id);
    END IF;
END $$;

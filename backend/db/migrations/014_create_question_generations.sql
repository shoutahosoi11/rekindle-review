CREATE TABLE question_generations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_type VARCHAR(20) NOT NULL,
    source_id UUID,
    prompt_used TEXT,
    model_used VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

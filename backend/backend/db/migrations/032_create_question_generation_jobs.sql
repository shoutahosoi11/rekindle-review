CREATE TABLE IF NOT EXISTS question_generation_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    book_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    reason TEXT NOT NULL,
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processing_started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_question_generation_jobs_active
    ON question_generation_jobs(user_id, book_key)
    WHERE status IN ('queued', 'processing');

CREATE INDEX IF NOT EXISTS idx_question_generation_jobs_enqueue_failed
    ON question_generation_jobs(user_id, created_at)
    WHERE status = 'enqueue_failed';

CREATE TABLE IF NOT EXISTS question_generation_job_highlights (
    job_id UUID NOT NULL REFERENCES question_generation_jobs(id) ON DELETE CASCADE,
    highlight_id UUID NOT NULL REFERENCES highlights(id) ON DELETE CASCADE,
    planned_question_count INT NOT NULL DEFAULT 1,
    PRIMARY KEY (job_id, highlight_id)
);

CREATE INDEX IF NOT EXISTS idx_question_generation_job_highlights_highlight
    ON question_generation_job_highlights(highlight_id);

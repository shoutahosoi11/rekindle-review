CREATE TABLE IF NOT EXISTS question_daily_budgets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  budget_date DATE NOT NULL,
  free_used INT NOT NULL DEFAULT 0,
  token_used INT NOT NULL DEFAULT 0,
  ad_views_today INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id, budget_date)
);

CREATE TABLE IF NOT EXISTS user_ad_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_count INT NOT NULL DEFAULT 3,
  earned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  used_at TIMESTAMPTZ,
  job_id UUID REFERENCES question_generation_jobs(id)
);

CREATE INDEX IF NOT EXISTS idx_user_ad_tokens_unused
  ON user_ad_tokens(user_id, earned_at)
  WHERE used_at IS NULL;

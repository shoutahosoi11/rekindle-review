ALTER TABLE questions ADD COLUMN generation_id UUID REFERENCES question_generations(id) ON DELETE SET NULL;

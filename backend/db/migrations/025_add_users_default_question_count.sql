ALTER TABLE users
ADD COLUMN default_question_count SMALLINT NOT NULL DEFAULT 3;

ALTER TABLE users
ADD CONSTRAINT users_default_question_count_check
CHECK (default_question_count BETWEEN 0 AND 10);

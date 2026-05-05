ALTER TABLE posts
ADD COLUMN body TEXT,
ADD COLUMN book_title TEXT,
ADD COLUMN question_count INTEGER NOT NULL DEFAULT 0;

CREATE TABLE post_questions (
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL,
    note TEXT,
    PRIMARY KEY (post_id, question_id)
);

CREATE INDEX idx_post_questions_post_id_sort_order
ON post_questions(post_id, sort_order);

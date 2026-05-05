-- Existing migrations already define composite primary keys for follows/likes/reposts
-- and count columns on posts. This migration only adds the missing comments.content
-- column while keeping backward compatibility with existing comments.body rows.

ALTER TABLE comments ADD COLUMN IF NOT EXISTS content TEXT;

UPDATE comments
SET content = body
WHERE content IS NULL;

ALTER TABLE comments
ALTER COLUMN content SET NOT NULL;

ALTER TABLE highlights
  ADD COLUMN IF NOT EXISTS book_order_index INT;

WITH numbered AS (
  SELECT
    id,
    ROW_NUMBER() OVER (
      PARTITION BY user_id, book_key
      ORDER BY created_at, id
    ) AS rn
  FROM highlights
  WHERE book_key IS NOT NULL
)
UPDATE highlights
SET book_order_index = numbered.rn
FROM numbered
WHERE highlights.id = numbered.id
  AND highlights.book_order_index IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_highlights_book_order_index
  ON highlights(user_id, book_key, book_order_index)
  WHERE book_order_index IS NOT NULL;

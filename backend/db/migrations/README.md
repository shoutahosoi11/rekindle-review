# Database Migrations

This project keeps backend migrations in `backend/db/migrations/` as
forward-only SQL files. The local `Makefile` runs every `*.sql` file in sorted
order with `psql`, so do not place paired `.down.sql` files in this directory.

## Security Hardening Migration

Phase 2 added:

- `031_security_hardening.sql`

It is intentionally a single forward-only file to match the existing migration
layout.

### What It Changes

- Ensures `pgcrypto` exists for SHA-256 hashing.
- Ensures `highlights.content_hash TEXT` exists.
- Ensures `highlights.source TEXT` exists and is nullable.
- Renames legacy source values:
  - `mobile_share` -> `share`
  - `kindle` -> `extension`
- Backfills `content_hash` for existing non-empty highlight content.
- Leaves `content_hash` as `NULL` for empty content and duplicate rows that
  would violate the unique index.
- Leaves existing `source IS NULL` rows as `NULL`.
- Creates the partial unique index:
  - `idx_highlights_user_content` on `(user_id, content_hash)`
  - only where `content_hash IS NOT NULL`
- Creates `rate_limit_counters` for per-user ingest rate limits.

### Preflight Checklist

Record these before running the migration:

```sql
SELECT COUNT(*) AS highlights_count FROM highlights;

SELECT source, COUNT(*)
FROM highlights
GROUP BY source
ORDER BY source NULLS FIRST;

SELECT COUNT(*) AS duplicate_hash_candidates
FROM (
    SELECT user_id, encode(digest(regexp_replace(trim(content), '\s+', ' ', 'g'), 'sha256'), 'hex') AS content_hash
    FROM highlights
    WHERE content IS NOT NULL
      AND trim(content) <> ''
) candidates
GROUP BY user_id, content_hash
HAVING COUNT(*) > 1;
```

Also confirm:

- Neon automatic backups / PITR are enabled.
- The migration connection string is the Neon direct connection string, not the
  pooled runtime connection string.
- Application deploy for Phase 2 is ready, because the new ingest paths expect
  `rate_limit_counters`.
- You have a rollback window and a recorded timestamp.

### Run on Neon Console

1. Open the Neon project.
2. Select the target branch and database.
3. Open SQL Editor.
4. Paste `backend/db/migrations/031_security_hardening.sql`.
5. Run it once.
6. Save the query result and timestamp in the deploy notes.

### Run with psql

Use the Neon direct connection string:

```sh
MIGRATION_DATABASE_URL="postgresql://USER:PASSWORD@HOST.neon.tech/DB_NAME?sslmode=require"

psql "${MIGRATION_DATABASE_URL}" \
  -v ON_ERROR_STOP=1 \
  -f backend/db/migrations/031_security_hardening.sql
```

For a fresh database, run all migrations in order:

```sh
MIGRATION_DATABASE_URL="postgresql://USER:PASSWORD@HOST.neon.tech/DB_NAME?sslmode=require"

for file in $(ls backend/db/migrations/*.sql | sort); do
  echo "==> ${file}"
  psql "${MIGRATION_DATABASE_URL}" -v ON_ERROR_STOP=1 -f "${file}"
done
```

### Run with golang-migrate

The repository does not currently use `golang-migrate` naming conventions.
If you choose to use it operationally, copy the SQL into an external
deployment-only migration directory with a single `up` migration and keep that
directory out of `backend/db/migrations/`.

Example external layout:

```text
ops/migrations/031_security_hardening.up.sql
```

Then run:

```sh
migrate \
  -path ops/migrations \
  -database "${MIGRATION_DATABASE_URL}" \
  up
```

### Postflight Checks

```sql
SELECT COUNT(*) AS highlights_count FROM highlights;

SELECT source, COUNT(*)
FROM highlights
GROUP BY source
ORDER BY source NULLS FIRST;

SELECT COUNT(*) AS hashed_highlights
FROM highlights
WHERE content_hash IS NOT NULL;

SELECT indexname, indexdef
FROM pg_indexes
WHERE indexname = 'idx_highlights_user_content';

SELECT to_regclass('public.rate_limit_counters') AS rate_limit_table;
```

Smoke-test application behavior:

- Insert or import a new highlight and confirm `content_hash` is set.
- Import the same normalized text twice and confirm duplicate handling returns
  the existing highlight rather than creating a second row.
- Send more than the configured ingest limit to confirm `429` and
  `Retry-After: 86400`.
- Confirm existing highlight list and question flows still load.

### Manual Rollback SQL

Prefer Neon PITR for production rollback. Use SQL rollback only after confirming
the application version no longer depends on these columns or tables.

```sql
DROP INDEX IF EXISTS idx_highlights_user_content;

DROP TABLE IF EXISTS rate_limit_counters;

UPDATE highlights
SET source = 'mobile_share'
WHERE source = 'share';

UPDATE highlights
SET source = 'kindle'
WHERE source = 'extension';

ALTER TABLE highlights
    DROP COLUMN IF EXISTS content_hash;

ALTER TABLE highlights
    DROP COLUMN IF EXISTS source;
```

If you used PITR, redeploy the previous application revision immediately after
restoring the database branch.

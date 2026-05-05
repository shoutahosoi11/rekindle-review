-- name: IncrementRateLimitCounter :one
INSERT INTO rate_limit_counters (user_id, bucket, period, count)
VALUES ($1, $2, CURRENT_DATE, 1)
ON CONFLICT (user_id, bucket, period)
DO UPDATE SET count = rate_limit_counters.count + 1,
              updated_at = NOW()
RETURNING count;

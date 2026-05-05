# Incident Runbook

Use this when production behavior looks unsafe or expensive. The first goal is
to stop the blast radius, then preserve enough evidence to fix the root cause.

## Cloud Run Scales Unexpectedly

First actions:

- Confirm current instance count, request count, latency, and error rate.
- Set `--max-instances=0` if cost or downstream pressure is active.
- If one revision caused the issue, route traffic back to the previous known
  good revision.
- Check Cloudflare logs for request source, path, and method concentration.
- Check Cloud Run logs for the top failing endpoints.

Commands:

```sh
gcloud run services update api-service \
  --region=asia-northeast1 \
  --max-instances=0

gcloud run revisions list \
  --service=api-service \
  --region=asia-northeast1
```

## Rate Limit Thresholds Are Being Hit

First actions:

- Check whether the source is many IPs or a small number of users.
- Raise Cloudflare challenge/blocking only for abusive paths first.
- Keep backend per-user limits in place.
- Confirm `Retry-After` is present for backend `429` responses.
- Preserve `rate_limit_counters` samples for investigation.

Useful query:

```sql
SELECT user_id, bucket, period, count, updated_at
FROM rate_limit_counters
ORDER BY updated_at DESC
LIMIT 50;
```

## Cloudflare Detects Abnormal Traffic

First actions:

- Switch suspicious high-volume rules from log to challenge.
- Block clearly abusive paths.
- Confirm the Cloud Run direct URL is not bypassing Cloudflare.
- Temporarily lower Cloud Run `--max-instances` if origin pressure is rising.
- Export Cloudflare security events for the incident record.

## Neon Database Load Is Abnormal

First actions:

- Check active connections and slow queries in Neon.
- Reduce Cloud Run `--max-instances`.
- Disable or slow question worker polling if writes are the pressure source.
- Pause non-essential ingest or sync workflows at the edge.
- Use PITR only after confirming data corruption or unsafe migration effects.

Useful checks:

```sql
SELECT state, COUNT(*)
FROM pg_stat_activity
GROUP BY state;

SELECT relname, n_live_tup, n_dead_tup
FROM pg_stat_user_tables
ORDER BY n_live_tup DESC;
```

## Question Generation Spend Spikes

First actions:

- Set `QUESTION_SYNC_DAILY_LIMIT` lower on Cloud Run.
- Increase `QUESTION_WORKER_REQUEST_INTERVAL_MS`.
- Stop the worker revision if generation is still running.
- Check `question_generations` and `user_daily_generation_counts`.
- Confirm mobile/frontend are not repeatedly triggering `/api/questions/sync`.

Useful query:

```sql
SELECT date, SUM(count) AS generated_count
FROM user_daily_generation_counts
GROUP BY date
ORDER BY date DESC;
```

## After Stabilization

- Record incident start/end times.
- Record which limits were changed.
- Keep relevant Cloud Run, Cloudflare, Neon, and application logs.
- Open a follow-up issue with the root cause and permanent fix.
- Restore normal limits only after replaying the triggering path in staging.

# Budget Alerts And Quota Controls

Cloud Run `--max-instances=10` is the runtime blast-radius cap. GCP Budget
Alerts are the human alerting layer when spend still moves unexpectedly.

## Budget Alert Setup

Recommended starting monthly budget:

- Development / staging: JPY 3,000 to 5,000.
- Early production: JPY 10,000 to 20,000.

Choose the number based on expected Cloud Run, Artifact Registry, Secret
Manager, Cloud Storage, logging, and Gemini usage. Set alert thresholds at:

- 50%
- 80%
- 100%
- 120%

Create the budget in Google Cloud Console:

```text
Billing -> Budgets & alerts -> Create budget
```

Scope it to the project running Cloud Run and Gemini. Route notifications to
the operator email or a shared alert channel.

## Emergency Stop: Max Instances 0

Use this when traffic or spend is actively out of control:

```sh
PROJECT_ID="your-gcp-project"
REGION="asia-northeast1"
SERVICE="api-service"

gcloud run services update "${SERVICE}" \
  --project="${PROJECT_ID}" \
  --region="${REGION}" \
  --max-instances=0
```

This stops new scaling and makes the API unavailable once existing instances
drain.

## Emergency Stop: Route Traffic Away

If a known good revision exists, move traffic:

```sh
gcloud run services update-traffic "${SERVICE}" \
  --project="${PROJECT_ID}" \
  --region="${REGION}" \
  --to-revisions="GOOD_REVISION=100"
```

To stop a bad revision from receiving traffic:

```sh
gcloud run services update-traffic "${SERVICE}" \
  --project="${PROJECT_ID}" \
  --region="${REGION}" \
  --to-revisions="BAD_REVISION=0,GOOD_REVISION=100"
```

## Gemini API Quotas

Even if Gemini is not the immediate source of spend, configure quotas before
expanding generation workflows:

- Daily request quota per project.
- Per-minute request quota.
- Budget alerts that include Gemini spend.
- Separate staging and production projects if possible.

Application-level controls already exist:

- `QUESTION_SYNC_DAILY_LIMIT`
- `QUESTION_SYNC_PER_TRIGGER_LIMIT`
- `QUESTION_WORKER_MAX_QUESTIONS_PER_CALL`
- `QUESTION_WORKER_REQUEST_INTERVAL_MS`

These controls should be set intentionally in Cloud Run environment variables.

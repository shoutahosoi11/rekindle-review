# Cloud Run Deployment Guardrails

This service is deployed to Cloud Run by `.github/workflows/deploy-api.yml`.
The API is an Echo HTTP server on port `8080` with separate lightweight
probes:

- `/health`: process liveness only.
- `/ready`: readiness, including database connectivity.

## Required Limits

Set `--max-instances=10` on the API service. This is the final cost guardrail if
application rate limits, Cloudflare rules, or client-side behavior fail. With a
database-backed app and LLM-triggering workflows, an unbounded Cloud Run service
can turn traffic spikes into database pressure and API spend quickly.

Set `--concurrency=80`. Echo can handle many concurrent requests, and `80` is a
reasonable Cloud Run default for mixed API traffic. It keeps instance count
lower during ordinary bursts while still allowing `--max-instances=10` to cap
total concurrent in-flight requests at about `800`.

Recommended starting resources:

- CPU: `1`
- Memory: `512Mi`
- Request timeout: `60s`
- Min instances: `0` for cost control unless cold starts become a product issue.
- Max instances: `10`
- Concurrency: `80`

Increase memory only after observing Cloud Run memory metrics. Increase timeout
only for endpoints that truly need it; ingest and question sync should return
quickly and let workers do longer-running work.

## Health Check

Use liveness checks for process health:

```text
GET /health
```

Use readiness checks before routing real traffic or during manual verification:

```text
GET /ready
```

`/health` should not require Firebase Auth and should not touch expensive
dependencies. `/ready` may return `503` when the database is unavailable.

## Manual Deploy Snippet

```sh
PROJECT_ID="your-gcp-project"
REGION="asia-northeast1"
SERVICE="api-service"
IMAGE="asia-northeast1-docker.pkg.dev/${PROJECT_ID}/ai-study-tool/${SERVICE}:TAG"
RUNTIME_SA="cloud-run-api@${PROJECT_ID}.iam.gserviceaccount.com"

gcloud run deploy "${SERVICE}" \
  --project="${PROJECT_ID}" \
  --region="${REGION}" \
  --image="${IMAGE}" \
  --port=8080 \
  --service-account="${RUNTIME_SA}" \
  --max-instances=10 \
  --concurrency=80 \
  --cpu=1 \
  --memory=512Mi \
  --timeout=60s \
  --set-env-vars="CORS_ALLOWED_ORIGINS=https://YOUR_FRONTEND_DOMAIN,GCS_BUCKET_NAME=YOUR_BUCKET,GCS_SIGNING_SERVICE_ACCOUNT=${RUNTIME_SA}" \
  --set-secrets="DATABASE_URL=DATABASE_URL:latest,GEMINI_API_KEY=GEMINI_API_KEY:latest"
```

If Cloudflare is the only public entrypoint, deploy with authenticated invoker
instead of public access:

```sh
gcloud run deploy "${SERVICE}" \
  --project="${PROJECT_ID}" \
  --region="${REGION}" \
  --image="${IMAGE}" \
  --no-allow-unauthenticated \
  --max-instances=10 \
  --concurrency=80 \
  --timeout=60s
```

## GitHub Actions Update

The deploy workflow should include the same flags:

```yaml
flags: >-
  --port=8080
  --allow-unauthenticated
  --service-account=${{ vars.CLOUD_RUN_RUNTIME_SERVICE_ACCOUNT }}
  --max-instances=10
  --concurrency=80
  --cpu=1
  --memory=512Mi
  --timeout=60s
```

When Cloudflare authenticated origin access is ready, replace
`--allow-unauthenticated` with the chosen authenticated invoker setup.

## Verification

```sh
gcloud run services describe "${SERVICE}" \
  --project="${PROJECT_ID}" \
  --region="${REGION}" \
  --format="value(spec.template.spec.containerConcurrency,spec.template.metadata.annotations.autoscaling.knative.dev/maxScale)"

curl --fail "https://YOUR_SERVICE_URL/health"
curl --fail "https://YOUR_SERVICE_URL/ready"
```

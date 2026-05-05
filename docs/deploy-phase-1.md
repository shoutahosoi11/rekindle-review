# Phase 1 Deploy: Cloud Run + Neon + Firebase Auth + Cloud Storage

This guide sets up the first production-like environment:

- Cloud Run: `api-service`
- Neon PostgreSQL: managed Postgres reached through a standard `DATABASE_URL`
- Secret Manager: `DATABASE_URL` and `GEMINI_API_KEY`
- Firebase Auth: frontend issues ID tokens, backend verifies them
- Cloud Storage: upload/download signed URLs
- GitHub Actions: build, test, push image, deploy Cloud Run

The backend reads a standard lib/pq `DATABASE_URL`. For production, store the
Neon connection string in Secret Manager. Cloud SQL connectors, Cloud SQL Auth
Proxy, and `--add-cloudsql-instances` are not part of the current deployment.

## 1. Create Google Cloud resources

Choose values:

```sh
PROJECT_ID="your-gcp-project"
REGION="asia-northeast1"
SERVICE="api-service"
GAR_REPOSITORY="ai-study-tool"
BUCKET_NAME="${PROJECT_ID}-ai-study-tool-uploads"
```

Enable APIs:

```sh
gcloud services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  secretmanager.googleapis.com \
  iamcredentials.googleapis.com \
  storage.googleapis.com \
  cloudbuild.googleapis.com \
  --project="${PROJECT_ID}"
```

Create Artifact Registry:

```sh
gcloud artifacts repositories create "${GAR_REPOSITORY}" \
  --repository-format=docker \
  --location="${REGION}" \
  --project="${PROJECT_ID}"
```

Create Cloud Storage bucket:

```sh
gcloud storage buckets create "gs://${BUCKET_NAME}" \
  --location="${REGION}" \
  --uniform-bucket-level-access \
  --project="${PROJECT_ID}"
```

Apply CORS for browser uploads through signed URLs:

```sh
cp deploy/storage-cors.example.json /tmp/storage-cors.json
# Replace YOUR_FRONTEND_DOMAIN in /tmp/storage-cors.json.
gcloud storage buckets update "gs://${BUCKET_NAME}" \
  --cors-file=/tmp/storage-cors.json \
  --project="${PROJECT_ID}"
```

## 2. Create Neon PostgreSQL

Create a Neon project and database from the Neon dashboard. Use PostgreSQL 16
for parity with local Docker development.

Recommended connection choices:

- Runtime API: use Neon's pooled connection string unless a transaction-heavy
  code path needs the direct endpoint.
- Migrations: use Neon's direct connection string.
- SSL: keep `sslmode=require` in production.

Example:

```text
postgresql://USER:PASSWORD@HOST.neon.tech/DB_NAME?sslmode=require
```

## 3. Store secrets

Create Secret Manager secrets:

```sh
DATABASE_URL="postgresql://USER:PASSWORD@HOST.neon.tech/DB_NAME?sslmode=require"

printf '%s' "${DATABASE_URL}" | gcloud secrets create DATABASE_URL \
  --data-file=- \
  --project="${PROJECT_ID}"

printf '%s' 'PASTE_GEMINI_API_KEY_HERE' | gcloud secrets create GEMINI_API_KEY \
  --data-file=- \
  --project="${PROJECT_ID}"
```

If a secret already exists, add a new version:

```sh
printf '%s' "${DATABASE_URL}" | gcloud secrets versions add DATABASE_URL \
  --data-file=- \
  --project="${PROJECT_ID}"

printf '%s' 'PASTE_GEMINI_API_KEY_HERE' | gcloud secrets versions add GEMINI_API_KEY \
  --data-file=- \
  --project="${PROJECT_ID}"
```

## 4. Service accounts and IAM

Runtime service account:

```sh
gcloud iam service-accounts create cloud-run-api \
  --display-name="Cloud Run API runtime" \
  --project="${PROJECT_ID}"

RUNTIME_SA="cloud-run-api@${PROJECT_ID}.iam.gserviceaccount.com"
```

Grant runtime permissions:

```sh
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${RUNTIME_SA}" \
  --role="roles/secretmanager.secretAccessor"

gcloud storage buckets add-iam-policy-binding "gs://${BUCKET_NAME}" \
  --member="serviceAccount:${RUNTIME_SA}" \
  --role="roles/storage.objectAdmin"

gcloud iam service-accounts add-iam-policy-binding "${RUNTIME_SA}" \
  --member="serviceAccount:${RUNTIME_SA}" \
  --role="roles/iam.serviceAccountTokenCreator" \
  --project="${PROJECT_ID}"
```

The final binding lets Cloud Storage generate V4 signed URLs from Cloud Run.

Deploy service account for GitHub Actions:

```sh
gcloud iam service-accounts create github-deployer \
  --display-name="GitHub Actions deployer" \
  --project="${PROJECT_ID}"

DEPLOYER_SA="github-deployer@${PROJECT_ID}.iam.gserviceaccount.com"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${DEPLOYER_SA}" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${DEPLOYER_SA}" \
  --role="roles/artifactregistry.writer"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${DEPLOYER_SA}" \
  --role="roles/iam.serviceAccountUser"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${DEPLOYER_SA}" \
  --role="roles/secretmanager.secretAccessor"
```

Set up GitHub Workload Identity Federation using the official
`google-github-actions/auth` setup. Restrict the provider to your repository,
then store these GitHub secrets:

- `GCP_WORKLOAD_IDENTITY_PROVIDER`
- `GCP_DEPLOYER_SERVICE_ACCOUNT`

## 5. GitHub repository variables

Set these GitHub Actions variables:

| Variable | Example |
| --- | --- |
| `GCP_PROJECT_ID` | `your-gcp-project` |
| `GCP_REGION` | `asia-northeast1` |
| `CLOUD_RUN_API_SERVICE` | `api-service` |
| `ARTIFACT_REGISTRY_REPOSITORY` | `ai-study-tool` |
| `CLOUD_RUN_RUNTIME_SERVICE_ACCOUNT` | `cloud-run-api@project.iam.gserviceaccount.com` |
| `CORS_ALLOWED_ORIGINS` | `https://YOUR_FRONTEND_DOMAIN` |
| `GCS_BUCKET_NAME` | `your-bucket-name` |
| `GCS_SIGNING_SERVICE_ACCOUNT` | `cloud-run-api@project.iam.gserviceaccount.com` |

## 6. Run migrations

Run migrations against the Neon direct connection string:

```sh
MIGRATION_DATABASE_URL="postgresql://USER:PASSWORD@HOST.neon.tech/DB_NAME?sslmode=require"

for file in backend/db/migrations/*.sql; do
  psql "${MIGRATION_DATABASE_URL}" -f "$file"
done
```

The production schema must include `030_add_user_daily_generation_count.sql`
before enabling question sync in production.

## 7. First deploy

Push to `main` or run the `Deploy API to Cloud Run` workflow manually.

The workflow injects `DATABASE_URL` / `GEMINI_API_KEY` from Secret Manager and
does not attach a Cloud SQL instance. After the service exists, allow browser
access to the public API. The backend still requires Firebase ID tokens on
`/api/*`.

```sh
gcloud run services add-iam-policy-binding "${SERVICE}" \
  --region="${REGION}" \
  --member="allUsers" \
  --role="roles/run.invoker" \
  --project="${PROJECT_ID}"
```

Add the deployed frontend domain to:

- Firebase Auth authorized domains
- `CORS_ALLOWED_ORIGINS`
- Cloud Storage bucket CORS

## Notes

- Locally, `FIREBASE_CREDENTIALS_PATH` can point at a service account JSON.
- On Cloud Run, Firebase Admin uses the runtime service account through Application Default Credentials.
- Signed uploads must send the exact `Content-Type` returned by `/api/storage/signed-urls/upload`.
- For staging without Gemini cost, set `USE_GEMINI_MOCK=true`.
- To test tighter generation budgets, set `QUESTION_SYNC_DAILY_LIMIT` and `QUESTION_SYNC_PER_TRIGGER_LIMIT` to small values.
- Security deployment guardrails live under `docs/deployment/`:
  - `cloud-run.md`
  - `cloudflare.md`
  - `storage-security-checklist.md`
  - `budget-alert.md`
  - `incident-runbook.md`

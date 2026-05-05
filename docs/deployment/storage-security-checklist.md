# Cloud Storage Security Checklist

This project is designed to use signed URLs for uploads/downloads. Buckets
should stay private; public object access should not be required.

## Baseline

- Enable Uniform bucket-level access.
- Keep Public Access Prevention enabled when possible.
- Confirm `allUsers` has no role on the bucket.
- Confirm `allAuthenticatedUsers` has no role on the bucket.
- Grant object permissions only to the Cloud Run runtime service account.
- Use a signing service account only for V4 signed URL generation.
- Keep CORS limited to the production frontend origin.

## Verification Commands

```sh
PROJECT_ID="your-gcp-project"
BUCKET_NAME="your-bucket-name"

gcloud storage buckets describe "gs://${BUCKET_NAME}" \
  --project="${PROJECT_ID}" \
  --format="yaml(uniformBucketLevelAccess,publicAccessPrevention,cors_config)"

gcloud storage buckets get-iam-policy "gs://${BUCKET_NAME}" \
  --project="${PROJECT_ID}"
```

Look for and remove:

```text
allUsers
allAuthenticatedUsers
```

## Minimal IAM Shape

Runtime service account:

- `roles/storage.objectAdmin` on the bucket if the API must create, read, and
  delete user objects.
- Consider splitting upload and read roles later if object lifecycle grows.

Signing service account:

- `roles/iam.serviceAccountTokenCreator` granted only to the runtime identity
  that signs URLs.

## Before Enabling New Storage Features

- Confirm signed URL expiry is short enough for the workflow.
- Confirm upload content type is validated by the backend.
- Confirm object names cannot escape the intended user prefix.
- Confirm frontend and mobile never receive broad bucket credentials.
- Confirm lifecycle retention and deletion policy match product expectations.

## Future Upload Path Decision

The current design uses signed URLs so large upload/download traffic goes
directly between the client and Cloud Storage. This keeps Cloud Run cost and
latency lower, and it avoids making API instances carry file bodies.

An alternative is uploading through the backend first, then writing to Cloud
Storage server-side. That gives the backend one place to enforce deeper content
validation, malware scanning, quota accounting, and metadata normalization, but
it increases Cloud Run bandwidth, memory pressure, timeout risk, and request
cost. Treat that as a separate design change, not a small routing tweak.

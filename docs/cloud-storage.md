# Cloud Storage Source Support

Distributed Encoder supports video sources that live in object storage rather
than on a UNC/NFS share.  When a source has a `cloud_uri`, the agent downloads
the file to a local temporary path before encoding and removes it afterwards.

## Supported URI schemes

| Scheme    | Provider                   | Example                                     |
|-----------|----------------------------|---------------------------------------------|
| `s3://`   | Amazon S3 (or S3-compatible) | `s3://my-bucket/videos/movie.mkv`          |
| `gs://`   | Google Cloud Storage       | `gs://my-bucket/videos/movie.mkv`           |
| `az://`   | Azure Blob Storage         | `az://my-container/videos/movie.mkv`        |

## Environment variables

### Amazon S3

| Variable                | Required | Description                                         |
|-------------------------|----------|-----------------------------------------------------|
| `AWS_ACCESS_KEY_ID`     | No*      | IAM access key ID                                   |
| `AWS_SECRET_ACCESS_KEY` | No*      | IAM secret access key                               |
| `AWS_REGION`            | No*      | AWS region (e.g. `us-east-1`)                       |
| `AWS_SESSION_TOKEN`     | No       | Session token for temporary credentials             |

\* When running on an EC2 instance or ECS task with an instance/task IAM role
attached, credentials are automatically retrieved from the instance metadata
service (IMDS) — no environment variables are needed.

**Minimum IAM permissions** for the agent's identity:
```json
{
  "Effect": "Allow",
  "Action": ["s3:GetObject"],
  "Resource": "arn:aws:s3:::my-bucket/*"
}
```

### Google Cloud Storage

| Variable                        | Required | Description                                  |
|---------------------------------|----------|----------------------------------------------|
| `GOOGLE_APPLICATION_CREDENTIALS`| No*      | Path to a service account JSON key file      |

\* When running on GCE/GKE with workload identity or a service account attached
to the instance, credentials are provided automatically.

**Minimum IAM role** for the agent's service account:
`roles/storage.objectViewer` on the relevant bucket.

### Azure Blob Storage

| Variable                 | Required | Description                                      |
|--------------------------|----------|--------------------------------------------------|
| `AZURE_STORAGE_ACCOUNT`  | Yes      | Storage account name                             |
| `AZURE_STORAGE_KEY`      | Yes      | Storage account shared access key               |

**Minimum permissions** for the shared key: read access on the relevant
container (`Storage Blob Data Reader` role if using Azure RBAC instead).

## Download–encode–upload flow

```
Controller                     Agent
    |                             |
    | TaskAssignment              |
    | (source_path = cloud_uri)   |
    |─────────────────────────────>
    |                             |
    |                             | 1. cloudstorage.NewStore(uri)
    |                             | 2. store.Download(ctx, uri, workDir/cloud_src_<name>)
    |                             | 3. Execute encode script
    |                             |    DE_SOURCE_PATH=workDir/cloud_src_<name>
    |                             | 4. os.Remove(workDir/cloud_src_<name>)
    |                             |
    | TaskResult                  |
    <─────────────────────────────|
```

The local file is stored under `{work_dir}/{task_id}/cloud_src_{filename}` and
is removed on task completion regardless of success or failure (via a deferred
`os.Remove`).

The encode script can reference the downloaded file via the `DE_SOURCE_PATH`
environment variable, which is injected by the agent before the script runs.

## Registering a cloud source

### Via the web UI

1. Open **Sources** and click **Register Source**.
2. Leave the **UNC Path** field empty and fill in the **Cloud URI** field,
   e.g. `s3://my-bucket/videos/movie.mkv`.
3. Optionally provide a **Name** override.
4. Click **Register**.

### Via the REST API

```http
POST /api/v1/sources
Content-Type: application/json

{
  "cloud_uri": "s3://my-bucket/videos/movie.mkv",
  "name": "movie.mkv"
}
```

`path` and `cloud_uri` are mutually exclusive.  Providing both returns
`400 Bad Request`.

## Database migration

Cloud URI support requires migration **017**:

```sql
-- 017_source_cloud_uri.up.sql
ALTER TABLE sources ADD COLUMN cloud_uri TEXT;
```

Run migrations with the normal `migrate` tooling or let the controller apply
them automatically on startup.

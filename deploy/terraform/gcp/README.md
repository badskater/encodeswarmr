# encodeswarmr — Terraform (GCP)

Terraform configuration for deploying encodeswarmr on Google Cloud Platform.

## Architecture

```
                        ┌─────────────────────────────────┐
                        │           GCP Project           │
                        │                                 │
  Browser / API ──────► │  Controller VM(s)  :8080 HTTP   │
  Agents       ──────► │                    :9443 gRPC   │
                        │                                 │
                        │  Agent MIG (c2-standard-8 × N)  │
                        │                                 │
                        │  Cloud SQL PostgreSQL 16        │
                        │  Filestore NFS (media/encodes)  │
                        │  Secret Manager (mTLS certs)    │
                        └─────────────────────────────────┘
```

**Standard deployment** (`enable_ha = false`):
- 1 controller VM, ZONAL Cloud SQL, no load balancers.

**HA deployment** (`enable_ha = true`):
- 2-instance controller MIG behind TCP (gRPC :9443) and HTTP (:8080) load balancers.
- REGIONAL Cloud SQL with point-in-time recovery enabled.

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.5.0
- [gcloud CLI](https://cloud.google.com/sdk/docs/install) authenticated with sufficient permissions
- A GCP project with billing enabled
- SSH key pair

## Enable required APIs

Run once per project before the first `terraform apply`:

```bash
gcloud services enable \
  compute.googleapis.com \
  sqladmin.googleapis.com \
  file.googleapis.com \
  secretmanager.googleapis.com \
  servicenetworking.googleapis.com \
  cloudresourcemanager.googleapis.com \
  iam.googleapis.com \
  --project=YOUR_PROJECT_ID
```

## Quick start

```bash
cd deploy/terraform/gcp

# 1. Copy and edit the example variables file.
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars

# 2. Initialise Terraform.
terraform init

# 3. Preview the changes.
terraform plan

# 4. Apply.
terraform apply
```

After apply, Terraform prints the controller URL, Cloud SQL connection name, and other useful values.

## Enabling HA

In `terraform.tfvars`, set:

```hcl
enable_ha = true
```

Then re-run `terraform apply`. This provisions:
- A controller instance template and a 2-instance regional MIG.
- Regional TCP proxy LB (gRPC port 9443) and regional HTTP LB (port 8080).
- REGIONAL Cloud SQL availability type with point-in-time recovery (7-day log retention, 14 daily backups).

To revert to standard deployment, set `enable_ha = false` and re-apply.

## Remote state (recommended for teams)

Create a GCS bucket and uncomment the `backend "gcs"` block in `main.tf`:

```hcl
backend "gcs" {
  bucket = "your-terraform-state-bucket"
  prefix = "encodeswarmr/gcp"
}
```

## SSH access via IAP

All VMs have no external IP. Connect through Identity-Aware Proxy:

```bash
# Standard deployment — controller
gcloud compute ssh --tunnel-through-iap --zone us-central1-a encodeswarmr-prod-controller

# HA deployment — list instances first
gcloud compute instances list --filter="tags.items=encodeswarmr-controller"
gcloud compute ssh --tunnel-through-iap --zone us-central1-a <instance-name>

# Agents
gcloud compute instances list --filter="tags.items=encodeswarmr-agent"
gcloud compute ssh --tunnel-through-iap --zone us-central1-a <instance-name>
```

## Cost estimate (us-central1, approximate)

### Standard deployment

| Resource | Spec | $/month |
|---|---|---|
| Controller VM | e2-standard-2 | ~$50 |
| Agent VMs (×2) | c2-standard-8 | ~$560 |
| Cloud SQL | db-custom-2-8192, 20 GB SSD | ~$130 |
| Filestore | BASIC_HDD 1 TiB | ~$200 |
| Cloud NAT | per-VM hour + data | ~$10 |
| **Total** | | **~$950/month** |

### HA deployment (additional costs)

| Resource | Additional cost |
|---|---|
| 2nd controller VM | ~$50 |
| HTTP + TCP LBs | ~$30 |
| Cloud SQL HA replica | ~$130 |
| **Additional** | **~$210/month** |

Costs are estimates. Use the [GCP Pricing Calculator](https://cloud.google.com/products/calculator) for accurate figures.

## Cleanup

```bash
terraform destroy
```

Note: `deletion_protection = true` is set on the Cloud SQL instance. To destroy it, first set `deletion_protection = false` and apply, then destroy:

```bash
# Temporarily disable deletion protection
terraform apply -var="..." # after editing deletion_protection in database.tf
terraform destroy
```

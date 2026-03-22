# Terraform — AWS Deployment

Terraform scripts for deploying `encodeswarmr` on AWS in either standard
(single-controller) or HA (multi-controller) mode.

---

## Architecture

```
                          ┌─────────────────────────────────────────────┐
                          │                  AWS VPC                     │
                          │  10.10.0.0/16                                │
                          │                                              │
  Internet ──────────────►│  ┌──────────┐    ┌──────────┐               │
  (HA only)               │  │  Public  │    │  Public  │               │
                          │  │ Subnet 1 │    │ Subnet 2 │               │
                          │  │  (AZ-a)  │    │  (AZ-b)  │               │
                          │  │  ALB/NLB │    │  ALB/NLB │               │
                          │  │  NAT GW  │    │  NAT GW  │               │
                          │  └────┬─────┘    └────┬─────┘               │
                          │       │               │                      │
                          │  ┌────▼─────┐    ┌────▼─────┐               │
                          │  │ Private  │    │ Private  │               │
                          │  │ Subnet 1 │    │ Subnet 2 │               │
                          │  │  (AZ-a)  │    │  (AZ-b)  │               │
                          │  │          │    │          │               │
                          │  │ ┌──────┐ │    │ ┌──────┐ │               │
                          │  │ │Ctrl-1│ │    │ │Ctrl-2│ │  (HA only)   │
                          │  │ │:8080 │ │    │ │:8080 │ │               │
                          │  │ │:9443 │ │    │ │:9443 │ │               │
                          │  │ └──────┘ │    │ └──────┘ │               │
                          │  │          │    │          │               │
                          │  │ ┌──────┐ │    │ ┌──────┐ │               │
                          │  │ │Agent │ │    │ │Agent │ │               │
                          │  │ │(gRPC)│ │    │ │(gRPC)│ │               │
                          │  │ └──────┘ │    │ └──────┘ │               │
                          │  │          │    │          │               │
                          │  │ ┌──────┐ │    │ ┌──────┐ │               │
                          │  │ │  RDS │ │    │ │ RDS  │ │  (Multi-AZ)  │
                          │  │ │ PG16 │ │    │ │Stby  │ │               │
                          │  │ └──────┘ │    │ └──────┘ │               │
                          │  │          │    │          │               │
                          │  │ ┌──────────────────────┐ │               │
                          │  │ │         EFS          │ │               │
                          │  │ │  /media /encodes /tmp│ │               │
                          │  │ └──────────────────────┘ │               │
                          │  └──────────┘    └──────────┘               │
                          └─────────────────────────────────────────────┘
```

### Standard deployment (`enable_ha = false`)

| Component | Count | Notes |
|-----------|-------|-------|
| Controller | 1 | EC2 in ASG (replacement-only) |
| Agents | `agent_count` | EC2 in ASG |
| RDS PostgreSQL 16 | 1 | Single-AZ |
| EFS | 1 | Shared across AZs |
| NAT Gateway | 1 | Single AZ |
| ALB / NLB | 0 | No load balancer |

### HA deployment (`enable_ha = true`)

| Component | Count | Notes |
|-----------|-------|-------|
| Controller | 2 | One per AZ in ASG |
| Agents | `agent_count` | EC2 in ASG, spread across AZs |
| RDS PostgreSQL 16 | 1 primary + 1 standby | Multi-AZ automatic failover |
| EFS | 1 | Mount targets in each AZ |
| NAT Gateway | 2 | One per AZ for AZ-independent egress |
| ALB | 1 | HTTP web UI / REST API |
| NLB | 1 | gRPC TCP passthrough (mTLS preserved) |

---

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.6.0
- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html) >= 2.0 configured with credentials
- An AWS account with permissions to create VPC, EC2, RDS, EFS, IAM, SSM, and ELB resources
- An existing EC2 key pair in the target region (optional — SSM Session Manager works without it)

---

## Quick start — standard deployment

```bash
cd deploy/terraform/aws

# 1. Copy and edit the variables file
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars — set db_password and controller_session_secret at minimum

# 2. Initialise Terraform
terraform init

# 3. Preview the plan
terraform plan

# 4. Apply
terraform apply
```

After `apply` completes, Terraform prints the outputs including the controller URL,
RDS endpoint, EFS ID, and SSM commands for connecting to instances.

---

## HA deployment

Set `enable_ha = true` in `terraform.tfvars` before running `terraform apply`.

```hcl
# terraform.tfvars
enable_ha                = true
environment              = "prod"
db_instance_class        = "db.r6g.large"
db_backup_retention_days = 14
agent_count              = 4
```

HA mode adds:
- A second controller instance in the second AZ
- RDS Multi-AZ with automatic standby failover
- ALB for the HTTP web UI / REST API
- NLB for gRPC TCP passthrough (preserves mTLS end-to-end)
- A second NAT gateway so private subnets survive an AZ failure

---

## Upgrading to a new version

Update `encodeswarmr_version` in `terraform.tfvars` and run `terraform apply`.
The ASG instance refresh replaces instances one at a time (50 % minimum healthy).

```bash
terraform apply -var="encodeswarmr_version=1.1.0"
```

---

## Remote state (recommended for teams)

Uncomment the `backend "s3"` block in `main.tf` and create the S3 bucket and
DynamoDB table before running `terraform init`:

```bash
aws s3api create-bucket --bucket your-terraform-state-bucket --region us-east-1
aws dynamodb create-table \
  --table-name terraform-state-lock \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST
```

---

## Connecting to instances

Instances run in private subnets. Use AWS SSM Session Manager (no port 22 required):

```bash
# Find a controller instance ID
aws ec2 describe-instances \
  --filters "Name=tag:Role,Values=controller" "Name=instance-state-name,Values=running" \
  --query "Reservations[*].Instances[*].[InstanceId,PrivateIpAddress]" \
  --output table

# Open a shell session
aws ssm start-session --target i-0123456789abcdef0
```

To use SSH instead, set `ssh_key_name` in `terraform.tfvars` and add your IP to
`allowed_ssh_cidrs`.

---

## Cost estimate

Costs are approximate for `us-east-1` (March 2026). Actual costs depend on
data transfer, EFS usage, and instance run time.

### Standard (dev/test, enable_ha = false)

| Resource | Spec | Est. monthly |
|----------|------|-------------|
| Controller EC2 | t3.medium | ~$30 |
| Agent EC2 x2 | c5.2xlarge | ~$280 |
| RDS | db.t3.medium Single-AZ | ~$50 |
| EFS | 100 GiB bursting | ~$30 |
| NAT Gateway | 1 | ~$35 |
| **Total** | | **~$425/mo** |

### HA (production, enable_ha = true)

| Resource | Spec | Est. monthly |
|----------|------|-------------|
| Controller EC2 x2 | t3.large | ~$120 |
| Agent EC2 x4 | c5.2xlarge | ~$560 |
| RDS | db.r6g.large Multi-AZ | ~$380 |
| EFS | 500 GiB elastic | ~$150 |
| NAT Gateway x2 | 2 | ~$70 |
| ALB + NLB | 2 load balancers | ~$35 |
| **Total** | | **~$1,315/mo** |

Stop or terminate instances when not encoding to reduce costs.

---

## Cleanup

```bash
terraform destroy
```

Note: RDS deletion protection is enabled in `prod` environments. Disable it first:

```bash
terraform apply -var="environment=prod" # ensure deletion_protection can be set
# Then in the AWS console or via CLI, disable deletion protection on the RDS instance
terraform destroy
```

---

## File reference

| File | Purpose |
|------|---------|
| `main.tf` | Terraform block, AWS provider, data sources |
| `variables.tf` | All input variables with descriptions and defaults |
| `networking.tf` | VPC, subnets, IGW, NAT gateways, route tables, DB subnet group |
| `security.tf` | Security groups for ALB, controller, agent, database, EFS |
| `database.tf` | RDS PostgreSQL 16, parameter group, monitoring IAM role |
| `storage.tf` | EFS file system, mount targets, access points, backup policy |
| `certs.tf` | mTLS CA, controller cert, agent cert, SSM Parameter Store |
| `controller.tf` | Controller IAM role, launch template, ASG, user-data |
| `agents.tf` | Agent IAM role, launch template, ASG, user-data |
| `loadbalancer.tf` | ALB (HTTP) and NLB (gRPC) — HA mode only |
| `outputs.tf` | Controller URL, gRPC endpoint, RDS endpoint, EFS ID, SSH commands |
| `terraform.tfvars.example` | Example variable values — copy to `terraform.tfvars` |

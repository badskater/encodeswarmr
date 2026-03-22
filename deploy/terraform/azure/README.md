# Terraform — Azure Deployment

Deploys the encodeswarmr stack on Microsoft Azure.
Supports standard (single-controller) and HA (dual-controller + load balancer) modes,
toggled by the `enable_ha` variable.

## Architecture

```
Internet
    |
    +-- [Public IP / Load Balancer (HA)]
            |
    [Controller VM(s)]  ───── [Azure Files NFS/SMB] ──── [Agent VMSS]
            |
    [PostgreSQL Flexible Server]   [Azure Key Vault]
```

**Standard mode** (`enable_ha = false`)
- 1 controller VM (Standard_D2s_v5)
- Agent VMSS with configurable instance count
- Single-zone PostgreSQL Flexible Server
- LRS storage replication

**HA mode** (`enable_ha = true`)
- 2 controller VMs across availability zones 1 and 2
- Standard Azure Load Balancer (HTTP :8080, gRPC :9443)
- Zone-redundant PostgreSQL Flexible Server with standby in zone 2
- ZRS storage replication
- Zone-balanced VMSS for agents

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.6.0
- [Azure CLI](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli) >= 2.50.0
- An active Azure subscription
- SSH key pair on your local machine

## Quick Start

### 1. Authenticate with Azure

```bash
az login
az account set --subscription "<your-subscription-id>"
```

### 2. Configure variables

```bash
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:
- Set `db_admin_password` to a strong password, or export it as an environment variable:
  ```bash
  export TF_VAR_db_admin_password="<strong-password>"
  ```
- Set `ssh_public_key_path` to your SSH public key path.
- Restrict `allowed_ssh_cidrs` to your IP address(es).

### 3. Initialise and deploy

```bash
terraform init
terraform plan -out=tfplan
terraform apply tfplan
```

### 4. Upload mTLS certificates

After the first apply, the Key Vault is created with placeholder certificate secrets.
Upload your actual certificates using the Azure CLI or Key Vault UI:

```bash
KV_NAME=$(terraform output -raw key_vault_name)

# CA certificate
az keyvault secret set --vault-name "$KV_NAME" --name ca-cert \
  --file /path/to/ca.crt

# Controller certificate and key
az keyvault secret set --vault-name "$KV_NAME" --name controller-cert \
  --file /path/to/controller.crt
az keyvault secret set --vault-name "$KV_NAME" --name controller-key \
  --file /path/to/controller.key

# Agent certificate and key
az keyvault secret set --vault-name "$KV_NAME" --name agent-cert \
  --file /path/to/agent.crt
az keyvault secret set --vault-name "$KV_NAME" --name agent-key \
  --file /path/to/agent.key
```

### 5. Verify deployment

```bash
terraform output deployment_summary
```

Open the web UI:
```bash
open $(terraform output -raw controller_web_url)
```

---

## Toggle HA Mode

Standard to HA:
```bash
# In terraform.tfvars:
enable_ha = true

terraform plan -out=tfplan
terraform apply tfplan
```

HA to standard (scale down):
```bash
enable_ha = false
terraform plan -out=tfplan
terraform apply tfplan
```

---

## Cost Estimate (westus2, approximate)

### Standard mode
| Resource | SKU | Monthly |
|---|---|---|
| Controller VM | Standard_D2s_v5 (1x) | ~$80 |
| Agent VMSS | Standard_F8s_v2 (2x) | ~$280 |
| PostgreSQL | GP_Standard_D2s_v3 | ~$180 |
| Azure Files | Premium 5 TB | ~$230 |
| Key Vault | Standard | ~$5 |
| **Total** | | **~$775/mo** |

### HA mode (additional costs)
| Resource | Additional cost |
|---|---|
| Second controller VM | ~$80 |
| Standard Load Balancer | ~$20 |
| Zone-redundant PostgreSQL | ~$90 |
| ZRS storage premium | ~$60 |
| **HA premium** | **~$250/mo** |

Costs vary by region, actual usage, and reserved instance pricing.
Use the [Azure Pricing Calculator](https://azure.microsoft.com/en-us/pricing/calculator/) for accurate estimates.

---

## Scale Agents

The agent VMSS can be scaled manually or via autoscale rules:

```bash
# Scale agent count manually
az vmss scale \
  --resource-group "$(terraform output -raw resource_group_name)" \
  --name "$(terraform output -raw agent_vmss_name)" \
  --new-capacity 4
```

Or update `agent_count` in `terraform.tfvars` and re-apply.

---

## Cleanup

```bash
terraform destroy
```

> Note: The Key Vault has soft-delete enabled. If `enable_ha = true`, purge protection
> is enabled and the vault cannot be permanently deleted for the retention period (7 days).
> To force-delete after destroy:
> ```bash
> az keyvault purge --name "<vault-name>" --location westus2
> ```

---

## File Reference

| File | Purpose |
|---|---|
| `main.tf` | Terraform + provider config, resource group, locals |
| `variables.tf` | All input variables with defaults and validation |
| `networking.tf` | VNet, subnets, NSG associations, public IPs |
| `security.tf` | NSG rules for controller, agent, database |
| `database.tf` | PostgreSQL Flexible Server, private DNS, config |
| `storage.tf` | Azure Storage Account, file shares, private endpoint |
| `controller.tf` | Controller VM(s), NICs, managed identity, LB association |
| `agents.tf` | Agent VMSS, cloud-init, Key Vault access |
| `loadbalancer.tf` | Azure Load Balancer (HA mode only) |
| `keyvault.tf` | Key Vault, private endpoint, secrets, access policies |
| `outputs.tf` | Useful post-deploy values |
| `terraform.tfvars.example` | Example variable values |
| `templates/controller-cloud-init.yaml.tpl` | Controller VM bootstrap script |
| `templates/agent-cloud-init.yaml.tpl` | Agent VM bootstrap script |

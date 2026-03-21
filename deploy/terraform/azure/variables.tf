# ─── General ─────────────────────────────────────────────────────────────────

variable "location" {
  description = "Azure region for all resources."
  type        = string
  default     = "westus2"
}

variable "resource_group_name" {
  description = "Name of the resource group to create."
  type        = string
  default     = "distributed-encoder-rg"
}

variable "environment" {
  description = "Deployment environment tag (e.g. prod, staging, dev)."
  type        = string
  default     = "prod"

  validation {
    condition     = contains(["prod", "staging", "dev"], var.environment)
    error_message = "environment must be one of: prod, staging, dev."
  }
}

# ─── High Availability ────────────────────────────────────────────────────────

variable "enable_ha" {
  description = "Enable high-availability mode: 2 controller VMs, Azure Load Balancer, zone-redundant PostgreSQL."
  type        = bool
  default     = false
}

# ─── VM Sizing ────────────────────────────────────────────────────────────────

variable "controller_vm_size" {
  description = "Azure VM SKU for controller node(s)."
  type        = string
  default     = "Standard_D2s_v5"
}

variable "agent_vm_size" {
  description = "Azure VM SKU for encoding agent nodes."
  type        = string
  default     = "Standard_F8s_v2"
}

variable "agent_count" {
  description = "Initial number of agent instances in the scale set."
  type        = number
  default     = 2

  validation {
    condition     = var.agent_count >= 1 && var.agent_count <= 50
    error_message = "agent_count must be between 1 and 50."
  }
}

# ─── PostgreSQL ───────────────────────────────────────────────────────────────

variable "db_sku_name" {
  description = "SKU name for Azure Database for PostgreSQL Flexible Server."
  type        = string
  default     = "GP_Standard_D2s_v3"
}

variable "db_storage_mb" {
  description = "Storage size in MB for the PostgreSQL Flexible Server."
  type        = number
  default     = 65536

  validation {
    condition     = contains([32768, 65536, 131072, 262144, 524288, 1048576, 2097152, 4193280, 4194304, 8388608, 16777216], var.db_storage_mb)
    error_message = "db_storage_mb must be a valid PostgreSQL Flexible Server storage size."
  }
}

variable "db_admin_login" {
  description = "Administrator login name for PostgreSQL."
  type        = string
  default     = "distencoder_admin"
}

variable "db_admin_password" {
  description = "Administrator password for PostgreSQL. Store in a .env file or Azure Key Vault — never hardcode."
  type        = string
  sensitive   = true
}

variable "db_version" {
  description = "PostgreSQL major version."
  type        = string
  default     = "16"

  validation {
    condition     = contains(["14", "15", "16"], var.db_version)
    error_message = "db_version must be 14, 15, or 16."
  }
}

# ─── Networking ───────────────────────────────────────────────────────────────

variable "vnet_address_space" {
  description = "CIDR block for the virtual network."
  type        = string
  default     = "10.10.0.0/16"
}

variable "controller_subnet_prefix" {
  description = "CIDR prefix for the controller subnet."
  type        = string
  default     = "10.10.1.0/24"
}

variable "agent_subnet_prefix" {
  description = "CIDR prefix for the agent subnet."
  type        = string
  default     = "10.10.2.0/24"
}

variable "database_subnet_prefix" {
  description = "CIDR prefix for the database subnet (delegated to PostgreSQL Flexible Server)."
  type        = string
  default     = "10.10.3.0/24"
}

variable "storage_subnet_prefix" {
  description = "CIDR prefix for the storage private endpoint subnet."
  type        = string
  default     = "10.10.4.0/24"
}

variable "allowed_ssh_cidrs" {
  description = "List of CIDRs allowed to SSH into controller and agent VMs. Restrict to your IP(s)."
  type        = list(string)
  default     = []
}

# ─── SSH ──────────────────────────────────────────────────────────────────────

variable "ssh_public_key_path" {
  description = "Path to the SSH public key file used for VM authentication."
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

# ─── Application ──────────────────────────────────────────────────────────────

variable "distencoder_version" {
  description = "Version of distributed-encoder to install (e.g. 1.0.4). Used to fetch the correct .deb package."
  type        = string
  default     = "1.0.4"
}

variable "controller_docker_image" {
  description = "Docker image for the controller (registry/image:tag)."
  type        = string
  default     = "ghcr.io/badskater/distributed-encoder-controller:latest"
}

# ─── Storage ──────────────────────────────────────────────────────────────────

variable "storage_account_tier" {
  description = "Performance tier for Azure Storage Account (Standard or Premium)."
  type        = string
  default     = "Premium"

  validation {
    condition     = contains(["Standard", "Premium"], var.storage_account_tier)
    error_message = "storage_account_tier must be Standard or Premium."
  }
}

variable "file_share_quota_gb" {
  description = "Size quota (GB) for the Azure File Share."
  type        = number
  default     = 5120
}

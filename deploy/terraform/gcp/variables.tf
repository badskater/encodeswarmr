variable "project_id" {
  description = "GCP project ID where resources will be deployed."
  type        = string
}

variable "region" {
  description = "GCP region for resource deployment."
  type        = string
  default     = "us-central1"
}

variable "zone" {
  description = "GCP zone for zonal resources (e.g. single-instance deployments)."
  type        = string
  default     = "us-central1-a"
}

variable "environment" {
  description = "Deployment environment label (e.g. prod, staging, dev)."
  type        = string
  default     = "prod"
}

# ---------------------------------------------------------------------------
# Compute
# ---------------------------------------------------------------------------

variable "controller_machine_type" {
  description = "Machine type for the controller VM(s)."
  type        = string
  default     = "e2-standard-2"
}

variable "agent_machine_type" {
  description = "Machine type for agent VMs. Compute-optimised recommended for encoding."
  type        = string
  default     = "c2-standard-8"
}

variable "agent_count" {
  description = "Number of agent VMs to run in the agent Managed Instance Group."
  type        = number
  default     = 2
}

# ---------------------------------------------------------------------------
# High-availability
# ---------------------------------------------------------------------------

variable "enable_ha" {
  description = <<-EOT
    When true, deploys a highly-available topology:
      - Controller: instance template + 2-instance MIG behind TCP/HTTP load balancers.
      - Cloud SQL: REGIONAL availability type with point-in-time recovery.
    When false, a single controller VM and ZONAL Cloud SQL instance are used.
  EOT
  type        = bool
  default     = false
}

# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

variable "db_tier" {
  description = "Cloud SQL machine tier (e.g. db-custom-2-8192)."
  type        = string
  default     = "db-custom-2-8192"
}

variable "db_name" {
  description = "PostgreSQL database name."
  type        = string
  default     = "encodeswarmr"
}

variable "db_user" {
  description = "PostgreSQL user name."
  type        = string
  default     = "encodeswarmr"
}

variable "db_password" {
  description = "PostgreSQL user password. Mark as sensitive; store in a .env file or secret manager."
  type        = string
  sensitive   = true
}

# ---------------------------------------------------------------------------
# Access
# ---------------------------------------------------------------------------

variable "ssh_user" {
  description = "Linux user account added to VMs for SSH access."
  type        = string
  default     = "encodeswarmr"
}

variable "ssh_public_key" {
  description = "SSH public key material (content of id_rsa.pub) added to the ssh_user account on all VMs."
  type        = string
}

# ---------------------------------------------------------------------------
# Application
# ---------------------------------------------------------------------------

variable "encodeswarmr_version" {
  description = "encodeswarmr release version to install (e.g. 1.0.4). Used to fetch the .deb from GitHub releases."
  type        = string
  default     = "1.0.4"
}

# ---------------------------------------------------------------------------
# Networking
# ---------------------------------------------------------------------------

variable "network_cidr" {
  description = "Base CIDR block. The module carves /24 subnets from this range."
  type        = string
  default     = "10.100.0.0/16"
}

variable "controller_subnet_cidr" {
  description = "CIDR for the controller subnet."
  type        = string
  default     = "10.100.1.0/24"
}

variable "agent_subnet_cidr" {
  description = "CIDR for the agent subnet."
  type        = string
  default     = "10.100.2.0/24"
}

variable "filestore_tier" {
  description = "Filestore tier: BASIC_HDD (cheaper) or BASIC_SSD (faster). STANDARD/PREMIUM/ENTERPRISE tiers are not used."
  type        = string
  default     = "BASIC_HDD"

  validation {
    condition     = contains(["BASIC_HDD", "BASIC_SSD"], var.filestore_tier)
    error_message = "filestore_tier must be BASIC_HDD or BASIC_SSD."
  }
}

variable "filestore_capacity_gb" {
  description = "Filestore capacity in GB. Minimum 1024 GB for BASIC_HDD, 2560 GB for BASIC_SSD."
  type        = number
  default     = 1024
}

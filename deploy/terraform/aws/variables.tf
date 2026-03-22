# ── General ────────────────────────────────────────────────────────────────────

variable "region" {
  description = "AWS region to deploy into."
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Deployment environment label (dev, staging, prod). Used in resource names and tags."
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment must be one of: dev, staging, prod."
  }
}

# ── Networking ─────────────────────────────────────────────────────────────────

variable "vpc_cidr" {
  description = "CIDR block for the VPC."
  type        = string
  default     = "10.10.0.0/16"
}

variable "allowed_ssh_cidrs" {
  description = "List of CIDR blocks permitted to SSH into controller and agent instances."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

# ── Compute ────────────────────────────────────────────────────────────────────

variable "controller_instance_type" {
  description = "EC2 instance type for the controller. t3.medium is sufficient for most deployments."
  type        = string
  default     = "t3.medium"
}

variable "agent_instance_type" {
  description = "EC2 instance type for encoding agents. c5.2xlarge provides good CPU performance for x264/x265."
  type        = string
  default     = "c5.2xlarge"
}

variable "agent_count" {
  description = "Desired number of encoding agent instances in the ASG."
  type        = number
  default     = 2
}

variable "controller_count" {
  description = "Number of controller instances. Set to 1 for standard, 2 for HA. Overridden by enable_ha."
  type        = number
  default     = 1
}

variable "ssh_key_name" {
  description = "Name of an existing EC2 key pair to enable SSH access. Leave empty to disable SSH."
  type        = string
  default     = ""
}

# ── High Availability ──────────────────────────────────────────────────────────

variable "enable_ha" {
  description = "When true, deploys 2 controller instances, RDS Multi-AZ, ALB, and NLB for gRPC. When false, deploys a single controller with no load balancer."
  type        = bool
  default     = false
}

# ── Database ───────────────────────────────────────────────────────────────────

variable "db_instance_class" {
  description = "RDS instance class for PostgreSQL. db.t3.medium is the minimum recommended class."
  type        = string
  default     = "db.t3.medium"
}

variable "db_multi_az" {
  description = "Enable RDS Multi-AZ standby replica. Automatically set to true when enable_ha = true."
  type        = bool
  default     = false
}

variable "db_name" {
  description = "Name of the PostgreSQL database to create."
  type        = string
  default     = "encodeswarmr"
}

variable "db_username" {
  description = "Master username for the RDS PostgreSQL instance."
  type        = string
  default     = "encodeswarmr"
}

variable "db_password" {
  description = "Master password for the RDS PostgreSQL instance. Must be at least 16 characters."
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.db_password) >= 16
    error_message = "db_password must be at least 16 characters."
  }
}

variable "db_allocated_storage" {
  description = "Initial allocated storage for RDS in GiB."
  type        = number
  default     = 20
}

variable "db_max_allocated_storage" {
  description = "Maximum storage autoscaling ceiling for RDS in GiB."
  type        = number
  default     = 100
}

variable "db_backup_retention_days" {
  description = "Number of days to retain RDS automated backups."
  type        = number
  default     = 7
}

# ── Storage ────────────────────────────────────────────────────────────────────

variable "efs_performance_mode" {
  description = "EFS performance mode. generalPurpose is recommended for most use cases."
  type        = string
  default     = "generalPurpose"

  validation {
    condition     = contains(["generalPurpose", "maxIO"], var.efs_performance_mode)
    error_message = "efs_performance_mode must be generalPurpose or maxIO."
  }
}

variable "efs_throughput_mode" {
  description = "EFS throughput mode. bursting is cost-effective; provisioned allows setting a fixed throughput."
  type        = string
  default     = "bursting"

  validation {
    condition     = contains(["bursting", "provisioned", "elastic"], var.efs_throughput_mode)
    error_message = "efs_throughput_mode must be bursting, provisioned, or elastic."
  }
}

# ── Application ────────────────────────────────────────────────────────────────

variable "encodeswarmr_version" {
  description = "Distributed-encoder release version to deploy (e.g. 1.0.4). Used to download the correct GitHub release."
  type        = string
  default     = "1.0.4"
}

variable "controller_http_port" {
  description = "TCP port the controller listens on for HTTP/REST/web-UI traffic."
  type        = number
  default     = 8080
}

variable "controller_grpc_port" {
  description = "TCP port the controller listens on for gRPC + mTLS agent connections."
  type        = number
  default     = 9443
}

variable "controller_session_secret" {
  description = "Secret key used to sign controller web sessions. Must be a long random string."
  type        = string
  sensitive   = true
}

variable "enable_oidc" {
  description = "Enable OIDC authentication on the controller."
  type        = bool
  default     = false
}

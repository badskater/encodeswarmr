terraform {
  required_version = ">= 1.5.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 6.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }

  # Uncomment and configure to use a GCS backend for remote state.
  # backend "gcs" {
  #   bucket = "your-terraform-state-bucket"
  #   prefix = "encodeswarmr/gcp"
  # }
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

# Random suffix to ensure globally unique resource names
resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name_prefix = "encodeswarmr-${var.environment}"
  common_labels = {
    project     = "encodeswarmr"
    environment = var.environment
    managed_by  = "terraform"
  }
}

# ---------------------------------------------------------------------------
# VPC Network
# ---------------------------------------------------------------------------

resource "google_compute_network" "main" {
  name                    = "${local.name_prefix}-vpc"
  auto_create_subnetworks = false
  description             = "VPC for distributed-encoder (${var.environment})"
  project                 = var.project_id
}

# ---------------------------------------------------------------------------
# Subnets
# ---------------------------------------------------------------------------

resource "google_compute_subnetwork" "controller" {
  name                     = "${local.name_prefix}-controller-subnet"
  network                  = google_compute_network.main.id
  region                   = var.region
  ip_cidr_range            = var.controller_subnet_cidr
  private_ip_google_access = true

  log_config {
    aggregation_interval = "INTERVAL_10_MIN"
    flow_sampling        = 0.5
    metadata             = "INCLUDE_ALL_METADATA"
  }
}

resource "google_compute_subnetwork" "agents" {
  name                     = "${local.name_prefix}-agents-subnet"
  network                  = google_compute_network.main.id
  region                   = var.region
  ip_cidr_range            = var.agent_subnet_cidr
  private_ip_google_access = true

  log_config {
    aggregation_interval = "INTERVAL_10_MIN"
    flow_sampling        = 0.5
    metadata             = "INCLUDE_ALL_METADATA"
  }
}

# ---------------------------------------------------------------------------
# Cloud Router + Cloud NAT
# Allows private VMs to reach the internet (package downloads, GitHub, etc.)
# without external IP addresses.
# ---------------------------------------------------------------------------

resource "google_compute_router" "main" {
  name    = "${local.name_prefix}-router"
  network = google_compute_network.main.id
  region  = var.region
}

resource "google_compute_router_nat" "main" {
  name                               = "${local.name_prefix}-nat"
  router                             = google_compute_router.main.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}

# ---------------------------------------------------------------------------
# Private Service Access
# Required for Cloud SQL private IP connectivity.
# ---------------------------------------------------------------------------

resource "google_compute_global_address" "private_ip_range" {
  name          = "${local.name_prefix}-sql-private-ip"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.main.id
}

resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = google_compute_network.main.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_range.name]
}

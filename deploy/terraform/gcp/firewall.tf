# ---------------------------------------------------------------------------
# Health-check source ranges used by Google load balancers and health probers.
# ---------------------------------------------------------------------------

locals {
  google_health_check_ranges = ["130.211.0.0/22", "35.191.0.0/16"]
  iap_ssh_range              = ["35.235.240.0/20"]
}

# ---------------------------------------------------------------------------
# Controller firewall rules
# ---------------------------------------------------------------------------

# Allow HTTP (8080) from anywhere — public-facing web UI / API.
resource "google_compute_firewall" "controller_http" {
  name    = "${local.name_prefix}-allow-controller-http"
  network = google_compute_network.main.id

  description = "Allow inbound HTTP traffic on port 8080 to controller VMs."

  allow {
    protocol = "tcp"
    ports    = ["8080"]
  }

  direction     = "INGRESS"
  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["encodeswarmr-controller"]
}

# Allow gRPC (9443) from the agent subnet only.
resource "google_compute_firewall" "controller_grpc" {
  name    = "${local.name_prefix}-allow-controller-grpc"
  network = google_compute_network.main.id

  description = "Allow inbound gRPC traffic on port 9443 to controller VMs from agents and LB health-checkers."

  allow {
    protocol = "tcp"
    ports    = ["9443"]
  }

  direction = "INGRESS"
  source_ranges = concat(
    [var.agent_subnet_cidr],
    local.google_health_check_ranges
  )
  target_tags = ["encodeswarmr-controller"]
}

# Allow SSH to controllers via IAP tunnel (no direct public SSH).
resource "google_compute_firewall" "controller_ssh_iap" {
  name    = "${local.name_prefix}-allow-controller-ssh-iap"
  network = google_compute_network.main.id

  description = "Allow SSH from Identity-Aware Proxy to controller VMs."

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  direction     = "INGRESS"
  source_ranges = local.iap_ssh_range
  target_tags   = ["encodeswarmr-controller"]
}

# Allow LB health checks to reach controller on both service ports.
resource "google_compute_firewall" "controller_healthcheck" {
  name    = "${local.name_prefix}-allow-controller-healthcheck"
  network = google_compute_network.main.id

  description = "Allow Google health-check probes to controller VMs."

  allow {
    protocol = "tcp"
    ports    = ["8080", "9443"]
  }

  direction     = "INGRESS"
  source_ranges = local.google_health_check_ranges
  target_tags   = ["encodeswarmr-controller"]
}

# ---------------------------------------------------------------------------
# Agent firewall rules
# ---------------------------------------------------------------------------

# Allow SSH to agents via IAP tunnel.
resource "google_compute_firewall" "agent_ssh_iap" {
  name    = "${local.name_prefix}-allow-agent-ssh-iap"
  network = google_compute_network.main.id

  description = "Allow SSH from Identity-Aware Proxy to agent VMs."

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  direction     = "INGRESS"
  source_ranges = local.iap_ssh_range
  target_tags   = ["encodeswarmr-agent"]
}

# Agents connect outbound to the controller on port 9443.
# Cloud NAT handles internet egress; no explicit egress rule required
# (GCP allows all egress by default).  This rule documents intent and
# restricts the source subnet for clarity.
resource "google_compute_firewall" "agent_egress_grpc" {
  name    = "${local.name_prefix}-allow-agent-egress-grpc"
  network = google_compute_network.main.id

  description = "Allow agent VMs to connect outbound to controller gRPC port 9443."

  allow {
    protocol = "tcp"
    ports    = ["9443"]
  }

  direction          = "EGRESS"
  destination_ranges = [var.controller_subnet_cidr]
  target_tags        = ["encodeswarmr-agent"]
}

# ---------------------------------------------------------------------------
# Internal traffic between controller and agent subnets
# (NFS Filestore mounts, internal service calls)
# ---------------------------------------------------------------------------

resource "google_compute_firewall" "internal" {
  name    = "${local.name_prefix}-allow-internal"
  network = google_compute_network.main.id

  description = "Allow all internal traffic between controller and agent subnets (NFS, etc.)."

  allow {
    protocol = "tcp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "icmp"
  }

  direction = "INGRESS"
  source_ranges = [
    var.controller_subnet_cidr,
    var.agent_subnet_cidr,
  ]
}

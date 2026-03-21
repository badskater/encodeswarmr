# ---------------------------------------------------------------------------
# Load Balancers — HA deployment only (enable_ha = true)
#
# Two regional load balancers are provisioned:
#   1. Regional TCP Proxy LB  — gRPC port 9443 (agent → controller)
#   2. Regional HTTP(S) LB    — HTTP port 8080  (web UI / API, /health check)
#
# Both backends point at the controller MIG.
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Health checks for backend services
# ---------------------------------------------------------------------------

resource "google_compute_region_health_check" "http" {
  count   = var.enable_ha ? 1 : 0
  name    = "${local.name_prefix}-lb-hc-http"
  region  = var.region
  project = var.project_id

  check_interval_sec  = 10
  timeout_sec         = 5
  healthy_threshold   = 2
  unhealthy_threshold = 3

  http_health_check {
    port         = 8080
    request_path = "/health"
  }
}

resource "google_compute_region_health_check" "grpc" {
  count   = var.enable_ha ? 1 : 0
  name    = "${local.name_prefix}-lb-hc-grpc"
  region  = var.region
  project = var.project_id

  check_interval_sec  = 10
  timeout_sec         = 5
  healthy_threshold   = 2
  unhealthy_threshold = 3

  tcp_health_check {
    port = 9443
  }
}

# ---------------------------------------------------------------------------
# Backend services
# ---------------------------------------------------------------------------

resource "google_compute_region_backend_service" "http" {
  count                 = var.enable_ha ? 1 : 0
  name                  = "${local.name_prefix}-bs-http"
  region                = var.region
  project               = var.project_id
  protocol              = "HTTP"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  timeout_sec           = 30

  health_checks = [google_compute_region_health_check.http[0].id]

  backend {
    group           = google_compute_region_instance_group_manager.controller[0].instance_group
    balancing_mode  = "UTILIZATION"
    capacity_scaler = 1.0
  }
}

resource "google_compute_region_backend_service" "grpc" {
  count                 = var.enable_ha ? 1 : 0
  name                  = "${local.name_prefix}-bs-grpc"
  region                = var.region
  project               = var.project_id
  protocol              = "TCP"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  timeout_sec           = 3600 # Long-lived gRPC streams.

  health_checks = [google_compute_region_health_check.grpc[0].id]

  backend {
    group           = google_compute_region_instance_group_manager.controller[0].instance_group
    balancing_mode  = "UTILIZATION"
    capacity_scaler = 1.0
  }
}

# ---------------------------------------------------------------------------
# HTTP Load Balancer — port 8080
# ---------------------------------------------------------------------------

resource "google_compute_region_url_map" "http" {
  count           = var.enable_ha ? 1 : 0
  name            = "${local.name_prefix}-url-map-http"
  region          = var.region
  project         = var.project_id
  default_service = google_compute_region_backend_service.http[0].id
}

resource "google_compute_region_target_http_proxy" "http" {
  count   = var.enable_ha ? 1 : 0
  name    = "${local.name_prefix}-http-proxy"
  region  = var.region
  project = var.project_id
  url_map = google_compute_region_url_map.http[0].id
}

resource "google_compute_forwarding_rule" "http" {
  count                 = var.enable_ha ? 1 : 0
  name                  = "${local.name_prefix}-fwd-http"
  region                = var.region
  project               = var.project_id
  load_balancing_scheme = "EXTERNAL_MANAGED"
  network_tier          = "STANDARD"
  target                = google_compute_region_target_http_proxy.http[0].id
  port_range            = "8080"
  network               = google_compute_network.main.id
}

# ---------------------------------------------------------------------------
# TCP Proxy Load Balancer — gRPC port 9443
# ---------------------------------------------------------------------------

resource "google_compute_region_target_tcp_proxy" "grpc" {
  count           = var.enable_ha ? 1 : 0
  name            = "${local.name_prefix}-tcp-proxy-grpc"
  region          = var.region
  project         = var.project_id
  backend_service = google_compute_region_backend_service.grpc[0].id
}

resource "google_compute_forwarding_rule" "grpc" {
  count                 = var.enable_ha ? 1 : 0
  name                  = "${local.name_prefix}-fwd-grpc"
  region                = var.region
  project               = var.project_id
  load_balancing_scheme = "EXTERNAL_MANAGED"
  network_tier          = "STANDARD"
  target                = google_compute_region_target_tcp_proxy.grpc[0].id
  port_range            = "9443"
  network               = google_compute_network.main.id
}

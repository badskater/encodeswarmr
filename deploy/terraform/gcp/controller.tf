# ---------------------------------------------------------------------------
# Service account for controller VMs
# ---------------------------------------------------------------------------

resource "google_service_account" "controller" {
  account_id   = "${local.name_prefix}-controller"
  display_name = "distributed-encoder controller service account (${var.environment})"
  project      = var.project_id
}

# Allow the controller SA to write logs and metrics.
resource "google_project_iam_member" "controller_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.controller.email}"
}

resource "google_project_iam_member" "controller_monitoring" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.controller.email}"
}

# ---------------------------------------------------------------------------
# Startup script — installed on every controller VM at first boot.
# Installs Docker, writes config from Secret Manager, mounts Filestore.
# ---------------------------------------------------------------------------

locals {
  controller_startup_script = <<-EOT
    #!/bin/bash
    set -euo pipefail
    export DEBIAN_FRONTEND=noninteractive

    echo "=== distributed-encoder controller startup ==="

    # Fetch secrets from Secret Manager using the instance metadata token.
    SECRET_PREFIX="${local.name_prefix}"
    PROJECT="${var.project_id}"

    fetch_secret() {
      local name="$1"
      local dest="$2"
      local token
      token=$(curl -s -H "Metadata-Flavor: Google" \
        "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" \
        | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
      curl -sf \
        -H "Authorization: Bearer $token" \
        "https://secretmanager.googleapis.com/v1/projects/$PROJECT/secrets/$name/versions/latest:access" \
        | python3 -c "import sys,json,base64; print(base64.b64decode(json.load(sys.stdin)['payload']['data']).decode())" \
        > "$dest"
    }

    # ---------------------------------------------------------------------------
    # Install Docker
    # ---------------------------------------------------------------------------
    if ! command -v docker &>/dev/null; then
      apt-get update -q
      apt-get install -y ca-certificates curl gnupg lsb-release
      install -m 0755 -d /etc/apt/keyrings
      curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
      chmod a+r /etc/apt/keyrings/docker.gpg
      echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
        https://download.docker.com/linux/debian $(lsb_release -cs) stable" \
        > /etc/apt/sources.list.d/docker.list
      apt-get update -q
      apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
      systemctl enable --now docker
    fi

    # ---------------------------------------------------------------------------
    # Mount Filestore (NFS)
    # ---------------------------------------------------------------------------
    ${local.nfs_mount_script}

    # ---------------------------------------------------------------------------
    # Pull mTLS certificates from Secret Manager
    # ---------------------------------------------------------------------------
    mkdir -p /etc/distributed-encoder/certs
    fetch_secret "$${SECRET_PREFIX}-ca-cert"         /etc/distributed-encoder/certs/ca.crt
    fetch_secret "$${SECRET_PREFIX}-controller-cert" /etc/distributed-encoder/certs/controller.crt
    fetch_secret "$${SECRET_PREFIX}-controller-key"  /etc/distributed-encoder/certs/controller.key
    chmod 600 /etc/distributed-encoder/certs/controller.key

    # Pull DB password
    fetch_secret "$${SECRET_PREFIX}-db-password" /tmp/db_pass
    DB_PASS=$(cat /tmp/db_pass); rm /tmp/db_pass

    # ---------------------------------------------------------------------------
    # Write controller config
    # ---------------------------------------------------------------------------
    mkdir -p /etc/distributed-encoder
    cat > /etc/distributed-encoder/controller.yaml <<YAML
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

database:
  url: "postgres://${var.db_user}:$${DB_PASS}@${google_sql_database_instance.main.private_ip_address}:5432/${var.db_name}?sslmode=require"
  max_conns: 25
  min_conns: 5
  max_conn_lifetime: 1h
  migrations_path: "/usr/share/distributed-encoder/migrations"

grpc:
  host: "0.0.0.0"
  port: 9443
  tls:
    cert: "/etc/distributed-encoder/certs/controller.crt"
    key:  "/etc/distributed-encoder/certs/controller.key"
    ca:   "/etc/distributed-encoder/certs/ca.crt"

tls:
  cert: "/etc/distributed-encoder/certs/controller.crt"
  key:  "/etc/distributed-encoder/certs/controller.key"
  ca:   "/etc/distributed-encoder/certs/ca.crt"

auth:
  session_ttl: 24h
  session_secret: "$(openssl rand -hex 32)"

logging:
  level: info
  format: json
  task_log_retention: 720h
  task_log_cleanup_interval: 6h
  task_log_max_lines_per_job: 500000

agent:
  auto_approve: false
  heartbeat_timeout: 90s
  dispatch_interval: 10s
  stale_threshold: 5m

engine:
  tick_interval: 10s
  stale_threshold: 5m

analysis:
  ffmpeg_bin:    "/usr/local/bin/ffmpeg"
  ffprobe_bin:   "/usr/local/bin/ffprobe"
  dovi_tool_bin: "/usr/local/bin/dovi_tool"
  concurrency:   2
  path_mappings:
    - name:    "NAS media"
      linux:   "/mnt/nas/media"
    - name:    "NAS encodes"
      linux:   "/mnt/nas/encodes"
    - name:    "NAS temp"
      linux:   "/mnt/nas/temp"
YAML

    # ---------------------------------------------------------------------------
    # Run distributed-encoder controller in Docker
    # ---------------------------------------------------------------------------
    docker pull "ghcr.io/badskater/distributed-encoder/controller:${var.distencoder_version}" || \
      docker pull "ghcr.io/badskater/distributed-encoder/controller:latest"

    docker run -d \
      --name distencoder-controller \
      --restart unless-stopped \
      -p 8080:8080 \
      -p 9443:9443 \
      -v /etc/distributed-encoder:/etc/distributed-encoder:ro \
      -v /mnt/nas:/mnt/nas \
      -v /usr/share/distributed-encoder:/usr/share/distributed-encoder \
      ghcr.io/badskater/distributed-encoder/controller:${var.distencoder_version}

    echo "=== controller startup complete ==="
  EOT
}

# ---------------------------------------------------------------------------
# Single controller VM (standard / non-HA deployment)
# ---------------------------------------------------------------------------

resource "google_compute_instance" "controller" {
  count = var.enable_ha ? 0 : 1

  name         = "${local.name_prefix}-controller"
  machine_type = var.controller_machine_type
  zone         = var.zone
  project      = var.project_id

  tags   = ["distencoder-controller"]
  labels = local.common_labels

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
      size  = 50
      type  = "pd-ssd"
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.controller.id
    # No access_config = no external IP; access is via Cloud NAT + IAP.
  }

  metadata = {
    ssh-keys               = "${var.ssh_user}:${var.ssh_public_key}"
    serial-port-enable     = "FALSE"
    block-project-ssh-keys = "TRUE"
    startup-script         = local.controller_startup_script
  }

  service_account {
    email  = google_service_account.controller.email
    scopes = ["cloud-platform"]
  }

  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }

  allow_stopping_for_update = true
}

# ---------------------------------------------------------------------------
# HA: Instance template + Managed Instance Group (2 instances)
# ---------------------------------------------------------------------------

resource "google_compute_instance_template" "controller" {
  count = var.enable_ha ? 1 : 0

  name_prefix  = "${local.name_prefix}-controller-tmpl-"
  machine_type = var.controller_machine_type
  project      = var.project_id
  region       = var.region

  tags   = ["distencoder-controller"]
  labels = local.common_labels

  disk {
    source_image = "debian-cloud/debian-12"
    auto_delete  = true
    boot         = true
    disk_size_gb = 50
    disk_type    = "pd-ssd"
  }

  network_interface {
    subnetwork = google_compute_subnetwork.controller.id
  }

  metadata = {
    ssh-keys               = "${var.ssh_user}:${var.ssh_public_key}"
    serial-port-enable     = "FALSE"
    block-project-ssh-keys = "TRUE"
    startup-script         = local.controller_startup_script
  }

  service_account {
    email  = google_service_account.controller.email
    scopes = ["cloud-platform"]
  }

  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "google_compute_region_instance_group_manager" "controller" {
  count = var.enable_ha ? 1 : 0

  name               = "${local.name_prefix}-controller-mig"
  base_instance_name = "${local.name_prefix}-controller"
  region             = var.region
  project            = var.project_id

  target_size = 2

  version {
    instance_template = google_compute_instance_template.controller[0].id
  }

  named_port {
    name = "http"
    port = 8080
  }

  named_port {
    name = "grpc"
    port = 9443
  }

  auto_healing_policies {
    health_check      = google_compute_health_check.controller_http[0].id
    initial_delay_sec = 300
  }

  update_policy {
    type                         = "PROACTIVE"
    minimal_action               = "REPLACE"
    max_unavailable_fixed        = 1
    max_surge_fixed              = 1
    replacement_method           = "SUBSTITUTE"
  }
}

# Health check used by the controller MIG auto-healing (HA only).
resource "google_compute_health_check" "controller_http" {
  count   = var.enable_ha ? 1 : 0
  name    = "${local.name_prefix}-controller-hc-http"
  project = var.project_id

  check_interval_sec  = 15
  timeout_sec         = 5
  healthy_threshold   = 2
  unhealthy_threshold = 3

  http_health_check {
    port         = 8080
    request_path = "/health"
  }
}

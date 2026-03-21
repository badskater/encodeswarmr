# ---------------------------------------------------------------------------
# Service account for agent VMs
# ---------------------------------------------------------------------------

resource "google_service_account" "agent" {
  account_id   = "${local.name_prefix}-agent"
  display_name = "distributed-encoder agent service account (${var.environment})"
  project      = var.project_id
}

resource "google_project_iam_member" "agent_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.agent.email}"
}

resource "google_project_iam_member" "agent_monitoring" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.agent.email}"
}

# ---------------------------------------------------------------------------
# Determine the controller endpoint agents connect to.
# Standard: private IP of the single controller VM.
# HA      : TCP Load Balancer frontend IP.
# ---------------------------------------------------------------------------

locals {
  controller_grpc_address = var.enable_ha ? (
    "${google_compute_forwarding_rule.grpc[0].ip_address}:9443"
  ) : (
    "${google_compute_instance.controller[0].network_interface[0].network_ip}:9443"
  )
}

# ---------------------------------------------------------------------------
# Startup script — installed on every agent VM at first boot.
# Installs .deb package + encoding tools, mounts Filestore, writes config.
# ---------------------------------------------------------------------------

locals {
  agent_startup_script = <<-EOT
    #!/bin/bash
    set -euo pipefail
    export DEBIAN_FRONTEND=noninteractive

    echo "=== distributed-encoder agent startup ==="

    SECRET_PREFIX="${local.name_prefix}"
    PROJECT="${var.project_id}"
    VERSION="${var.distencoder_version}"

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
    # System dependencies
    # ---------------------------------------------------------------------------
    apt-get update -q
    apt-get install -y \
      curl wget ca-certificates \
      ffmpeg \
      x264 x265 \
      nfs-common \
      python3

    # Install svt-av1 from source (Debian packages may be older).
    if ! command -v SvtAv1EncApp &>/dev/null; then
      apt-get install -y cmake gcc g++ make nasm git
      git clone --depth=1 https://gitlab.com/AOMediaCodec/SVT-AV1.git /tmp/svt-av1
      cd /tmp/svt-av1 && mkdir build && cd build
      cmake .. -G"Unix Makefiles" -DCMAKE_BUILD_TYPE=Release
      make -j"$(nproc)"
      make install
      ldconfig
      rm -rf /tmp/svt-av1
    fi

    # ---------------------------------------------------------------------------
    # Install distributed-encoder agent .deb
    # ---------------------------------------------------------------------------
    DEB_URL="https://github.com/badskater/distributed-encoder/releases/download/v$${VERSION}/distributed-encoder-agent_$${VERSION}_amd64.deb"
    wget -q -O /tmp/agent.deb "$DEB_URL"
    dpkg -i /tmp/agent.deb || apt-get install -f -y
    rm /tmp/agent.deb

    # ---------------------------------------------------------------------------
    # Mount Filestore (NFS)
    # ---------------------------------------------------------------------------
    ${local.nfs_mount_script}

    # ---------------------------------------------------------------------------
    # Pull mTLS certificates from Secret Manager
    # ---------------------------------------------------------------------------
    mkdir -p /etc/distributed-encoder-agent/certs
    fetch_secret "$${SECRET_PREFIX}-ca-cert"    /etc/distributed-encoder-agent/certs/ca.crt
    fetch_secret "$${SECRET_PREFIX}-agent-cert" /etc/distributed-encoder-agent/certs/agent.crt
    fetch_secret "$${SECRET_PREFIX}-agent-key"  /etc/distributed-encoder-agent/certs/agent.key
    chmod 600 /etc/distributed-encoder-agent/certs/agent.key

    # ---------------------------------------------------------------------------
    # Write agent config
    # ---------------------------------------------------------------------------
    mkdir -p /etc/distributed-encoder-agent

    # Resolve instance hostname for agent identification.
    HOSTNAME=$(curl -sf -H "Metadata-Flavor: Google" \
      http://metadata.google.internal/computeMetadata/v1/instance/name || hostname)

    cat > /etc/distributed-encoder-agent/agent.yaml <<YAML
controller:
  address: "${local.controller_grpc_address}"
  tls:
    cert: "/etc/distributed-encoder-agent/certs/agent.crt"
    key:  "/etc/distributed-encoder-agent/certs/agent.key"
    ca:   "/etc/distributed-encoder-agent/certs/ca.crt"
  reconnect:
    initial_delay: 5s
    max_delay: 5m
    multiplier: 2.0

agent:
  hostname: "$${HOSTNAME}"
  work_dir: "/var/lib/distributed-encoder-agent/work"
  log_dir:  "/var/log/distributed-encoder-agent"
  offline_db: "/var/lib/distributed-encoder-agent/offline.db"
  heartbeat_interval: 30s
  poll_interval: 10s
  cleanup_on_success: true
  keep_failed_jobs: 10

tools:
  ffmpeg:   "/usr/bin/ffmpeg"
  ffprobe:  "/usr/bin/ffprobe"
  x265:     "/usr/bin/x265"
  x264:     "/usr/bin/x264"
  svt_av1:  "/usr/local/bin/SvtAv1EncApp"
  avs_pipe: ""
  vspipe:   ""
  dovi_tool: ""

gpu:
  enabled: false
  vendor: ""
  max_vram_mb: 0
  monitor_interval: 5s

allowed_shares:
  - "/mnt/nas/media"
  - "/mnt/nas/encodes"

logging:
  level: info
  format: json
  max_size_mb: 100
  max_backups: 5
  compress: true
  stream_buffer_size: 1000
  stream_flush_interval: 1s
YAML

    # ---------------------------------------------------------------------------
    # Start agent service
    # ---------------------------------------------------------------------------
    systemctl enable --now distributed-encoder-agent

    echo "=== agent startup complete ==="
  EOT
}

# ---------------------------------------------------------------------------
# Instance template for agents (always a MIG, even for single count)
# ---------------------------------------------------------------------------

resource "google_compute_instance_template" "agent" {
  name_prefix  = "${local.name_prefix}-agent-tmpl-"
  machine_type = var.agent_machine_type
  project      = var.project_id
  region       = var.region

  tags   = ["distencoder-agent"]
  labels = local.common_labels

  disk {
    source_image = "debian-cloud/debian-12"
    auto_delete  = true
    boot         = true
    disk_size_gb = 100  # Extra space for working files during encoding.
    disk_type    = "pd-ssd"
  }

  network_interface {
    subnetwork = google_compute_subnetwork.agents.id
  }

  metadata = {
    ssh-keys               = "${var.ssh_user}:${var.ssh_public_key}"
    serial-port-enable     = "FALSE"
    block-project-ssh-keys = "TRUE"
    startup-script         = local.agent_startup_script
  }

  service_account {
    email  = google_service_account.agent.email
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

resource "google_compute_region_instance_group_manager" "agents" {
  name               = "${local.name_prefix}-agents-mig"
  base_instance_name = "${local.name_prefix}-agent"
  region             = var.region
  project            = var.project_id

  target_size = var.agent_count

  version {
    instance_template = google_compute_instance_template.agent.id
  }

  update_policy {
    type                         = "OPPORTUNISTIC"
    minimal_action               = "REPLACE"
    max_unavailable_fixed        = 1
    max_surge_fixed              = 0
    replacement_method           = "SUBSTITUTE"
  }
}

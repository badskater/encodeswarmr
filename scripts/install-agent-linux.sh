#!/usr/bin/env bash
# install-agent-linux.sh — Install the EncodeSwarmr agent on Linux.
#
# Supports Debian/Ubuntu (apt/.deb) and RHEL/Rocky Linux/AlmaLinux (dnf/.rpm).
# Falls back to a raw binary install on other distributions.
#
# Usage (all parameters can be set as environment variables or entered interactively):
#
#   sudo CONTROLLER_ADDRESS=encoder.example.com:9443 \
#        AGENT_HOSTNAME=encode-01 \
#        AGENT_VERSION=1.0.0 \
#        CERT_DIR=/tmp/certs \
#        ./scripts/install-agent-linux.sh
#
# Parameters (env vars or prompted):
#   CONTROLLER_ADDRESS   Controller gRPC host:port (e.g. encoder.example.com:9443)
#   AGENT_HOSTNAME       Name for this agent. Auto-detected from hostname if not set.
#   AGENT_VERSION        Release version without "v" prefix (e.g. 1.0.0).
#                        Required when AGENT_BINARY is not set.
#   CERT_DIR             Directory containing ca.crt, <AGENT_HOSTNAME>.crt,
#                        <AGENT_HOSTNAME>.key. Defaults to /tmp/certs.
#   AGENT_BINARY         Path to a pre-downloaded agent binary. Skips download.
#
# What this script does:
#   1. Detects the Linux distribution family (deb/rpm/binary).
#   2. Creates /var/lib/encodeswarmr-agent and /etc/encodeswarmr/.
#   3. Installs the agent binary via .deb, .rpm, or raw binary copy.
#   4. Copies mTLS certificate files to /etc/encodeswarmr/certs/.
#   5. Writes /etc/encodeswarmr/agent.yaml.
#   6. Verifies encoding tools (ffmpeg, x265, x264).
#   7. Enables and starts the encodeswarmr-agent systemd service.
#
# Idempotent: re-running upgrades the binary and overwrites agent.yaml.

set -euo pipefail

CONFIG_DIR="/etc/encodeswarmr"
CERTS_DIR="${CONFIG_DIR}/certs"
CONFIG_FILE="${CONFIG_DIR}/agent.yaml"
WORK_DIR="/var/lib/encodeswarmr-agent/work"
LOG_DIR="/var/log/encodeswarmr-agent"
SERVICE_NAME="encodeswarmr-agent"

# ── Colour helpers ─────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BOLD='\033[1m'; NC='\033[0m'
info()  { echo -e "${GREEN}==>${NC} $*"; }
warn()  { echo -e "${YELLOW}WARN:${NC} $*"; }
error() { echo -e "${RED}ERROR:${NC} $*" >&2; exit 1; }
step()  { echo -e "\n${BOLD}[$1]${NC} $2"; }

# ── Root check ─────────────────────────────────────────────────────────────────
if [[ $EUID -ne 0 ]]; then
  error "This script must be run as root. Try: sudo $0"
fi

# ── Detect distro family ──────────────────────────────────────────────────────
DISTRO_FAMILY="binary"
if command -v apt-get &>/dev/null; then
  DISTRO_FAMILY="deb"
elif command -v dnf &>/dev/null || command -v yum &>/dev/null; then
  DISTRO_FAMILY="rpm"
fi
info "Detected package manager family: ${DISTRO_FAMILY}"

# ── Prompt helper ─────────────────────────────────────────────────────────────
prompt() {
  local var="$1" msg="$2" default="${3:-}"
  if [[ -n "${!var:-}" ]]; then return; fi
  if [[ -n "$default" ]]; then
    read -rp "${msg} [${default}]: " val
    printf -v "$var" '%s' "${val:-$default}"
  else
    while true; do
      read -rp "${msg}: " val
      [[ -n "$val" ]] && break
      echo "  Value is required."
    done
    printf -v "$var" '%s' "$val"
  fi
}

# ── Collect parameters ────────────────────────────────────────────────────────
AGENT_HOSTNAME="${AGENT_HOSTNAME:-$(hostname -s)}"
prompt CONTROLLER_ADDRESS "Controller gRPC address (e.g. encoder.example.com:9443)"
prompt AGENT_HOSTNAME     "Agent hostname" "${AGENT_HOSTNAME}"
prompt CERT_DIR           "Directory containing ca.crt, <hostname>.crt, <hostname>.key" "/tmp/certs"

if [[ -z "${AGENT_BINARY:-}" && -z "${AGENT_VERSION:-}" ]]; then
  prompt AGENT_VERSION "Agent release version (e.g. 1.0.0 — without v prefix)"
fi

# ── Step 1: Create directory structure ────────────────────────────────────────
step "1/7" "Creating directory structure"
mkdir -p "${CERTS_DIR}" "${WORK_DIR}" "${LOG_DIR}"
info "Directories ready."

# ── Step 2: Install agent binary ──────────────────────────────────────────────
step "2/7" "Installing agent binary"

if [[ -n "${AGENT_BINARY:-}" ]]; then
  # Use pre-downloaded binary
  if [[ ! -f "${AGENT_BINARY}" ]]; then
    error "AGENT_BINARY not found: ${AGENT_BINARY}"
  fi
  install -m 0755 "${AGENT_BINARY}" /usr/bin/encodeswarmr-agent
  info "Installed binary from ${AGENT_BINARY}"

elif [[ "${DISTRO_FAMILY}" == "deb" ]]; then
  DEB_FILE="/tmp/encodeswarmr-agent_${AGENT_VERSION}_linux_amd64.deb"
  if [[ ! -f "${DEB_FILE}" ]]; then
    info "Downloading .deb package..."
    curl -fsSL -o "${DEB_FILE}" \
      "https://github.com/badskater/encodeswarmr/releases/download/v${AGENT_VERSION}/encodeswarmr-agent_${AGENT_VERSION}_linux_amd64.deb"
  fi
  dpkg -i "${DEB_FILE}"
  info ".deb package installed."

elif [[ "${DISTRO_FAMILY}" == "rpm" ]]; then
  RPM_FILE="/tmp/encodeswarmr-agent_${AGENT_VERSION}.x86_64.rpm"
  if [[ ! -f "${RPM_FILE}" ]]; then
    info "Downloading .rpm package..."
    curl -fsSL -o "${RPM_FILE}" \
      "https://github.com/badskater/encodeswarmr/releases/download/v${AGENT_VERSION}/encodeswarmr-agent_${AGENT_VERSION}.x86_64.rpm"
  fi
  dnf install -y "${RPM_FILE}" 2>/dev/null || rpm -Uvh "${RPM_FILE}"
  info ".rpm package installed."

else
  # Raw binary fallback for other distros
  BINARY_FILE="/tmp/encodeswarmr-agent-${AGENT_VERSION}"
  if [[ ! -f "${BINARY_FILE}" ]]; then
    info "Downloading raw binary..."
    curl -fsSL -o "${BINARY_FILE}" \
      "https://github.com/badskater/encodeswarmr/releases/download/v${AGENT_VERSION}/encodeswarmr-agent-linux-amd64"
  fi
  install -m 0755 "${BINARY_FILE}" /usr/bin/encodeswarmr-agent
  info "Binary installed to /usr/bin/encodeswarmr-agent"
fi

# ── Step 3: Copy certificates ─────────────────────────────────────────────────
step "3/7" "Copying mTLS certificates"
CERT_MISSING=0
for fname in "ca.crt" "${AGENT_HOSTNAME}.crt" "${AGENT_HOSTNAME}.key"; do
  src="${CERT_DIR}/${fname}"
  if [[ -f "${src}" ]]; then
    cp "${src}" "${CERTS_DIR}/${fname}"
    chmod 640 "${CERTS_DIR}/${fname}"
    info "Copied ${fname}"
  else
    warn "Missing cert file: ${src}"
    CERT_MISSING=$((CERT_MISSING + 1))
  fi
done
if [[ -f "${CERT_DIR}/ca.crt" ]]; then
  chmod 644 "${CERTS_DIR}/ca.crt"
fi
if [[ ${CERT_MISSING} -gt 0 ]]; then
  warn "${CERT_MISSING} cert file(s) missing. Copy them to ${CERTS_DIR}/ before starting the service."
fi
chown -R encodeswarmr-agent:encodeswarmr-agent "${CERTS_DIR}" 2>/dev/null || true

# ── Step 4: Write agent.yaml ──────────────────────────────────────────────────
step "4/7" "Writing ${CONFIG_FILE}"
cat > "${CONFIG_FILE}" <<EOF
controller:
  address: "${CONTROLLER_ADDRESS}"
  tls:
    cert: "${CERTS_DIR}/${AGENT_HOSTNAME}.crt"
    key:  "${CERTS_DIR}/${AGENT_HOSTNAME}.key"
    ca:   "${CERTS_DIR}/ca.crt"
  reconnect:
    initial_delay: 5s
    max_delay: 5m
    multiplier: 2.0

agent:
  hostname: "${AGENT_HOSTNAME}"
  work_dir:   "${WORK_DIR}"
  log_dir:    "${LOG_DIR}"
  offline_db: "/var/lib/encodeswarmr-agent/offline.db"
  heartbeat_interval: 30s
  poll_interval: 10s
  cleanup_on_success: true
  keep_failed_jobs: 10

tools:
  ffmpeg:   "/usr/bin/ffmpeg"
  ffprobe:  "/usr/bin/ffprobe"
  x265:     "/usr/bin/x265"
  x264:     "/usr/bin/x264"
  svt_av1:  ""
  avs_pipe: ""
  vspipe:   ""

gpu:
  enabled: true
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
EOF
chmod 640 "${CONFIG_FILE}"
info "Config written."

# ── Step 5: Verify encoding tools ────────────────────────────────────────────
step "5/7" "Verifying encoding tools"
TOOLS_MISSING=0
printf '  %-12s  %-40s  %s\n' "Tool" "Path" "Status"
printf '  %s\n' "$(printf '─%.0s' {1..60})"
for name_path in "ffmpeg:/usr/bin/ffmpeg" "ffprobe:/usr/bin/ffprobe" "x265:/usr/bin/x265" "x264:/usr/bin/x264"; do
  name="${name_path%%:*}"
  path="${name_path##*:}"
  if command -v "${name}" &>/dev/null || [[ -x "${path}" ]]; then
    printf '  %-12s  %-40s  %s\n' "${name}" "${path}" "FOUND"
  else
    printf '  %-12s  %-40s  %s\n' "${name}" "${path}" "MISSING"
    TOOLS_MISSING=$((TOOLS_MISSING + 1))
  fi
done
echo ""
if [[ ${TOOLS_MISSING} -gt 0 ]]; then
  warn "${TOOLS_MISSING} tool(s) not found. The agent will start but encoding jobs requiring"
  warn "missing tools will fail. Install them and update tool paths in ${CONFIG_FILE}."
  warn "See DEPLOYMENT.md §1.4 for package names and download links."
else
  info "All tools found."
fi

# ── Step 6: Enable systemd service ───────────────────────────────────────────
step "6/7" "Enabling systemd service"

# Package installs write the unit file; binary-only install needs it written here.
if [[ "${DISTRO_FAMILY}" == "binary" ]]; then
  UNIT_DIR="/usr/lib/systemd/system"
  mkdir -p "${UNIT_DIR}"
  cat > "${UNIT_DIR}/${SERVICE_NAME}.service" <<'UNIT'
[Unit]
Description=EncodeSwarmr Agent
Documentation=https://github.com/badskater/encodeswarmr
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=encodeswarmr-agent
Group=encodeswarmr-agent
EnvironmentFile=-/etc/encodeswarmr/agent-environment
ExecStart=/usr/bin/encodeswarmr-agent run \
  --config /etc/encodeswarmr/agent.yaml
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=encodeswarmr-agent
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=full
ReadWritePaths=/var/lib/encodeswarmr-agent /var/log/encodeswarmr-agent

[Install]
WantedBy=multi-user.target
UNIT
  # Create service user if not present
  if ! id encodeswarmr-agent &>/dev/null 2>&1; then
    groupadd -r encodeswarmr-agent 2>/dev/null || true
    useradd -r -s /sbin/nologin -d /var/lib/encodeswarmr-agent \
      -g encodeswarmr-agent -M encodeswarmr-agent
  fi
fi

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
info "Service enabled."

# ── Step 7: Start service ─────────────────────────────────────────────────────
step "7/7" "Starting service"

if [[ ${CERT_MISSING} -gt 0 ]]; then
  warn "Skipping service start — cert files are missing."
  warn "Copy certs to ${CERTS_DIR}/ then run: systemctl start ${SERVICE_NAME}"
else
  systemctl start "${SERVICE_NAME}"
  sleep 2
  if systemctl is-active --quiet "${SERVICE_NAME}"; then
    info "Service is running."
  else
    warn "Service may not have started. Check: journalctl -u ${SERVICE_NAME} -n 30"
  fi
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}============================================================${NC}"
echo -e "${GREEN}  EncodeSwarmr Agent Installed!${NC}"
echo -e "${BOLD}============================================================${NC}"
echo ""
echo "  Agent hostname  : ${AGENT_HOSTNAME}"
echo "  Controller      : ${CONTROLLER_ADDRESS}"
echo "  Config file     : ${CONFIG_FILE}"
echo "  Service name    : ${SERVICE_NAME}"
echo ""
echo -e "${YELLOW}  Next step: approve this agent.${NC}"
echo "  Option A — web UI: open the web UI → Farm Servers → Approve."
echo "  Option B — CLI on the controller host:"
echo "    docker compose exec controller /app/controller agent approve ${AGENT_HOSTNAME}"
echo ""
echo "  Useful commands:"
echo "    systemctl status  ${SERVICE_NAME}"
echo "    systemctl stop    ${SERVICE_NAME}"
echo "    systemctl start   ${SERVICE_NAME}"
echo "    journalctl -u ${SERVICE_NAME} -f"
echo -e "${BOLD}============================================================${NC}"

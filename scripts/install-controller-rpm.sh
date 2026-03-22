#!/usr/bin/env bash
# install-controller-rpm.sh — Bootstrap the EncodeSwarmr controller
# on RHEL 9 / Rocky Linux 9 / AlmaLinux 9 (or compatible RPM-based distros).
#
# Usage (all parameters can be set as environment variables or entered interactively):
#
#   sudo DOMAIN=encoder.example.com \
#        AGENT_NAMES="ENCODE-01,ENCODE-02" \
#        CONTROLLER_VERSION=v1.0.0 \
#        ./scripts/install-controller-rpm.sh
#
# Parameters (env vars or prompted):
#   CONTROLLER_VERSION   Release tag to pull (e.g. v1.0.0). Use "dev" for local build.
#   DOMAIN               Hostname or IP for the controller TLS cert SAN and access URL.
#   AGENT_NAMES          Comma-separated agent hostnames for per-agent cert generation.
#   POSTGRES_PASSWORD    PostgreSQL password. Auto-generated (openssl) if not set.
#   SESSION_SECRET       HTTP session signing secret (>=32 chars). Auto-generated if not set.
#
# What this script does:
#   1. Verifies RHEL/Rocky/AlmaLinux 9 and root/sudo access.
#   2. Installs Docker CE + Compose V2 if not present.
#   3. Creates /opt/encodeswarmr/ directory structure.
#   4. Calls gen-certs.sh to generate CA + controller + per-agent mTLS certs.
#   5. Writes /opt/encodeswarmr/.env with all secrets (chmod 600).
#   6. Copies deployments/docker-compose.yml to the install directory.
#   7. Runs: docker compose up -d
#   8. Waits for PostgreSQL to become healthy.
#   9. Prints a summary with next steps and agent cert file locations.
#
# Note: Database migrations run automatically when the controller container starts.
#
# Idempotent: re-running skips Docker install if already present and overwrites .env.

set -euo pipefail

INSTALL_DIR="/opt/encodeswarmr"
CERTS_DIR="${INSTALL_DIR}/certs"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

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

# ── OS check ──────────────────────────────────────────────────────────────────
if ! grep -qiE 'rhel|rocky|almalinux|centos|fedora' /etc/os-release 2>/dev/null; then
  warn "This script targets RHEL/Rocky Linux/AlmaLinux 9. Proceeding on unsupported OS."
fi

# ── Prompt helper ─────────────────────────────────────────────────────────────
# Usage: prompt VAR_NAME "Prompt text" [default]
# Sets VAR_NAME to the entered value (or default). No-op if VAR_NAME already set.
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

gen_secret() { openssl rand -hex 32; }

# ── Collect parameters ────────────────────────────────────────────────────────
prompt CONTROLLER_VERSION "Controller version to deploy (e.g. v1.0.0)" "dev"
prompt DOMAIN             "Hostname or IP for the controller (used for TLS SAN)"
prompt AGENT_NAMES        "Comma-separated agent hostnames for cert generation" "agent"

[[ -z "${POSTGRES_PASSWORD:-}" ]] && POSTGRES_PASSWORD="$(gen_secret)" && \
  info "Auto-generated POSTGRES_PASSWORD (saved to .env)"

[[ -z "${SESSION_SECRET:-}" ]] && SESSION_SECRET="$(gen_secret)" && \
  info "Auto-generated SESSION_SECRET (saved to .env)"

# ── Step 1: Install Docker CE + Compose V2 ────────────────────────────────────
step "1/7" "Checking Docker installation"
if command -v docker &>/dev/null; then
  info "Docker already installed: $(docker --version)"
else
  info "Installing Docker CE from official DNF repository..."

  # Install prerequisites
  dnf install -y dnf-plugins-core

  # Add Docker's official RPM repository
  dnf config-manager --add-repo https://download.docker.com/linux/rhel/docker-ce.repo

  # Install Docker CE, CLI, containerd, and Compose plugin
  dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

  systemctl enable --now docker
  info "Docker CE installed successfully."
fi

if ! docker compose version &>/dev/null; then
  error "Docker Compose V2 not found. Ensure docker-compose-plugin is installed."
fi

# ── Step 2: Create directory structure ────────────────────────────────────────
step "2/7" "Creating directory structure at ${INSTALL_DIR}"
mkdir -p "${INSTALL_DIR}"/{certs,data,logs,scripts}
info "Directories created."

# ── Step 3: Copy scripts and compose file ────────────────────────────────────
step "3/7" "Copying files to ${INSTALL_DIR}"

cp "${SCRIPT_DIR}/gen-certs.sh" "${INSTALL_DIR}/scripts/gen-certs.sh"
chmod +x "${INSTALL_DIR}/scripts/gen-certs.sh"
info "Copied gen-certs.sh"

cp "${REPO_ROOT}/deployments/docker-compose.yml" "${INSTALL_DIR}/docker-compose.yml"
info "Copied docker-compose.yml"

# ── Step 4: Generate mTLS certificates ───────────────────────────────────────
step "4/7" "Generating mTLS certificates"
info "CN=${DOMAIN}, agents=${AGENT_NAMES}"

bash "${INSTALL_DIR}/scripts/gen-certs.sh" \
  --out "${CERTS_DIR}" \
  --controller-cn "${DOMAIN}" \
  --controller-ip "${DOMAIN}" \
  --agents "${AGENT_NAMES}"

info "Certificates written to ${CERTS_DIR}/"

# ── Step 5: Write .env file ───────────────────────────────────────────────────
step "5/7" "Writing ${INSTALL_DIR}/.env"
cat > "${INSTALL_DIR}/.env" <<EOF
# Generated by install-controller-rpm.sh — do not commit this file
CONTROLLER_VERSION=${CONTROLLER_VERSION}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}

DE_DB_HOST=postgres
DE_DB_PORT=5432
DE_DB_NAME=encodeswarmr
DE_DB_USER=encodeswarmr
DE_DB_PASS=${POSTGRES_PASSWORD}

DE_HTTP_PORT=8080
DE_GRPC_PORT=9443

DE_GRPC_TLS_CERT=/certs/server.crt
DE_GRPC_TLS_KEY=/certs/server.key
DE_GRPC_TLS_CA=/certs/ca.crt

DE_SESSION_SECRET=${SESSION_SECRET}
DE_AGENT_AUTO_APPROVE=false
EOF
chmod 600 "${INSTALL_DIR}/.env"
info ".env written (permissions 600)"

# ── Step 6: Start services ────────────────────────────────────────────────────
step "6/7" "Starting services with docker compose"
cd "${INSTALL_DIR}"
docker compose up -d
info "Services started."

# ── Step 7: Wait for PostgreSQL ───────────────────────────────────────────────
step "7/7" "Waiting for PostgreSQL to become healthy"
RETRIES=30
until docker compose exec -T postgres pg_isready -U encodeswarmr -q 2>/dev/null; do
  RETRIES=$((RETRIES - 1))
  if [[ $RETRIES -le 0 ]]; then
    error "PostgreSQL did not become healthy. Run: docker compose logs postgres"
  fi
  echo -n "."
  sleep 3
done
echo ""
info "PostgreSQL is healthy."

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}============================================================${NC}"
echo -e "${GREEN}  EncodeSwarmr Controller is running!${NC}"
echo -e "${BOLD}============================================================${NC}"
echo ""
echo "  Web UI  :  http://${DOMAIN}:8080"
echo "  gRPC    :  ${DOMAIN}:9443"
echo ""
echo "  Next steps:"
echo "  1. Open http://${DOMAIN}:8080 in your browser."
echo "     The setup wizard will guide you through creating"
echo "     the first admin account."
echo ""
echo "  2. For each agent, copy these cert files to the agent host:"
for name in $(echo "$AGENT_NAMES" | tr ',' ' '); do
  echo "       ${CERTS_DIR}/ca.crt"
  echo "       ${CERTS_DIR}/${name}.crt"
  echo "       ${CERTS_DIR}/${name}.key"
done
echo ""
echo "  3. Windows agents: run install-agent.ps1 on each encoding host."
echo "     Linux agents:   run install-agent-linux.sh on each encoding host."
echo ""
echo "  To view logs:  docker compose -f ${INSTALL_DIR}/docker-compose.yml logs -f"
echo "  To stop:       docker compose -f ${INSTALL_DIR}/docker-compose.yml down"
echo -e "${BOLD}============================================================${NC}"

#!/usr/bin/env bash
# gen-certs.sh — Generate mTLS certificates for the EncodeSwarmr.
#
# Usage:
#   ./scripts/gen-certs.sh [options]
#
# Options:
#   --out DIR           Output directory (default: ./certs)
#   --controller-cn CN  Controller certificate CN / SAN DNS name
#                       (default: controller.internal)
#   --controller-ip IP  Additional SAN IP for the controller cert (optional)
#   --days-ca N         CA certificate validity in days (default: 3650)
#   --days-leaf N       Server/agent certificate validity in days (default: 365)
#   --agents LIST       Comma-separated list of agent names to generate certs for
#                       (default: agent)
#
# Examples:
#   # Minimal — generates ca, server, and one agent cert
#   ./scripts/gen-certs.sh
#
#   # Production — custom CN, IP SAN, multiple agents
#   ./scripts/gen-certs.sh \
#     --out /etc/encodeswarmr/certs \
#     --controller-cn encoder.example.com \
#     --controller-ip 10.0.0.10 \
#     --agents "ENCODE-01,ENCODE-02,ENCODE-03"
#
# Outputs (in --out DIR):
#   ca.crt / ca.key                  — CA certificate and private key
#   server.crt / server.key          — Controller certificate and key
#   <agent-name>.crt / .key          — One pair per agent name
#
# After running, distribute:
#   - ca.crt + server.crt + server.key  → Controller (deployments/certs/)
#   - ca.crt + <agent>.crt + <agent>.key → each agent (C:\DistEncoder\certs\)
#
# Keep ca.key and all .key files private. Never commit them.

set -euo pipefail

# ── Defaults ────────────────────────────────────────────────────────────────
OUT_DIR="./certs"
CONTROLLER_CN="controller.internal"
CONTROLLER_IP=""
DAYS_CA=3650
DAYS_LEAF=365
AGENTS="agent"

# ── Argument parsing ─────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --out)           OUT_DIR="$2";         shift 2 ;;
    --controller-cn) CONTROLLER_CN="$2";   shift 2 ;;
    --controller-ip) CONTROLLER_IP="$2";   shift 2 ;;
    --days-ca)       DAYS_CA="$2";         shift 2 ;;
    --days-leaf)     DAYS_LEAF="$2";       shift 2 ;;
    --agents)        AGENTS="$2";          shift 2 ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Run with --help or read the script header for usage." >&2
      exit 1
      ;;
  esac
done

# ── Preflight ────────────────────────────────────────────────────────────────
if ! command -v openssl &>/dev/null; then
  echo "ERROR: openssl not found in PATH." >&2
  exit 1
fi

mkdir -p "$OUT_DIR"
echo "Output directory: $OUT_DIR"

# ── CA ───────────────────────────────────────────────────────────────────────
echo ""
echo "==> Generating CA..."
openssl genrsa -out "$OUT_DIR/ca.key" 4096 2>/dev/null
openssl req -new -x509 \
  -days "$DAYS_CA" \
  -key "$OUT_DIR/ca.key" \
  -out "$OUT_DIR/ca.crt" \
  -subj "/CN=DistEncoder CA"
echo "    ca.crt  (valid ${DAYS_CA} days)"
echo "    ca.key  — keep private, not needed on controller or agents"

# ── Controller (server) cert ─────────────────────────────────────────────────
echo ""
echo "==> Generating controller certificate (CN=${CONTROLLER_CN})..."

# Build SAN extension
SAN="DNS:${CONTROLLER_CN},DNS:localhost"
if [[ -n "$CONTROLLER_IP" ]]; then
  SAN="${SAN},IP:${CONTROLLER_IP}"
fi

openssl genrsa -out "$OUT_DIR/server.key" 2048 2>/dev/null
openssl req -new \
  -key "$OUT_DIR/server.key" \
  -out "$OUT_DIR/server.csr" \
  -subj "/CN=${CONTROLLER_CN}"
openssl x509 -req \
  -days "$DAYS_LEAF" \
  -in "$OUT_DIR/server.csr" \
  -CA "$OUT_DIR/ca.crt" \
  -CAkey "$OUT_DIR/ca.key" \
  -CAcreateserial \
  -out "$OUT_DIR/server.crt" \
  -extfile <(printf "subjectAltName=%s" "$SAN") 2>/dev/null
rm -f "$OUT_DIR/server.csr"
echo "    server.crt  SAN: ${SAN}  (valid ${DAYS_LEAF} days)"
echo "    server.key"

# ── Agent certs ───────────────────────────────────────────────────────────────
echo ""
echo "==> Generating agent certificates..."
IFS=',' read -ra AGENT_LIST <<< "$AGENTS"
for AGENT_NAME in "${AGENT_LIST[@]}"; do
  AGENT_NAME="${AGENT_NAME// /}"  # trim spaces
  [[ -z "$AGENT_NAME" ]] && continue

  openssl genrsa -out "$OUT_DIR/${AGENT_NAME}.key" 2048 2>/dev/null
  openssl req -new \
    -key "$OUT_DIR/${AGENT_NAME}.key" \
    -out "$OUT_DIR/${AGENT_NAME}.csr" \
    -subj "/CN=${AGENT_NAME}"
  openssl x509 -req \
    -days "$DAYS_LEAF" \
    -in "$OUT_DIR/${AGENT_NAME}.csr" \
    -CA "$OUT_DIR/ca.crt" \
    -CAkey "$OUT_DIR/ca.key" \
    -CAcreateserial \
    -out "$OUT_DIR/${AGENT_NAME}.crt" 2>/dev/null
  rm -f "$OUT_DIR/${AGENT_NAME}.csr"
  echo "    ${AGENT_NAME}.crt / ${AGENT_NAME}.key  (valid ${DAYS_LEAF} days)"
done

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "==> Done. Files in ${OUT_DIR}/:"
ls -1 "$OUT_DIR/"
echo ""
echo "Controller deployment:"
echo "  Copy ca.crt, server.crt, server.key  ->  deployments/certs/"
echo ""
echo "Each agent (repeat per agent name):"
echo "  Copy ca.crt, <agent>.crt, <agent>.key  ->  C:\\DistEncoder\\certs\\"
echo ""
echo "Certificate expiry check:"
echo "  openssl x509 -in ${OUT_DIR}/server.crt -noout -dates"
echo ""
echo "NOTE: ca.key is only needed to sign new certs. Keep it offline and secure."

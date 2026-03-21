#!/bin/bash
set -e

# Create the system user and group (no login shell, no home directory)
if ! id encodeswarmr &>/dev/null 2>&1; then
    adduser --system --group --no-create-home \
        --home /var/lib/encodeswarmr \
        --shell /usr/sbin/nologin \
        encodeswarmr
fi

# Create runtime directories with correct ownership
install -d -o encodeswarmr -g encodeswarmr -m 750 \
    /var/lib/encodeswarmr \
    /var/log/encodeswarmr

# Create optional environment file if it doesn't exist yet
if [ ! -f /etc/encodeswarmr/environment ]; then
    install -o root -g encodeswarmr -m 640 \
        /dev/null /etc/encodeswarmr/environment
fi

# Fix ownership of the certs directory (created by nFPM as root)
chown encodeswarmr:encodeswarmr /etc/encodeswarmr/certs 2>/dev/null || true

# Reload systemd and enable the service
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload >/dev/null 2>&1 || true
    systemctl enable encodeswarmr-controller >/dev/null 2>&1 || true

    # Only auto-start on a fresh install (not on upgrade).
    # dpkg passes the old version as $2 on upgrades; $2 is empty on first install.
    if [ "$1" = "configure" ] && [ -z "$2" ]; then
        systemctl start encodeswarmr-controller 2>/dev/null || true
    fi
fi

echo ""
echo "================================================================"
echo "  EncodeSwarmr Controller installed"
echo "================================================================"
echo ""
echo "  Before starting the service, complete these steps:"
echo ""
echo "  1. Edit /etc/encodeswarmr/controller.yaml"
echo "     Required settings:"
echo "       database.url          PostgreSQL connection string"
echo "       grpc.tls.cert/key/ca  mTLS certificate paths"
echo "       auth.session_secret   At least 32 random characters"
echo "                             Generate: openssl rand -hex 32"
echo ""
echo "  2. Place TLS certificates in /etc/encodeswarmr/certs/"
echo "     Required files: ca.crt  server.crt  server.key"
echo "     See: https://github.com/badskater/encodeswarmr/blob/main/DEPLOYMENT.md"
echo ""
echo "  3. Run database migrations:"
echo "     migrate -path /usr/share/encodeswarmr/migrations \\"
echo "             -database \"\$DATABASE_URL\" up"
echo "     Install golang-migrate: https://github.com/golang-migrate/migrate"
echo ""
echo "  4. Start the service:"
echo "     systemctl start encodeswarmr-controller"
echo "     systemctl status encodeswarmr-controller"
echo ""
echo "  Web UI:  http://localhost:8080"
echo "  Logs:    journalctl -u encodeswarmr-controller -f"
echo "================================================================"
echo ""

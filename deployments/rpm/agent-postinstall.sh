#!/bin/bash
set -e

# RPM %post — $1 = 1 on fresh install, 2 on upgrade.

# Create group if it doesn't exist
if ! getent group encodeswarmr-agent &>/dev/null; then
    groupadd -r encodeswarmr-agent
fi

# Create system user if it doesn't exist
if ! id encodeswarmr-agent &>/dev/null 2>&1; then
    useradd -r -s /sbin/nologin \
        -d /var/lib/encodeswarmr-agent \
        -g encodeswarmr-agent \
        -M \
        encodeswarmr-agent
fi

# Create runtime directories with correct ownership
install -d -o encodeswarmr-agent -g encodeswarmr-agent -m 750 \
    /var/lib/encodeswarmr-agent \
    /var/lib/encodeswarmr-agent/work \
    /var/log/encodeswarmr-agent

# Create optional environment file if it doesn't exist yet
if [ ! -f /etc/encodeswarmr/agent-environment ]; then
    install -o root -g encodeswarmr-agent -m 640 \
        /dev/null /etc/encodeswarmr/agent-environment
fi

# Fix ownership of the certs directory (created by nFPM as root)
chown encodeswarmr-agent:encodeswarmr-agent \
    /etc/encodeswarmr/certs 2>/dev/null || true

# Reload systemd and enable the service.
# Do NOT auto-start: the agent requires TLS certs and a configured
# controller address before it can connect. The operator must start
# the service manually after completing configuration.
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload >/dev/null 2>&1 || true
    systemctl enable encodeswarmr-agent >/dev/null 2>&1 || true
fi

echo ""
echo "================================================================"
echo "  EncodeSwarmr Agent installed"
echo "================================================================"
echo ""
echo "  Before starting the service, complete these steps:"
echo ""
echo "  1. Edit /etc/encodeswarmr/agent.yaml"
echo "     Required settings:"
echo "       controller.address      Controller hostname:port (gRPC)"
echo "       controller.tls.*        mTLS certificate paths"
echo ""
echo "  2. Place TLS certificates in /etc/encodeswarmr/certs/"
echo "     Required files: ca.crt  agent.crt  agent.key"
echo "     See: https://github.com/badskater/encodeswarmr/blob/main/DEPLOYMENT.md"
echo ""
echo "  3. Start the service:"
echo "     systemctl start encodeswarmr-agent"
echo "     systemctl status encodeswarmr-agent"
echo ""
echo "  Logs:  journalctl -u encodeswarmr-agent -f"
echo "================================================================"
echo ""

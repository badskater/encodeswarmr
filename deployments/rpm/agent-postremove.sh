#!/bin/bash
set -e

# RPM %postun — $1 = 0 on final removal, 1 on upgrade.
# Remove user and data directories only on complete removal.
if [ "$1" -eq 0 ]; then
    if id encodeswarmr-agent &>/dev/null 2>&1; then
        userdel encodeswarmr-agent 2>/dev/null || true
    fi
    if getent group encodeswarmr-agent &>/dev/null; then
        groupdel encodeswarmr-agent 2>/dev/null || true
    fi

    rm -rf \
        /var/lib/encodeswarmr-agent \
        /var/log/encodeswarmr-agent

    # Remove agent-specific config files but leave /etc/encodeswarmr/
    # in case the controller package is also installed on this host.
    rm -f \
        /etc/encodeswarmr/agent.yaml \
        /etc/encodeswarmr/agent-environment

    if [ -d /run/systemd/system ]; then
        systemctl daemon-reload >/dev/null 2>&1 || true
    fi
fi

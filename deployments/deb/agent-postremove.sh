#!/bin/bash
set -e

# Only remove user and data directories when purging (apt purge / dpkg --purge).
# On a normal removal (apt remove) configuration files and the user are kept.
if [ "$1" = "purge" ]; then
    if id encodeswarmr-agent &>/dev/null 2>&1; then
        deluser --remove-home encodeswarmr-agent 2>/dev/null || true
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

#!/bin/bash
set -e

# RPM %preun — $1 = 0 on final removal, 1 on upgrade.
# Only stop and disable the service on complete removal, not on upgrade.
if [ "$1" -eq 0 ] && [ -d /run/systemd/system ]; then
    systemctl stop    encodeswarmr-agent 2>/dev/null || true
    systemctl disable encodeswarmr-agent 2>/dev/null || true
fi

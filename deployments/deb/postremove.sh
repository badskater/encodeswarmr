#!/bin/bash
set -e

# Only remove user and data directories when purging (apt purge / dpkg --purge).
# On a normal removal (apt remove) configuration files and the user are kept.
if [ "$1" = "purge" ]; then
    if id encodeswarmr &>/dev/null 2>&1; then
        deluser --remove-home encodeswarmr 2>/dev/null || true
    fi

    rm -rf \
        /var/lib/encodeswarmr \
        /var/log/encodeswarmr \
        /etc/encodeswarmr

    if [ -d /run/systemd/system ]; then
        systemctl daemon-reload >/dev/null 2>&1 || true
    fi
fi

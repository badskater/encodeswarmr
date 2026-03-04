#!/bin/bash
set -e

# Only remove user and data directories when purging (apt purge / dpkg --purge).
# On a normal removal (apt remove) configuration files and the user are kept.
if [ "$1" = "purge" ]; then
    if id distributed-encoder &>/dev/null 2>&1; then
        deluser --remove-home distributed-encoder 2>/dev/null || true
    fi

    rm -rf \
        /var/lib/distributed-encoder \
        /var/log/distributed-encoder \
        /etc/distributed-encoder

    if [ -d /run/systemd/system ]; then
        systemctl daemon-reload >/dev/null 2>&1 || true
    fi
fi

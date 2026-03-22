#!/bin/bash
set -e

# RPM %postun — $1 = 0 on final removal, 1 on upgrade.
# Remove user, data directories, and config only on complete removal.
if [ "$1" -eq 0 ]; then
    if id encodeswarmr &>/dev/null 2>&1; then
        userdel encodeswarmr 2>/dev/null || true
    fi
    if getent group encodeswarmr &>/dev/null; then
        groupdel encodeswarmr 2>/dev/null || true
    fi

    rm -rf \
        /var/lib/encodeswarmr \
        /var/log/encodeswarmr \
        /etc/encodeswarmr

    if [ -d /run/systemd/system ]; then
        systemctl daemon-reload >/dev/null 2>&1 || true
    fi
fi

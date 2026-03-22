#!/bin/bash
set -e

if [ -d /run/systemd/system ]; then
    systemctl stop    encodeswarmr-controller 2>/dev/null || true
    systemctl disable encodeswarmr-controller 2>/dev/null || true
fi

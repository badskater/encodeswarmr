#!/bin/bash
set -e

if [ -d /run/systemd/system ]; then
    systemctl stop    encodeswarmr-agent 2>/dev/null || true
    systemctl disable encodeswarmr-agent 2>/dev/null || true
fi

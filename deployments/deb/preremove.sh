#!/bin/bash
set -e

if [ -d /run/systemd/system ]; then
    systemctl stop    distributed-encoder-controller 2>/dev/null || true
    systemctl disable distributed-encoder-controller 2>/dev/null || true
fi

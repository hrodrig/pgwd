#!/bin/sh
# prerm: stop and disable pgwd services before package removal.
set -e
if command -v systemctl >/dev/null 2>&1; then
    systemctl stop pgwd.service 2>/dev/null || true
    systemctl stop pgwd.timer 2>/dev/null || true
    systemctl disable pgwd.service 2>/dev/null || true
    systemctl disable pgwd.timer 2>/dev/null || true
fi

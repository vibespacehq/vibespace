#!/bin/bash
echo "[vibespace] claude session started" >> /proc/1/fd/1 2>/dev/null
exec /home/user/.npm-global/bin/claude "$@"

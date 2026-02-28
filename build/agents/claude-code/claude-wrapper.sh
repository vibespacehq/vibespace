#!/bin/bash
echo "[vibespace] claude session started" >&2
exec /home/user/.npm-global/bin/claude "$@"

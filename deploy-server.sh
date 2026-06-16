#!/usr/bin/env bash
# Run on the Linux server (e.g. ssh ahad@your-server)
set -euo pipefail

APP_DIR="${APP_DIR:-$HOME/iptv-proxy}"
SERVICE="${SERVICE:-tv-proxy}"

cd "$APP_DIR"

echo "==> Pull latest code (skip if you copy files manually)"
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  git pull --ff-only
fi

echo "==> Build binary"
go build -o tv-proxy .

echo "==> Restart systemd service"
sudo systemctl daemon-reload
sudo systemctl restart "$SERVICE"
sudo systemctl status "$SERVICE" --no-pager

echo "==> Health check"
curl -fsS http://127.0.0.1:8080/health
echo

echo "Done. Verify manifest URLs use https://proxy.previewcloud.cloud (not 127.0.0.1)."

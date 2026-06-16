#!/usr/bin/env bash
# Run on the Linux server (e.g. ssh ahad@your-server)
set -euo pipefail

APP_DIR="${APP_DIR:-$HOME/iptv-proxy}"
SERVICE="${SERVICE:-tv-proxy}"
ENV_FILE="${ENV_FILE:-$APP_DIR/.env}"

cd "$APP_DIR"

if [[ -f "$ENV_FILE" ]]; then
  echo "==> Loading $ENV_FILE"
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

if [[ -z "${PROXY_TOKEN_SECRET:-}" ]]; then
  echo "WARNING: PROXY_TOKEN_SECRET is not set."
  echo "         Cloudflare UI sends ?t= tokens — proxy will return 400 until this matches Pages."
  echo "         Copy env.example to .env and set the same secret as Cloudflare Pages."
fi

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
health="$(curl -fsS http://127.0.0.1:8080/health)"
echo "$health"
echo

if command -v python3 >/dev/null 2>&1; then
  enabled="$(printf '%s' "$health" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("play_tokens_enabled", False))')"
  if [[ "$enabled" != "True" ]]; then
    echo "ERROR: play_tokens_enabled is false on the proxy."
    echo "Add PROXY_TOKEN_SECRET to systemd (EnvironmentFile=$ENV_FILE) and redeploy."
    exit 1
  fi
fi

echo "Done. Verify manifest URLs use https://proxy.previewcloud.cloud (not 127.0.0.1)."

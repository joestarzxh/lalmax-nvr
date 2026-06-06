#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/env.sh"

if is_running; then
  echo "${APP_NAME} running, pid=$(app_pid)"
else
  echo "${APP_NAME} not running"
  exit 1
fi

if command -v curl >/dev/null 2>&1; then
  listen="$(config_listen)"
  if [[ "${listen}" == :* ]]; then
    url="http://127.0.0.1${listen}/api/health"
  else
    url="http://${listen}/api/health"
  fi
  echo "health: ${url}"
  curl -fsS "${url}" || true
  echo
fi

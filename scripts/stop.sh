#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/env.sh"

if ! is_running; then
  echo "${APP_NAME} is not running"
  rm -f "${PID_FILE}"
  cleanup_hls_temp || true
  exit 0
fi

pid="$(app_pid)"
echo "stopping ${APP_NAME}, pid=${pid}"
kill "${pid}"

for _ in {1..50}; do
  if ! kill -0 "${pid}" 2>/dev/null; then
    rm -f "${PID_FILE}"
    cleanup_hls_temp || true
    echo "${APP_NAME} stopped"
    exit 0
  fi
  sleep 0.1
done

echo "${APP_NAME} did not stop within 5s, sending SIGKILL"
kill -9 "${pid}" 2>/dev/null || true
rm -f "${PID_FILE}"
cleanup_hls_temp || true
echo "${APP_NAME} stopped"

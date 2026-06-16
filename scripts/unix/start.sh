#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/env.sh"

ensure_config

listen_addr="$(config_listen)"

if is_running; then
  echo "${APP_NAME} already running, pid=$(app_pid), listen=${listen_addr}"
  exit 0
fi

if [[ ! -x "${BIN_PATH}" ]]; then
  "${SCRIPT_DIR}/build.sh"
fi

ensure_log_file

echo "starting ${APP_NAME}"
echo "  config: ${CONFIG_FILE}"
echo "  listen: ${listen_addr}"
echo "  pid:    ${PID_FILE}"
echo "  log:    ${LOG_FILE}"

cd "${ROOT_DIR}"
nohup "${BIN_PATH}" -config "${CONFIG_FILE}" >>"${LOG_FILE}" 2>&1 &
pid="$!"
echo "${pid}" >"${PID_FILE}"

sleep 0.5
if ! kill -0 "${pid}" 2>/dev/null; then
  echo "${APP_NAME} failed to start, see ${LOG_FILE}" >&2
  rm -f "${PID_FILE}"
  exit 1
fi

echo "${APP_NAME} started, pid=${pid}"

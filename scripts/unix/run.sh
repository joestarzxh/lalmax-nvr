#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/env.sh"

ensure_config

if [[ ! -x "${BIN_PATH}" ]]; then
  "${SCRIPT_DIR}/build.sh"
fi

echo "running ${APP_NAME}"
echo "  config: ${CONFIG_FILE}"
echo "  listen: $(config_listen)"

cd "${ROOT_DIR}"
exec "${BIN_PATH}" -config "${CONFIG_FILE}"

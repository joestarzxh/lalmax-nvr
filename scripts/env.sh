#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

APP_NAME="lalmax-nvr"
BIN_DIR="${BIN_DIR:-${ROOT_DIR}/bin}"
RUN_DIR="${RUN_DIR:-${ROOT_DIR}/run}"
LOG_DIR="${LOG_DIR:-${ROOT_DIR}/logs}"
DATA_DIR="${DATA_DIR:-${ROOT_DIR}/data}"
GOCACHE="${GOCACHE:-/private/tmp/lalmax-nvr-gocache}"
GOMODCACHE="${GOMODCACHE:-/private/tmp/lalmax-nvr-gomodcache}"
CGO_ENABLED="${CGO_ENABLED:-0}"

BIN_PATH="${BIN_PATH:-${BIN_DIR}/${APP_NAME}}"
PID_FILE="${PID_FILE:-${RUN_DIR}/${APP_NAME}.pid}"
LOG_FILE="${LOG_FILE:-${LOG_DIR}/${APP_NAME}.log}"
CONFIG_FILE="${CONFIG_FILE:-${ROOT_DIR}/config/lalmax-nvr.yaml}"
DEFAULT_CONFIG_FILE="${ROOT_DIR}/config/config.example.yaml"

mkdir -p "${BIN_DIR}" "${RUN_DIR}" "${LOG_DIR}" "${DATA_DIR}" "${GOCACHE}" "${GOMODCACHE}" "$(dirname "${CONFIG_FILE}")"

ensure_log_file() {
  mkdir -p "$(dirname "${LOG_FILE}")"
  touch "${LOG_FILE}"
}

sync_storage_root() {
  local escaped_data_dir
  escaped_data_dir="${DATA_DIR//\//\\/}"
  if grep -q '^[[:space:]]*root_dir:[[:space:]]*' "${CONFIG_FILE}" 2>/dev/null; then
    perl -0pi -e "s#(^\\s*root_dir:\\s*).*\$#\${1}\"${escaped_data_dir}\"#m" "${CONFIG_FILE}"
  fi
  # Fix stale lalmax_config_path pointing to old default
  if grep -q 'lalmax_config_path:.*"/var/lib/lalmax-nvr/' "${CONFIG_FILE}" 2>/dev/null; then
    perl -pi -e 's#^\s*lalmax_config_path:\s*"/var/lib/lalmax-nvr/.*$#  # lalmax_config_path: auto-generated at {root_dir}/config/lalmax.conf.json#' "${CONFIG_FILE}"
  fi
}

ensure_config() {
  if [[ -f "${CONFIG_FILE}" ]]; then
    sync_storage_root
    return
  fi
  if [[ ! -f "${DEFAULT_CONFIG_FILE}" ]]; then
    echo "missing default config: ${DEFAULT_CONFIG_FILE}" >&2
    exit 1
  fi
  mkdir -p "$(dirname "${CONFIG_FILE}")"
  cp "${DEFAULT_CONFIG_FILE}" "${CONFIG_FILE}"
  sync_storage_root
  echo "created ${CONFIG_FILE} from config.example.yaml"
}

config_listen() {
  local listen
  listen="$(
    awk '
      /^[[:space:]]*server:[[:space:]]*$/ { in_server=1; next }
      in_server && /^[^[:space:]]/ { in_server=0 }
      in_server && /^[[:space:]]*listen:[[:space:]]*/ {
        sub(/^[[:space:]]*listen:[[:space:]]*/, "", $0)
        gsub(/"/, "", $0)
        gsub(/'\''/, "", $0)
        print
        exit
      }
    ' "${CONFIG_FILE}" 2>/dev/null
  )"
  if [[ -n "${listen}" ]]; then
    printf '%s\n' "${listen}"
    return
  fi
  printf ':9090\n'
}

config_listen_port() {
  local listen
  listen="$(config_listen)"
  if [[ "${listen}" == *:* ]]; then
    printf '%s\n' "${listen##*:}"
    return
  fi
  printf '%s\n' "${listen}"
}

lookup_running_pid() {
  local pid
  if [[ -f "${PID_FILE}" ]]; then
    pid="$(cat "${PID_FILE}")"
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
      printf '%s\n' "${pid}"
      return 0
    fi
  fi

  local port
  port="$(config_listen_port)"
  if command -v lsof >/dev/null 2>&1; then
    pid="$(lsof -nP -iTCP:"${port}" 2>/dev/null | awk 'NR > 1 && $0 ~ /\(LISTEN\)$/ {print $2; exit}')"
    if [[ -n "${pid}" ]]; then
      printf '%s\n' "${pid}"
      return 0
    fi
  fi

  return 1
}

is_running() {
  local pid
  pid="$(lookup_running_pid)" || return 1
  if [[ ! -f "${PID_FILE}" ]] || [[ "$(cat "${PID_FILE}" 2>/dev/null)" != "${pid}" ]]; then
    printf '%s\n' "${pid}" >"${PID_FILE}"
  fi
  return 0
}

app_pid() {
  lookup_running_pid || true
}

config_storage_root() {
  local root
  root="$(
    awk '
      /^[[:space:]]*storage:[[:space:]]*$/ { in_storage=1; next }
      in_storage && /^[^[:space:]]/ { in_storage=0 }
      in_storage && /^[[:space:]]*root_dir:[[:space:]]*/ {
        sub(/^[[:space:]]*root_dir:[[:space:]]*/, "", $0)
        gsub(/"/, "", $0)
        gsub(/'\''/, "", $0)
        print
        exit
      }
    ' "${CONFIG_FILE}" 2>/dev/null
  )"
  if [[ -n "${root}" ]]; then
    printf '%s\n' "${root}"
    return
  fi
  printf '%s\n' "${DATA_DIR}"
}

config_hls_temp_dir() {
  local temp
  temp="$(
    awk '
      /^[[:space:]]*hls:[[:space:]]*$/ { in_hls=1; next }
      in_hls && /^[^[:space:]]/ { in_hls=0 }
      in_hls && /^[[:space:]]*lal_temp_dir:[[:space:]]*/ {
        sub(/^[[:space:]]*lal_temp_dir:[[:space:]]*/, "", $0)
        gsub(/"/, "", $0)
        gsub(/'\''/, "", $0)
        print
        exit
      }
    ' "${CONFIG_FILE}" 2>/dev/null
  )"
  if [[ -n "${temp}" ]]; then
    printf '%s\n' "${temp}"
    return
  fi
  printf 'hls-temp\n'
}

resolve_hls_temp_path() {
  local temp_dir root_dir
  temp_dir="$(config_hls_temp_dir)"
  if [[ "${temp_dir}" == /* ]]; then
    printf '%s\n' "${temp_dir}"
    return
  fi
  root_dir="$(config_storage_root)"
  printf '%s\n' "${root_dir}/${temp_dir}"
}

cleanup_hls_temp() {
  local hls_temp
  hls_temp="$(resolve_hls_temp_path)"
  if [[ ! -d "${hls_temp}" ]]; then
    return 0
  fi
  echo "cleaning HLS temp dir: ${hls_temp}"
  rm -rf "${hls_temp}"
}

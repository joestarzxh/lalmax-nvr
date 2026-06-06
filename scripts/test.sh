#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/env.sh"

cd "${ROOT_DIR}"
CGO_ENABLED="${CGO_ENABLED}" GOCACHE="${GOCACHE}" GOMODCACHE="${GOMODCACHE}" go test ./...

#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN="${REPO_ROOT}/bin/openai-backup"

if [[ ! -x "${BIN}" ]]; then
  echo "未找到可执行文件 ${BIN}，请先运行 scripts/build-backend.sh"
  exit 1
fi

exec "${BIN}" "$@"

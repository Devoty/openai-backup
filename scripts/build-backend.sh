#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

mkdir -p bin
go build -o bin/openai-backup ./...

echo "Go 后端已编译到 ${REPO_ROOT}/bin/openai-backup"

#!/usr/bin/env bash

set -Eeuo pipefail

repo_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$repo_dir"

backend_addr="${CONTADINHO_DEV_ADDR:-localhost:8000}"
frontend_host="${VITE_DEV_HOST:-127.0.0.1}"
frontend_port="${VITE_DEV_PORT:-5173}"
backend_url="http://${backend_addr}"

if ! command -v go >/dev/null 2>&1; then
  echo "Erro: Go não está instalado ou não está no PATH." >&2
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "Erro: npm não está instalado ou não está no PATH." >&2
  exit 1
fi

if [[ ! -d "$repo_dir/frontend/node_modules" ]]; then
  echo "Dependências do frontend não encontradas. Execute: (cd frontend && npm install)" >&2
  exit 1
fi

has_setsid=false
if command -v setsid >/dev/null 2>&1; then
  has_setsid=true
fi

backend_pid=""
frontend_pid=""
last_pid=""

start_process() {
  if [[ "$has_setsid" == true ]]; then
    setsid --wait "$@" &
  else
    "$@" &
  fi
  last_pid=$!
}

stop_process() {
  local pid="$1"

  if [[ "$has_setsid" == true ]]; then
    kill -TERM -- "-$pid" 2>/dev/null || true
  else
    kill -TERM "$pid" 2>/dev/null || true
  fi
}

cleanup() {
  local exit_code=$?
  trap - EXIT INT TERM

  [[ -z "$frontend_pid" ]] || stop_process "$frontend_pid"
  [[ -z "$backend_pid" ]] || stop_process "$backend_pid"

  [[ -z "$frontend_pid" ]] || wait "$frontend_pid" 2>/dev/null || true
  [[ -z "$backend_pid" ]] || wait "$backend_pid" 2>/dev/null || true

  exit "$exit_code"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

echo "Backend:  http://${backend_addr}"
echo "Frontend: http://localhost:${frontend_port}"
echo "Pressione Ctrl-C para encerrar os dois processos."

start_process go run ./cmd/contadinho -addr "$backend_addr"
backend_pid="$last_pid"

start_process env CONTADINHO_DEV_API_URL="$backend_url" npm --prefix "$repo_dir/frontend" run dev -- \
  --host "$frontend_host" \
  --port "$frontend_port"
frontend_pid="$last_pid"

status=0
wait -n "$backend_pid" "$frontend_pid" || status=$?
exit "$status"

#!/usr/bin/env bash
# cleanup-test-daemons.sh — best-effort cleanup for stale test daemons/processes.
# Safe to run repeatedly.
set -euo pipefail

QUIET=0
if [[ "${1:-}" == "--quiet" ]]; then
  QUIET=1
fi

log() {
  if [[ "$QUIET" -eq 0 ]]; then
    echo "$@"
  fi
}

kill_pattern() {
  local pattern="$1"
  local label="$2"
  local pids

  pids="$(pgrep -f "$pattern" 2>/dev/null || true)"
  if [[ -z "$pids" ]]; then
    return
  fi

  log "Stopping $label..."
  while IFS= read -r pid; do
    [[ -z "$pid" ]] && continue
    kill -TERM "$pid" 2>/dev/null || true
  done <<< "$pids"

  sleep 0.3

  pids="$(pgrep -f "$pattern" 2>/dev/null || true)"
  if [[ -n "$pids" ]]; then
    while IFS= read -r pid; do
      [[ -z "$pid" ]] && continue
      kill -KILL "$pid" 2>/dev/null || true
    done <<< "$pids"
  fi
}

kill_test_ports() {
  local start="$1"
  local end="$2"

  command -v lsof >/dev/null 2>&1 || return 0

  for port in $(seq "$start" "$end"); do
    local pids
    pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
    [[ -z "$pids" ]] && continue
    while IFS= read -r pid; do
      [[ -z "$pid" ]] && continue
      # Only kill kaboom-test-binary processes, not production daemons
      local cmd
      cmd="$(ps -p "$pid" -o comm= 2>/dev/null || true)"
      [[ "$cmd" == *kaboom-test-binary* ]] || continue
      kill -TERM "$pid" 2>/dev/null || true
    done <<< "$pids"
    sleep 0.05
    pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
    [[ -z "$pids" ]] && continue
    while IFS= read -r pid; do
      [[ -z "$pid" ]] && continue
      local cmd
      cmd="$(ps -p "$pid" -o comm= 2>/dev/null || true)"
      [[ "$cmd" == *kaboom-test-binary* ]] || continue
      kill -KILL "$pid" 2>/dev/null || true
    done <<< "$pids"
  done
}

is_test_port() {
  local port="$1"
  (( (port >= 7890 && port <= 7910) || (port >= 17890 && port <= 17999) ))
}

cleanup_pid_files() {
  local state_root="${KABOOM_STATE_DIR:-$HOME/.kaboom}"
  local run_dir="$state_root/run"

  if [[ -d "$run_dir" ]]; then
    for pid_file in "$run_dir"/kaboom-*.pid; do
      [[ -e "$pid_file" ]] || break
      local base port
      base="$(basename "$pid_file")"
      port="${base#kaboom-}"
      port="${port%.pid}"
      if [[ "$port" =~ ^[0-9]+$ ]] && is_test_port "$port"; then
        rm -f "$pid_file"
      fi
    done
  fi

  for port in $(seq 7890 7910); do
    rm -f "$HOME/.kaboom-$port.pid" 2>/dev/null || true
  done
  for port in $(seq 17890 17999); do
    rm -f "$HOME/.kaboom-$port.pid" 2>/dev/null || true
  done
}

if [[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]; then
  taskkill /F /IM kaboom-test-binary.exe >/dev/null 2>&1 || true
else
  kill_pattern "kaboom-test-binary --daemon --port" "kaboom test daemons"
  kill_pattern "kaboom-test-binary --port" "kaboom test clients"
  kill_test_ports 7890 7910
  kill_test_ports 17890 17999
fi

cleanup_pid_files

log "Test daemon cleanup complete."

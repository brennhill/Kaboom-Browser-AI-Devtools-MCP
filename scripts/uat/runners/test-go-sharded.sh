#!/usr/bin/env bash
# test-go-sharded.sh - Run a single Go package's tests split across N shards.
#
# Why: the main Go command package has a very large test surface. Splitting tests across
# multiple go test processes can reduce wall-clock time on multi-core machines.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PKG="${KABOOM_CMD_PKG:-./cmd/browser-agent}"
SHARDS="${GO_TEST_SHARDS:-4}"
COUNT="${GO_TEST_COUNT:-1}"
SHORT_MODE=0

usage() {
  cat <<'EOF'
Usage: scripts/uat/runners/test-go-sharded.sh [options] [-- <extra go test args>]

Options:
  --package <pkg>   Go package to shard (default: $KABOOM_CMD_PKG or ./cmd/browser-agent)
  --shards <n>      Number of shards/processes (default: 4)
  --count <n>       go test -count value (default: 1)
  --short           Enable go test -short
  -h, --help        Show help

Examples:
  scripts/uat/runners/test-go-sharded.sh --package ./cmd/browser-agent --shards 4
  scripts/uat/runners/test-go-sharded.sh --short -- -race
EOF
}

EXTRA_ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --package)
      PKG="$2"
      shift 2
      ;;
    --shards)
      SHARDS="$2"
      shift 2
      ;;
    --count)
      COUNT="$2"
      shift 2
      ;;
    --short)
      SHORT_MODE=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    --)
      shift
      EXTRA_ARGS+=("$@")
      break
      ;;
    *)
      EXTRA_ARGS+=("$1")
      shift
      ;;
  esac
done

if ! [[ "$SHARDS" =~ ^[0-9]+$ ]] || [[ "$SHARDS" -lt 1 ]]; then
  echo "invalid --shards value: $SHARDS" >&2
  exit 2
fi
if ! [[ "$COUNT" =~ ^[0-9]+$ ]] || [[ "$COUNT" -lt 1 ]]; then
  echo "invalid --count value: $COUNT" >&2
  exit 2
fi

# NOTE: do NOT use `mapfile`/`readarray` here. Those are bash 4+ builtins, and
# macOS ships bash 3.2 as /bin/bash — there `mapfile` is "command not found",
# which aborted `make test` after the whole Go suite had already passed. Use a
# portable read loop, the same as scripts/uat/runners/test-js-sharded.sh.
TESTS=()
while IFS= read -r test_name; do
  [ -n "$test_name" ] && TESTS+=("$test_name")
done < <(go test "$PKG" -list '^Test' 2>/dev/null | grep '^Test' || true)
if [[ ${#TESTS[@]} -eq 0 ]]; then
  echo "no top-level tests discovered in $PKG; running package directly"
  CMD=(go test "$PKG" -count "$COUNT")
  if [[ "$SHORT_MODE" -eq 1 ]]; then
    CMD+=(-short)
  fi
  CMD+=("${EXTRA_ARGS[@]}")
  "${CMD[@]}"
  exit 0
fi

if [[ "$SHARDS" -gt "${#TESTS[@]}" ]]; then
  SHARDS="${#TESTS[@]}"
fi

echo "Sharding $PKG: ${#TESTS[@]} tests across $SHARDS shard(s)"

declare -a SHARD_TESTS
for ((i=0; i<SHARDS; i++)); do
  SHARD_TESTS[i]=""
done

for i in "${!TESTS[@]}"; do
  shard=$(( i % SHARDS ))
  if [[ -z "${SHARD_TESTS[shard]}" ]]; then
    SHARD_TESTS[shard]="${TESTS[i]}"
  else
    SHARD_TESTS[shard]+="|${TESTS[i]}"
  fi
done

TMP_DIR="$(mktemp -d /tmp/kaboom-go-shards.XXXXXX)"
cleanup() {
  rm -rf "$TMP_DIR"
  if [[ -f "$SCRIPT_DIR/../../maintenance/cleanup-test-daemons.sh" ]]; then
    bash "$SCRIPT_DIR/../../maintenance/cleanup-test-daemons.sh" --quiet >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

declare -a PIDS
declare -a SHARD_LOGS
declare -a SHARD_CMDS

for ((i=0; i<SHARDS; i++)); do
  regex="^(${SHARD_TESTS[i]})$"
  SHARD_LOGS[i]="$TMP_DIR/shard-$i.log"
  CMD=(go test "$PKG" -count "$COUNT" -run "$regex")
  if [[ "$SHORT_MODE" -eq 1 ]]; then
    CMD+=(-short)
  fi
  CMD+=("${EXTRA_ARGS[@]}")
  SHARD_CMDS[i]="${CMD[*]}"

  (
    set -e
    "${CMD[@]}"
  ) >"${SHARD_LOGS[i]}" 2>&1 &
  PIDS[i]=$!
done

FAILURES=0
for ((i=0; i<SHARDS; i++)); do
  if ! wait "${PIDS[i]}"; then
    FAILURES=$((FAILURES + 1))
    echo "Shard $i failed: ${SHARD_CMDS[i]}" >&2
    cat "${SHARD_LOGS[i]}" >&2
  fi
done

if [[ "$FAILURES" -ne 0 ]]; then
  echo "Sharded test run failed: $FAILURES shard(s) failed" >&2
  exit 1
fi

echo "Sharded test run passed"

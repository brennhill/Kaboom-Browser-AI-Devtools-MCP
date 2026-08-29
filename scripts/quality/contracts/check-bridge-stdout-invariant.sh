#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT"

echo "Checking bridge/wrapper stdout invariant..."

PATTERN='fmt\.Print(f|ln)?\(|fmt\.Fprintf\(os\.Stdout|fmt\.Fprint\(os\.Stdout|io\.WriteString\(os\.Stdout|os\.Stdout\.(Write|WriteString|Sync)\('

TARGET_FILES=(
  "cmd/browser-agent/internal/bridge/bridge.go"
  "cmd/browser-agent/internal/bridge/bridge_fastpath.go"
  "cmd/browser-agent/internal/bridge/stdioisolate/isolation.go"
  "cmd/browser-agent/internal/bridge/stdioisolate/isolation_unix.go"
  "cmd/browser-agent/internal/bridge/stdioisolate/isolation_windows.go"
  "cmd/browser-agent/internal/bridge/bridge_transport.go"
  "cmd/browser-agent/config.go"
  "cmd/browser-agent/internal/connectmode/runner.go"
  "cmd/browser-agent/main_connection_mcp.go"
  "cmd/browser-agent/internal/pushapi/runtime.go"
)

VIOLATIONS=0
for file in "${TARGET_FILES[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "Missing required file: $file"
    VIOLATIONS=1
    continue
  fi
  if grep -En -- "$PATTERN" "$file" >/tmp/kaboom-stdout-invariant.tmp 2>/dev/null; then
    echo "INVARIANT VIOLATION: direct stdout write in $file"
    cat /tmp/kaboom-stdout-invariant.tmp
    VIOLATIONS=1
  fi
done
rm -f /tmp/kaboom-stdout-invariant.tmp

if ! grep -En -- 'server\.runtime\.BridgeRunner\(\)\.EnsureIOIsolation\(config\.LogFile\)' cmd/browser-agent/config.go >/dev/null 2>&1; then
  echo "INVARIANT VIOLATION: bridge mode must initialize IO isolation through its constructed runner in config.go"
  VIOLATIONS=1
fi

if ! grep -En -- 'bridge\.SendStartupError\("Bridge stdio isolation failed:' cmd/browser-agent/config.go >/dev/null 2>&1; then
  echo "INVARIANT VIOLATION: bridge isolation failures must be surfaced as JSON-RPC startup errors"
  VIOLATIONS=1
fi

if ! grep -En -- 'syscall\.CloseOnExec\(fd\)' cmd/browser-agent/internal/bridge/stdioisolate/isolation_unix.go >/dev/null 2>&1; then
  echo "INVARIANT VIOLATION: duplicated MCP transport fd must be marked close-on-exec"
  VIOLATIONS=1
fi

if [[ $VIOLATIONS -ne 0 ]]; then
  echo "❌ Bridge/wrapper stdout invariant failed"
  exit 1
fi

echo "✅ Bridge/wrapper stdout invariant OK"

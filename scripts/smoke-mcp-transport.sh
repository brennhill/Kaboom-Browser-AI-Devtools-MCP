#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "Running MCP transport smoke gate..."

GOCACHE="${GOCACHE:-/tmp/go-build-cache}" \
GOMODCACHE="${GOMODCACHE:-/tmp/go-modcache}" \
go test -tags=integration ./cmd/browser-agent \
  -run 'TestStdioIsolation_StartupNoiseDoesNotPolluteMCPTransport|TestStdioIsolation_ContentLengthFramingNotPollutedByStartupNoise|TestStdioIsolation_BridgeExitsAfterStdinEOF' \
  -count=1 -v

echo "✅ MCP transport smoke gate passed"

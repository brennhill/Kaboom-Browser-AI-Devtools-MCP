#!/usr/bin/env bash
# run-go-integration.sh — Run real-binary Go lifecycle and transport tests.

set -euo pipefail

cd "$(dirname "$0")/../.."

readonly INTEGRATION_TEST_PATTERN='^(TestFastStart_|TestBridgeStartupContention_|TestCLI|TestIntegration_|TestMCPProtocol_|TestServerPersistence_|TestReliability_|TestStdio)'

readonly BUILD_DIR="$(mktemp -d "${TMPDIR:-/tmp}/kaboom-integration.XXXXXX")"
trap 'rm -rf "$BUILD_DIR"' EXIT
readonly INTEGRATION_BINARY="$BUILD_DIR/kaboom-test-binary"

build_args=(build -cover -o "$INTEGRATION_BINARY")
if [[ -n "${KABOOM_GO_COVERDIR:-}" ]]; then
  build_args=(build -cover -coverpkg=./... -o "$INTEGRATION_BINARY")
fi
go "${build_args[@]}" ./cmd/browser-agent
export KABOOM_INTEGRATION_BINARY="$INTEGRATION_BINARY"
export KABOOM_INTEGRATION_INSTRUMENTED=1

go test -p 1 -tags=integration -timeout=15m "$@" ./cmd/browser-agent/... \
  -run "$INTEGRATION_TEST_PATTERN"

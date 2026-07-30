#!/usr/bin/env bash
# run-go-integration.sh — Run real-binary Go lifecycle and transport tests.

set -euo pipefail

cd "$(dirname "$0")/../.."

readonly INTEGRATION_TEST_PATTERN='^(TestFastStart_|TestBridgeStartupContention_|TestCLI|TestIntegration_|TestMCPProtocol_|TestServerPersistence_|TestReliability_|TestStdio)'

go test -tags=integration -timeout=15m "$@" ./cmd/browser-agent \
  -run "$INTEGRATION_TEST_PATTERN"

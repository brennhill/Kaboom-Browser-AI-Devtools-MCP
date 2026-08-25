#!/usr/bin/env bash
# install-go-tools.sh — Installs the canonical pinned Go security scanners.

set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=go-tool-versions.env
source "$SCRIPT_DIR/go-tool-versions.env"

GO_TOOL_BIN=${GOBIN:-"$(go env GOPATH)/bin"}
mkdir -p "$GO_TOOL_BIN"

GOBIN="$GO_TOOL_BIN" go install "github.com/securego/gosec/v2/cmd/gosec@$GOSEC_VERSION"
GOBIN="$GO_TOOL_BIN" go install "golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSION"
GOBIN="$GO_TOOL_BIN" go install "github.com/zricethezav/gitleaks/v8@$GITLEAKS_VERSION"

printf 'Installed pinned Go security tools in %s\n' "$GO_TOOL_BIN"

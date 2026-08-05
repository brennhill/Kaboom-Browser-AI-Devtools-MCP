#!/usr/bin/env bash
# install-go-tools.sh — Installs the canonical pinned Go security scanners.

set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=go-tool-versions.env
source "$SCRIPT_DIR/go-tool-versions.env"

go install "github.com/securego/gosec/v2/cmd/gosec@$GOSEC_VERSION"
go install "golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSION"

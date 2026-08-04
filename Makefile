# Kaboom Build Makefile

# Recursive on purpose: a combined `make bump-version ... build` invocation must
# read VERSION after the bump target, not freeze the old value while parsing.
VERSION = $(shell cat VERSION)
BINARY_NAME := kaboom-agentic-browser
HOOKS_BINARY_NAME := kaboom-hooks
BUILD_DIR := dist
LDFLAGS := -s -w -X main.version=$(VERSION) -X github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/export.version=$(VERSION) -X github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry.Version=$(VERSION)
HOOKS_LDFLAGS := -s -w -X main.version=$(VERSION)
CMD_PKG ?= ./cmd/browser-agent
CMD_DIR ?= $(patsubst ./%,%,$(CMD_PKG))
HOOKS_PKG := ./cmd/hooks

# Build targets
PLATFORMS := \
	darwin-amd64 \
	darwin-arm64 \
	linux-amd64 \
	linux-arm64 \
	windows-amd64

.PHONY: all clean build test test-js test-fast test-all test-go-quick test-go-long test-go-sharded test-performance test-race test-cover test-integration test-cover-integration test-cover-all test-bench fuzz-smoke fuzz-nightly mutation-test \
	dev run checksums verify-zero-deps verify-imports verify-size check-file-length \
	lint lint-go lint-js lint-dead lint-dead-go lint-dead-ts format format-fix typecheck check check-wire-drift check-ts-json-casing check-openapi-types check-invariants check-schema ci \
	ci-local ci-go ci-js ci-security ci-e2e ci-bench \
	release-check install-hooks bench-baseline bump-version sync-version validate-versions \
	pypi-binaries pypi-build pypi-publish pypi-test-publish pypi-clean \
	security-check pre-commit verify-all npm-binaries validate-semver \
	verify-llm check-folder-size check-structure check-dormant-tests check-duplicates validate-architecture folder-baseline-update check-test-determinism test-determinism-baseline-update check-go-architecture go-architecture-baseline-update \
	test-upgrade-guards release-gate clean-test-daemons uat \
	generate-wire-types generate-dom-primitives \
	site-dev site-build site-preview \
	$(PLATFORMS)

GO_TEST_SHARDS ?= 4
GO_TEST_COUNT ?= 1
GO_TEST_PARALLEL ?= 16
GO_TEST_P ?= 8
GO_TEST_STATE_DIR ?= /tmp/kaboom-state-test
GO_TEST_TOOLCHAIN ?= auto
GO_TEST_CACHE_DIR ?= /tmp/go-build-cache
SUPPORTED_GO_TOOLCHAIN := go$(shell awk '/^go / {print $$2}' go.mod)

uat:
	./scripts/test-all-tools-comprehensive.sh --suite all

all: validate-semver clean build

clean:
	rm -rf $(BUILD_DIR)

# Generate TypeScript wire types from Go source of truth
generate-wire-types:
	@node scripts/build/generate-wire-types.js

generate-dom-primitives:
	@node scripts/build/generate-dom-primitives.js

generate-draw-mode:
	@node scripts/build/generate-draw-mode.js

# Compile TypeScript to JavaScript (REQUIRED before tests)
compile-ts: validate-versions generate-wire-types generate-dom-primitives generate-draw-mode
	@echo "=== Compiling TypeScript ==="
	@npx tsc
	@if [ ! -f extension/background/init.js ]; then \
		echo "❌ ERROR: TypeScript compilation failed - extension/background/init.js not found"; \
		exit 1; \
	fi
	@echo "=== Bundling extension scripts ==="
	@node scripts/build/bundle-content.js
	@if [ ! -f extension/content.bundled.js ]; then \
		echo "❌ ERROR: Content script bundling failed"; \
		exit 1; \
	fi
	@if [ ! -f extension/inject.bundled.js ]; then \
		echo "❌ ERROR: Inject script bundling failed"; \
		exit 1; \
	fi
	@if [ ! -f extension/early-patch.bundled.js ]; then \
		echo "❌ ERROR: Early-patch script bundling failed"; \
		exit 1; \
	fi
	@echo "✅ TypeScript compilation successful"

# "All tests" (documented contract): file-length gate + Go (-short) + JS suites.
test: check-file-length
	$(MAKE) test-go-quick
	$(MAKE) test-performance
	$(MAKE) test-js

test-long:
	$(MAKE) test-go-long

test-js: compile-ts
	./scripts/test-js-sharded.sh

test-fast:
	go vet $(CMD_PKG)/
	$(MAKE) test-go-quick
	./scripts/test-js-sharded.sh

# `test` now includes the JS suite; keep test-all as a stable alias.
test-all: test

test-go-quick:
	@set -e; trap 'bash ./scripts/cleanup-test-daemons.sh --quiet >/dev/null 2>&1 || true' EXIT; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) KABOOM_STATE_DIR=$(GO_TEST_STATE_DIR) go test -short -count=$(GO_TEST_COUNT) -p $(GO_TEST_P) -parallel $(GO_TEST_PARALLEL) ./internal/...; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) KABOOM_STATE_DIR=$(GO_TEST_STATE_DIR) GO_TEST_SHARDS=$(GO_TEST_SHARDS) GO_TEST_COUNT=$(GO_TEST_COUNT) KABOOM_CMD_PKG=$(CMD_PKG) ./scripts/test-go-sharded.sh --package $(CMD_PKG) --short -- -parallel $(GO_TEST_PARALLEL)

test-go-long:
	@set -e; trap 'bash ./scripts/cleanup-test-daemons.sh --quiet >/dev/null 2>&1 || true' EXIT; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) KABOOM_STATE_DIR=$(GO_TEST_STATE_DIR) go test -count=$(GO_TEST_COUNT) -p $(GO_TEST_P) -parallel $(GO_TEST_PARALLEL) ./internal/...; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) KABOOM_STATE_DIR=$(GO_TEST_STATE_DIR) GO_TEST_SHARDS=$(GO_TEST_SHARDS) GO_TEST_COUNT=$(GO_TEST_COUNT) KABOOM_CMD_PKG=$(CMD_PKG) ./scripts/test-go-sharded.sh --package $(CMD_PKG) -- -parallel $(GO_TEST_PARALLEL)

test-go-sharded:
	@set -e; trap 'bash ./scripts/cleanup-test-daemons.sh --quiet >/dev/null 2>&1 || true' EXIT; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) KABOOM_STATE_DIR=$(GO_TEST_STATE_DIR) GO_TEST_SHARDS=$(GO_TEST_SHARDS) GO_TEST_COUNT=$(GO_TEST_COUNT) KABOOM_CMD_PKG=$(CMD_PKG) ./scripts/test-go-sharded.sh --package $(CMD_PKG) -- -parallel $(GO_TEST_PARALLEL)

# Wall-clock SLOs must run without cross-package or intra-package contention.
# Unit lanes retain deterministic correctness/race coverage under -short.
test-performance:
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) GOMAXPROCS=1 go test -count=1 -p 1 -parallel 1 -run '^(TestSLO|TestRedactPerformance)' ./internal/pagination ./internal/redaction

test-race:
	go test -race -v ./...

test-cover:
	./scripts/build/run-go-coverage.sh

test-integration:
	@set -e; trap 'bash ./scripts/cleanup-test-daemons.sh --quiet >/dev/null 2>&1 || true' EXIT; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) KABOOM_STATE_DIR=$(GO_TEST_STATE_DIR) go test -tags=integration -count=1 -timeout=300s ./internal/... $(CMD_PKG)/...

test-cover-integration:
	@mkdir -p coverage/integration
	GOCOVERDIR=coverage/integration go test -tags=integration -cover -timeout 120s $(CMD_PKG)/ -count=1
	@go tool covdata percent -i=coverage/integration

test-cover-all:
	@mkdir -p coverage/unit coverage/integration coverage/merged
	GOCOVERDIR=coverage/unit go test -cover ./internal/...
	GOCOVERDIR=coverage/integration go test -tags=integration -cover -timeout 120s $(CMD_PKG)/ -count=1
	go tool covdata merge -i=coverage/unit,coverage/integration -o=coverage/merged
	go tool covdata textfmt -i=coverage/merged -o=coverage/coverage.txt
	@go tool cover -func=coverage/coverage.txt | grep total
	@echo "HTML report: go tool cover -html=coverage/coverage.txt"

test-bench:
	go test -bench=. -benchmem -count=3 $(CMD_PKG)/...

fuzz-smoke:
	@scripts/ci/run-fuzz-campaigns.sh --smoke

fuzz-nightly:
	@scripts/ci/run-fuzz-campaigns.sh --nightly

mutation-test:
	@node scripts/ci/run-targeted-mutations.mjs

clean-test-daemons:
	bash ./scripts/cleanup-test-daemons.sh

verify-zero-deps:
	@if grep -q '^require' go.mod; then echo "FAIL: go.mod contains external dependencies"; exit 1; fi
	@if [ -f go.sum ]; then echo "FAIL: go.sum exists (implies external dependencies)"; exit 1; fi
	@echo "OK: Zero external dependencies verified"

verify-imports:
	@VIOLATIONS=$$(go list -f '{{range .Imports}}{{.}} {{end}}' $(CMD_PKG)/ | tr ' ' '\n' | grep -v '^$$' | grep -v '^[a-z]' | grep -v '^github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP'); \
	if [ -n "$$VIOLATIONS" ]; then echo "FAIL: Non-stdlib imports found:"; echo "$$VIOLATIONS"; exit 1; fi
	@echo "OK: All imports are stdlib or internal"

verify-size:
	@make dev 2>/dev/null
	@SIZE=$$(wc -c < dist/$(BINARY_NAME) | tr -d ' '); \
	MAX=15000000; \
	if [ $$SIZE -gt $$MAX ]; then echo "FAIL: Binary size $${SIZE} bytes exceeds $${MAX} byte limit"; exit 1; \
	else echo "OK: Binary size $${SIZE} bytes (limit: $${MAX})"; fi

# Check file line limits (800 lines, hand-written source only)
check-file-length:
	@bash scripts/check-file-length.sh

# Ratcheting per-folder source-file limit (10 files; existing folders frozen at
# their current count and may only shrink).
check-folder-size:
	@node scripts/check-folder-size.cjs

# Re-freeze the folder baseline after reducing a folder's file count.
folder-baseline-update:
	@node scripts/check-folder-size.cjs --update

# Go tests excluded from the default suite (stale build tags, *_test.go.* files).
check-dormant-tests:
	@bash scripts/check-dormant-tests.sh

# Ratchet wall-clock sleeps out of Go tests. Existing per-file debt may only decrease.
check-test-determinism:
	@go test ./scripts/contracts
	@go run ./scripts/contracts

test-determinism-baseline-update:
	@go run ./scripts/contracts --update

# Prevent mutable package state and public Go APIs from growing unnoticed.
check-go-architecture:
	@go test ./scripts/contracts/goarchitecture
	@go run ./scripts/contracts/goarchitecture

# Only lowers existing allowances; intentional growth requires reviewed baseline edits.
go-architecture-baseline-update:
	@go run ./scripts/contracts/goarchitecture --update

# All structural gates: physical size, dependency direction, public surface,
# cycles, dormant tests, and high-risk extension duplication.
check-structure: check-file-length check-folder-size check-dormant-tests check-test-determinism check-go-architecture lint-boundaries lint-silent-catches lint-circular check-duplicates

validate-architecture:
	@bash scripts/validate-architecture.sh

check-duplicates:
	@npx jscpd src/background src/popup --min-lines 8 --min-tokens 60 --threshold 0

# Validate strict semver (X.Y.Z format, no pre-release)
validate-semver:
	@bash scripts/validate-semver.sh

# Validate optionalDependencies match package version
validate-deps-versions:
	@node npm/kaboom-agentic-browser/lib/validate-versions.js

build: $(PLATFORMS)

# Every platform build validates independently, including direct and parallel builds.
$(PLATFORMS): validate-versions

darwin-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-x64 $(CMD_PKG)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(HOOKS_LDFLAGS)" -o $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-darwin-x64 $(HOOKS_PKG)

darwin-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PKG)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(HOOKS_LDFLAGS)" -o $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-darwin-arm64 $(HOOKS_PKG)

linux-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-x64 $(CMD_PKG)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(HOOKS_LDFLAGS)" -o $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-linux-x64 $(HOOKS_PKG)

linux-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PKG)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(HOOKS_LDFLAGS)" -o $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-linux-arm64 $(HOOKS_PKG)

windows-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-win32-x64.exe $(CMD_PKG)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(HOOKS_LDFLAGS)" -o $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-win32-x64.exe $(HOOKS_PKG)

# Build and copy binaries to NPM package directories (for releases).
# Staged names MUST match each platform package's "files" whitelist and the
# launcher's lookup (bin/kaboom-agentic-browser + bin/kaboom-hooks).
npm-binaries: build compile-ts
	@echo "=== Copying binaries to NPM packages ==="
	@mkdir -p npm/darwin-arm64/bin npm/darwin-x64/bin npm/linux-arm64/bin npm/linux-x64/bin npm/win32-x64/bin
	cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 npm/darwin-arm64/bin/$(BINARY_NAME)
	cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-x64 npm/darwin-x64/bin/$(BINARY_NAME)
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 npm/linux-arm64/bin/$(BINARY_NAME)
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-x64 npm/linux-x64/bin/$(BINARY_NAME)
	cp $(BUILD_DIR)/$(BINARY_NAME)-win32-x64.exe npm/win32-x64/bin/$(BINARY_NAME).exe
	@echo "=== Copying hooks binaries to NPM packages ==="
	cp $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-darwin-arm64 npm/darwin-arm64/bin/$(HOOKS_BINARY_NAME)
	cp $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-darwin-x64 npm/darwin-x64/bin/$(HOOKS_BINARY_NAME)
	cp $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-linux-arm64 npm/linux-arm64/bin/$(HOOKS_BINARY_NAME)
	cp $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-linux-x64 npm/linux-x64/bin/$(HOOKS_BINARY_NAME)
	cp $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-win32-x64.exe npm/win32-x64/bin/$(HOOKS_BINARY_NAME).exe
	@echo "=== Copying extension to main NPM package ==="
	@mkdir -p npm/kaboom-agentic-browser/extension
	@cp -r extension/* npm/kaboom-agentic-browser/extension/
	@echo "=== Verifying embedded versions ==="
	@case "$$(uname -s)-$$(uname -m)" in \
		Darwin-arm64) VERIFY_BIN=npm/darwin-arm64/bin/$(BINARY_NAME) ;; \
		Darwin-*) VERIFY_BIN=npm/darwin-x64/bin/$(BINARY_NAME) ;; \
		Linux-aarch64|Linux-arm64) VERIFY_BIN=npm/linux-arm64/bin/$(BINARY_NAME) ;; \
		*) VERIFY_BIN=npm/linux-x64/bin/$(BINARY_NAME) ;; \
	esac; \
	EMBEDDED=$$($$VERIFY_BIN --version 2>&1 | grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+'); \
	EXPECTED=$$(cat VERSION); \
	if [ "$$EMBEDDED" != "$$EXPECTED" ]; then \
		echo "❌ ERROR: Embedded version $$EMBEDDED does not match VERSION file $$EXPECTED"; \
		exit 1; \
	fi
	@echo "=== Verifying every platform package ships its binaries (publish guard) ==="
	@for d in darwin-arm64 darwin-x64 linux-arm64 linux-x64 win32-x64; do \
		node scripts/release/verify-platform-binaries.js npm/$$d || exit 1; \
	done
	@echo "✅ NPM binaries and extension ready with version $(VERSION)"

# Build for current platform only (for development)
dev:
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PKG)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(HOOKS_BINARY_NAME) $(HOOKS_PKG)

# Run the server locally
run:
	CGO_ENABLED=0 go run $(CMD_PKG)

# Create checksums
checksums:
	cd $(BUILD_DIR) && shasum -a 256 * > checksums.txt

# --- Code Quality ---

lint: lint-go lint-js

lint-go:
	go vet $(CMD_PKG)/
	@GOLANGCI=$$(command -v "$$(go env GOPATH)/bin/golangci-lint" 2>/dev/null || command -v golangci-lint 2>/dev/null || true); \
	if [ -n "$$GOLANGCI" ]; then $$GOLANGCI run $(CMD_PKG)/... ./internal/...; else echo "golangci-lint not installed (optional)"; fi

lint-dead-go:
	@echo "=== Checking for dead Go code (advisory) ==="
	@DEADCODE=$$(command -v "$$(go env GOPATH)/bin/deadcode" 2>/dev/null || command -v deadcode 2>/dev/null || true); \
	if [ -z "$$DEADCODE" ]; then echo "Install: go install golang.org/x/tools/cmd/deadcode@latest"; exit 1; fi; \
	RESULTS=$$($$DEADCODE -test $(CMD_PKG)/... ./internal/... 2>&1 | grep -v _test.go); \
	if [ -n "$$RESULTS" ]; then \
		COUNT=$$(echo "$$RESULTS" | wc -l | tr -d ' '); \
		echo "$$RESULTS"; \
		echo ""; \
		echo "Found $$COUNT unreachable function(s). To clean up:"; \
		echo "  1. Remove the dead function (and its doc comment)"; \
		echo "  2. If the file has live types/vars/consts but no live funcs, move the live symbols to a neighboring file first"; \
		echo "  3. Delete the file only when it has zero remaining symbols"; \
		echo "  4. Run 'go build ./...' after each deletion to verify"; \
	else \
		echo "No dead code found"; \
	fi

lint-dead-ts:
	@echo "=== Checking for dead TypeScript exports ==="
	@npx knip --no-exit-code

lint-dead: lint-dead-go lint-dead-ts

lint-circular:
	@bash scripts/check-circular-deps.sh

lint-boundaries:
	@node scripts/contracts/check-architecture-boundaries.cjs

lint-silent-catches:
	@node --test scripts/contracts/check-silent-catches.test.cjs
	@node scripts/contracts/check-silent-catches.cjs

lint-json-casing:
	@bash scripts/check-json-casing.sh

lint-hardening:
	@./scripts/lint-hardening.sh

lint-js:
	npx eslint extension/ tests/extension/

format:
	@echo "Checking Go formatting..."
	@test -z "$$(gofmt -l $(CMD_DIR)/)" || (gofmt -l $(CMD_DIR)/ && exit 1)
	npx prettier --check .

format-fix:
	gofmt -w $(CMD_DIR)/
	npx prettier --write .

typecheck:
	npx tsc --noEmit

check: check-structure lint lint-json-casing format typecheck check-invariants

check-wire-drift:
	@node scripts/build/generate-wire-types.js --check

check-ts-json-casing:
	@node scripts/contracts/check-ts-json-casing.js

check-openapi-types:
	@tmp_dir=$$(mktemp -d); tmp_file="$$tmp_dir/openapi-types.ts"; trap 'rm -f "$$tmp_file"; rmdir "$$tmp_dir"' EXIT; \
		npx --no-install openapi-typescript cmd/browser-agent/openapi.json -o "$$tmp_file" >/dev/null; \
		npx prettier --config .prettierrc --write "$$tmp_file" >/dev/null; \
		cmp -s "$$tmp_file" src/generated/openapi-types.ts || { \
			echo "OpenAPI TypeScript output is stale; regenerate src/generated/openapi-types.ts." >&2; exit 1; \
		}

check-invariants: check-wire-drift check-ts-json-casing check-openapi-types

check-schema:
	@npm run docs:lint:reference-schema-sync
	@go test ./cmd/browser-agent -run 'TestSchemaParity_' -count=1
	@./scripts/check-esm-extensions.sh
	@./scripts/check-sync-invariants.sh
	@./scripts/check-bridge-stdout-invariant.sh
	@./scripts/validate-codex-skills.sh

smoke-mcp-transport:
	@./scripts/smoke-mcp-transport.sh

# `test` already includes the JS suite (test-js).
ci: check test validate-deps-versions

# --- Local CI (mirrors GitHub Actions) ---

ci-local: ci-go ci-js ci-security
	@echo "All CI checks passed locally"

ci-e2e:
	cd tests/e2e && npm ci && npx playwright install chromium --with-deps && npx playwright test

extension-zip:
	@mkdir -p $(BUILD_DIR)
	@rm -f $(BUILD_DIR)/kaboom-extension-v$(VERSION).zip
	cd extension && zip -r ../$(BUILD_DIR)/kaboom-extension-v$(VERSION).zip \
		. \
		-x "*.DS_Store" "package.json" "*__tests__/*" "*.test.js" "*.test.cjs"
	@echo "Built $(BUILD_DIR)/kaboom-extension-v$(VERSION).zip"
	@ls -lh $(BUILD_DIR)/kaboom-extension-v$(VERSION).zip

extension-crx:
	@node scripts/release/build-crx.js

release-check: ci-local ci-e2e smoke-mcp-transport
	@echo "All release checks passed (CI + E2E)"

ci-go:
	go vet $(CMD_PKG)/
	$(MAKE) test-race
	$(MAKE) test-cover
	$(MAKE) build
	$(MAKE) verify-zero-deps

ci-js:
	npm ci
	npx eslint extension/ tests/extension/
	npx tsc --noEmit
	JS_TEST_TIMEOUT=20000 ./scripts/test-js-sharded.sh

ci-security: security-check

ci-bench:
	@command -v benchstat >/dev/null 2>&1 || { echo "benchstat not found. Install: go install golang.org/x/perf/cmd/benchstat@latest"; exit 1; }
	@test -f docs/benchmarks/baseline.txt || { echo "FAIL: No baseline. Run 'make bench-baseline' first."; exit 1; }
	go test -bench=. -benchmem -count=6 -run=^$$ $(CMD_PKG)/ > /tmp/kaboom-bench-current.txt
	benchstat docs/benchmarks/baseline.txt /tmp/kaboom-bench-current.txt

bench-baseline:
	@mkdir -p benchmarks
	go test -bench=. -benchmem -count=6 -run=^$$ $(CMD_PKG)/ > docs/benchmarks/baseline.txt
	@echo "Baseline saved to docs/benchmarks/baseline.txt"

install-hooks:
	@cp scripts/hooks/pre-commit .git/hooks/pre-commit
	@cp scripts/hooks/pre-push .git/hooks/pre-push
	@chmod +x .git/hooks/pre-commit .git/hooks/pre-push
	@echo "Git hooks installed (pre-commit and pre-push)."

# --- Quality Gates ---

# Run all security checks: source analysis, known Go vulnerabilities, production
# runtime dependency audit, bounded build-tool exceptions, and JS security lint.
security-check:
	@echo "Running security checks..."
	@command -v gosec >/dev/null 2>&1 || { echo "gosec not found. Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"; exit 1; }
	@command -v govulncheck >/dev/null 2>&1 || { echo "govulncheck not found. Install: go install golang.org/x/vuln/cmd/govulncheck@v1.1.4"; exit 1; }
	gosec -quiet -exclude=G104,G114,G204,G301,G304,G306 -severity=high ./cmd/browser-agent/... ./internal/...
	GOTOOLCHAIN=$(SUPPORTED_GO_TOOLCHAIN) govulncheck ./cmd/browser-agent/... ./internal/...
	node --test scripts/security/check-npm-audit.test.mjs
	node scripts/security/check-npm-audit.mjs
	npx eslint extension/ tests/extension/
	@echo "All security checks passed"

# Pre-commit quality gate (lint + security, no tests)
pre-commit: lint security-check
	@echo "Pre-commit checks passed"

# Full verification (lint + security + tests with coverage)
verify-all: lint security-check test-cover test-js
	@echo "All verification checks passed"

# Fast, high-signal verification loop for LLM-driven maintenance.
# Typical runtime target: ~60-120 seconds on a warm cache.
verify-llm: check-invariants check-schema
	@echo "Running verify-llm fast gate (schema + docs + core contracts)..."
	@npm run docs:lint:integrity
	@npm run docs:check:strict
	@npm run docs:lint:content-contract
	@go test ./cmd/browser-agent -run 'TestInteract_NavigateAndDocument_.*|TestNavigateAndDocument_.*|TestContractEnforcement_ErrorsHaveRetryableField|TestContractEnforcement_CommandResult_HasElapsedMs' -count=1
	@echo "verify-llm passed"

# Quality gate for top 1% standards (comprehensive)
# `test` already includes the JS suite (test-js).
quality-gate: check-structure lint lint-hardening lint-dead lint-json-casing typecheck security-check test validate-deps-versions
	@echo ""
	@echo "═══════════════════════════════════════════"
	@echo "✅ QUALITY GATE PASSED - Top 1% Standards"
	@echo "═══════════════════════════════════════════"
	@echo "  ✓ File length limits enforced"
	@echo "  ✓ Linting passed (ESLint + go vet)"
	@echo "  ✓ Dead code checked (deadcode + knip)"
	@echo "  ✓ No circular dependencies"
	@echo "  ✓ Import boundaries enforced"
	@echo "  ✓ JSON tags use snake_case"
	@echo "  ✓ Go file headers present"
	@echo "  ✓ Type safety verified (TypeScript)"
	@echo "  ✓ Security checks passed"
	@echo "  ✓ All Go tests passed"
	@echo "  ✓ All TypeScript tests passed"
	@echo "  ✓ Version consistency verified"
	@echo "═══════════════════════════════════════════"

# Upgrade/install guardrail suite: prevents stale daemons from surviving release upgrades.
test-upgrade-guards:
	go test ./cmd/browser-agent -run 'TestConnectWithRetriesRejectsVersionMismatch' -count=1
	node --test scripts/release/install-upgrade-regression.contract.test.mjs
	node scripts/release/run-npm-wrapper-tests.js
	node --test scripts/release/verify-platform-binaries.test.mjs
	@if [ -d pypi/kaboom-agentic-browser/tests ]; then \
		python3 -m unittest discover -s pypi/kaboom-agentic-browser/tests -p 'test_*.py'; \
	else \
		echo "pypi/ not present; skipping PyPI cleanup tests"; \
	fi
	node scripts/release/install-upgrade-regression.mjs

# Release gate for daemon cleanup/version safety.
release-gate: quality-gate test-upgrade-guards
	@echo "✅ release-gate passed"

# Set VERSION and every canonical release target in one validated transaction.
bump-version:
	@if [ -z "$(NEW_VERSION)" ]; then echo "Usage: make bump-version NEW_VERSION=X.Y.Z"; exit 1; fi
	@node scripts/release/version/version-sync.mjs "$(NEW_VERSION)"

# Repair canonical targets from VERSION without changing VERSION.
sync-version:
	@node scripts/release/version/version-sync.mjs --sync

# Fail closed when any canonical target differs from VERSION.
validate-versions:
	@node scripts/release/version/version-sync.mjs --check

context-size:
	@echo "=== Claude Code Initial Context Size ==="
	@echo ""
	@total=0; \
	for f in CLAUDE.md $$(find .claude/docs -name "*.md" -type f 2>/dev/null) $$(find .claude -maxdepth 1 -name "*.md" -type f 2>/dev/null); do \
		chars=$$(wc -c < "$$f"); \
		tokens=$$((chars / 4)); \
		total=$$((total + chars)); \
		printf "%6d tokens  %s\n" "$$tokens" "$$f"; \
	done; \
	echo ""; \
	printf "%6d tokens  TOTAL (always loaded - startup)\n" "$$((total / 4))"; \
	echo ""; \
	ref_total=0; \
	for f in $$(find .claude/refs -name "*.md" -type f 2>/dev/null); do \
		chars=$$(wc -c < "$$f"); \
		ref_total=$$((ref_total + chars)); \
	done; \
	printf "%6d tokens  References (loaded on-demand)\n" "$$((ref_total / 4))"; \
	echo ""; \
	cmd_total=0; \
	for f in $$(find .claude/commands -name "*.md" -type f 2>/dev/null); do \
		chars=$$(wc -c < "$$f"); \
		cmd_total=$$((cmd_total + chars)); \
	done; \
	printf "%6d tokens  Commands (loaded on invoke only)\n" "$$((cmd_total / 4))"; \
	echo ""; \
	echo "Budget: <5K lean | 5-15K normal | 15-30K heavy | >30K too large"

# --- PyPI Distribution ---

pypi-binaries: build
	@echo "Copying binaries to PyPI platform packages..."
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 pypi/kaboom-agentic-browser-darwin-arm64/kaboom_agentic_browser_darwin_arm64/kaboom
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-x64 pypi/kaboom-agentic-browser-darwin-x64/kaboom_agentic_browser_darwin_x64/kaboom
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 pypi/kaboom-agentic-browser-linux-arm64/kaboom_agentic_browser_linux_arm64/kaboom
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-x64 pypi/kaboom-agentic-browser-linux-x64/kaboom_agentic_browser_linux_x64/kaboom
	@cp $(BUILD_DIR)/$(BINARY_NAME)-win32-x64.exe pypi/kaboom-agentic-browser-win32-x64/kaboom_agentic_browser_win32_x64/kaboom.exe
	@echo "Copying hooks binaries to PyPI platform packages..."
	@cp $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-darwin-arm64 pypi/kaboom-agentic-browser-darwin-arm64/kaboom_agentic_browser_darwin_arm64/kaboom-hooks
	@cp $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-darwin-x64 pypi/kaboom-agentic-browser-darwin-x64/kaboom_agentic_browser_darwin_x64/kaboom-hooks
	@cp $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-linux-arm64 pypi/kaboom-agentic-browser-linux-arm64/kaboom_agentic_browser_linux_arm64/kaboom-hooks
	@cp $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-linux-x64 pypi/kaboom-agentic-browser-linux-x64/kaboom_agentic_browser_linux_x64/kaboom-hooks
	@cp $(BUILD_DIR)/$(HOOKS_BINARY_NAME)-win32-x64.exe pypi/kaboom-agentic-browser-win32-x64/kaboom_agentic_browser_win32_x64/kaboom-hooks.exe
	@echo "Binaries copied successfully"

pypi-preflight:
	@python3 -c 'import importlib.util,sys; missing=[n for n in ("build","setuptools","wheel") if importlib.util.find_spec(n) is None]; \
	print("Python build modules available: build, setuptools, wheel") if not missing else ( \
	print("ERROR: Missing Python build modules: " + ", ".join(missing)), \
	print("System Python may be externally managed. Use a virtual environment:") if sys.prefix==sys.base_prefix else print("Install with: python -m pip install --upgrade build setuptools wheel"), \
	print("  python3 -m venv .venv-build") if sys.prefix==sys.base_prefix else None, \
	print("  source .venv-build/bin/activate") if sys.prefix==sys.base_prefix else None, \
	print("  python -m pip install --upgrade pip build setuptools wheel") if sys.prefix==sys.base_prefix else None, \
	sys.exit(1))'

pypi-schema-check:
	@echo "Checking PyPI main pyproject normalization..."
	@node scripts/release/normalize-pypi-main-pyproject.js --check --validate
	@python3 -c 'import sys,tomllib; p="pypi/kaboom-agentic-browser/pyproject.toml"; d=tomllib.load(open(p,"rb")); project=d.get("project", {}); scripts=project.get("scripts", {}); \
	(isinstance(scripts, dict) or (print("ERROR: [project.scripts] must be a TOML table"), sys.exit(1))); \
	("dependencies" not in scripts or (print("ERROR: project.scripts.dependencies must not exist"), sys.exit(1))); \
	print("PyPI schema check ok"); print("project keys:", sorted(project.keys())); print("project.scripts keys:", sorted(scripts.keys())); print("project.dependencies count:", len(project.get("dependencies", [])))'

pypi-build: pypi-preflight pypi-schema-check
	@$(MAKE) pypi-binaries
	@echo "Normalizing PyPI main pyproject metadata..."
	@node scripts/release/normalize-pypi-main-pyproject.js
	@echo "Validating PyPI main pyproject metadata..."
	@node scripts/release/normalize-pypi-main-pyproject.js --validate
	@echo "Building PyPI wheels..."
	@for pkg in pypi/kaboom-agentic-browser-*/; do \
		echo "Building $$pkg..."; \
		(cd "$$pkg" && python3 -m build); \
	done
	@echo "Building main package..."
	@(cd pypi/kaboom-agentic-browser && python3 -m build)
	@echo "All PyPI packages built successfully"
	@echo ""
	@echo "Wheels created:"
	@find pypi -name "*.whl" -type f

pypi-test-publish: pypi-build
	@echo "Publishing to Test PyPI..."
	@echo "NOTE: Requires TWINE_USERNAME and TWINE_PASSWORD environment variables"
	@for pkg in pypi/kaboom-agentic-browser-*/; do \
		echo "Uploading $$pkg..."; \
		(cd "$$pkg" && python3 -m twine upload --repository testpypi dist/*); \
	done
	@echo "Uploading main package..."
	@(cd pypi/kaboom-agentic-browser && python3 -m twine upload --repository testpypi dist/*)
	@echo "All packages published to Test PyPI"
	@echo "Test installation: pip install --index-url https://test.pypi.org/simple/ kaboom-agentic-browser"

pypi-publish: pypi-build
	@echo "Publishing to PyPI..."
	@echo "NOTE: Requires TWINE_USERNAME and TWINE_PASSWORD environment variables"
	@echo "Press Ctrl+C to cancel, or Enter to continue..."
	@read dummy
	@for pkg in pypi/kaboom-agentic-browser-*/; do \
		echo "Uploading $$pkg..."; \
		(cd "$$pkg" && python3 -m twine upload dist/*); \
	done
	@echo "Uploading main package..."
	@(cd pypi/kaboom-agentic-browser && python3 -m twine upload dist/*)
	@echo "All packages published to PyPI"
	@echo "Installation: pip install kaboom-agentic-browser"

pypi-clean:
	@echo "Cleaning PyPI build artifacts..."
	@find pypi -type d -name "build" -exec rm -rf {} + 2>/dev/null || true
	@find pypi -type d -name "dist" -exec rm -rf {} + 2>/dev/null || true
	@find pypi -type d -name "*.egg-info" -exec rm -rf {} + 2>/dev/null || true
	@find pypi -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	@echo "PyPI artifacts cleaned"

# --- Docs Site (gokaboom.dev) ---

site-dev:
	cd gokaboom.dev && npm run dev

site-build:
	cd gokaboom.dev && npm run build

site-preview:
	cd gokaboom.dev && npm run preview

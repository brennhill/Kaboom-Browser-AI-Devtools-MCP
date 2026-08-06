#!/usr/bin/env bash
# lint-hardening.sh — Custom linters for Kaboom codebase hardening.
# Catches patterns that standard linters miss: unprotected goroutines,
# missing Content-Type headers, unchecked JSON encodes, route mismatches.
#
# Usage: ./scripts/quality/verification/lint-hardening.sh [--fix]
# Exit 0 = clean, Exit 1 = violations found

set -euo pipefail

FAIL=0
WARNINGS=0

red()    { printf '\033[0;31m%s\033[0m\n' "$1"; }
yellow() { printf '\033[0;33m%s\033[0m\n' "$1"; }
green()  { printf '\033[0;32m%s\033[0m\n' "$1"; }
bold()   { printf '\033[1m%s\033[0m\n' "$1"; }

fail() { red "  FAIL: $1"; FAIL=$((FAIL + 1)); }
warn() { yellow "  WARN: $1"; WARNINGS=$((WARNINGS + 1)); }
pass() { green "  PASS: $1"; }

# ─────────────────────────────────────────────
# 1. No bare go func() in production code
# ─────────────────────────────────────────────
bold "1. Checking for bare go func() (should use util.SafeGo)..."

BARE_GO=$(grep -rn 'go func()' cmd/browser-agent/ internal/ \
  --include='*.go' \
  | grep -v '_test.go' \
  | grep -v 'SafeGo' \
  | grep -v 'safego\.go' \
  | grep -v '// lint:allow-bare-goroutine' \
  || true)

if [ -n "$BARE_GO" ]; then
  fail "Found bare go func() without SafeGo (use util.SafeGo or add // lint:allow-bare-goroutine):"
  echo "$BARE_GO" | while IFS= read -r line; do echo "    $line"; done
else
  pass "No bare go func() found"
fi

# ─────────────────────────────────────────────
# 2. Unchecked json.NewEncoder(w).Encode()
# ─────────────────────────────────────────────
bold "2. Checking for unchecked json.NewEncoder(w).Encode()..."

# Match json.NewEncoder(w).Encode( NOT preceded by _ = or err =
UNCHECKED_ENCODE=$(grep -rn 'json\.NewEncoder(w)\.Encode(' cmd/browser-agent/ internal/ \
  --include='*.go' \
  | grep -v '_test.go' \
  | grep -v '_ = json\.NewEncoder' \
  | grep -v '//nolint' \
  | grep -v 'err.*=.*json\.NewEncoder' \
  || true)

if [ -n "$UNCHECKED_ENCODE" ]; then
  fail "Found unchecked json.NewEncoder(w).Encode() (prefix with '_ =' or assign error):"
  echo "$UNCHECKED_ENCODE" | while IFS= read -r line; do echo "    $line"; done
else
  pass "All json.NewEncoder(w).Encode() calls are checked"
fi

# ─────────────────────────────────────────────
# 3. WriteHeader before Content-Type
# ─────────────────────────────────────────────
bold "3. Checking for WriteHeader before Content-Type..."

# This is a heuristic: look for w.WriteHeader followed by json.NewEncoder
# without Content-Type set between them. We check individual files.
HEADER_ORDER_ISSUES=""
for f in $(find cmd/browser-agent/ internal/ -name '*.go' ! -name '*_test.go'); do
  # Find lines where WriteHeader is called with an error status, then check
  # if Content-Type was set before it (within 3 lines above)
  while IFS= read -r line_info; do
    lineno=$(echo "$line_info" | cut -d: -f1)
    # Check 3 lines before for Content-Type
    start=$((lineno > 3 ? lineno - 3 : 1))
    context=$(sed -n "${start},${lineno}p" "$f")
    if ! echo "$context" | grep -q 'Content-Type'; then
      # Check if this WriteHeader is for an error (not 200/204)
      if echo "$line_info" | grep -qE 'StatusBadRequest|StatusInternalServerError|StatusNotFound|StatusForbidden|StatusUnauthorized'; then
        HEADER_ORDER_ISSUES="${HEADER_ORDER_ISSUES}    ${f}:${line_info}\n"
      fi
    fi
  done < <(grep -n 'w\.WriteHeader(' "$f" 2>/dev/null || true)
done

if [ -n "$HEADER_ORDER_ISSUES" ]; then
  warn "Possible WriteHeader before Content-Type (check manually):"
  printf "%b" "$HEADER_ORDER_ISSUES"
else
  pass "No obvious WriteHeader-before-Content-Type issues"
fi

# ─────────────────────────────────────────────
# 4. Route ↔ OpenAPI sync check
# ─────────────────────────────────────────────
bold "4. Checking route ↔ OpenAPI sync..."

# Public daemon route registration is owned by server.go.
SERVER_ROUTE_FILES="cmd/browser-agent/server.go"

# Extract registered routes from Go source
GO_ROUTES=$(grep -h -o 'HandleFunc("[^"]*"' $SERVER_ROUTE_FILES \
  | sed 's/HandleFunc("//;s/"//' \
  | sort -u)

# Extract paths from openapi.json
OPENAPI_PATHS=$(python3 -c "
import json, sys
with open('cmd/browser-agent/openapi.json') as f:
    spec = json.load(f)
for p in sorted(spec.get('paths', {}).keys()):
    print(p)
" 2>/dev/null || true)

ROUTE_SYNC_OK=true

# Check Go routes exist in OpenAPI (skip / and /clients/ which is a prefix pattern)
for route in $GO_ROUTES; do
  if [ "$route" = "/" ] || [ "$route" = "/clients/" ] || [ "$route" = "/api/status" ] || [ "$route" = "/logs.html" ] || [ "$route" = "/setup" ] || [ "$route" = "/docs" ]; then
    continue
  fi
  if ! echo "$OPENAPI_PATHS" | grep -qx "$route"; then
    fail "Route $route registered in Go but missing from openapi.json"
    ROUTE_SYNC_OK=false
  fi
done

# Check OpenAPI paths are registered in Go (skip parameterized paths)
for path in $OPENAPI_PATHS; do
  # Skip paths with parameters like /clients/{id}
  if echo "$path" | grep -q '{'; then
    base=$(echo "$path" | sed 's/{[^}]*}//')
    if ! echo "$GO_ROUTES" | grep -q "^${base}"; then
      fail "OpenAPI path $path has no Go route (checked prefix $base)"
      ROUTE_SYNC_OK=false
    fi
    continue
  fi
  if ! echo "$GO_ROUTES" | grep -qx "$path"; then
    fail "OpenAPI path $path not registered in Go server_routes*.go"
    ROUTE_SYNC_OK=false
  fi
done

if [ "$ROUTE_SYNC_OK" = true ]; then
  pass "All routes match between Go and OpenAPI ($(echo "$GO_ROUTES" | wc -l | tr -d ' ') routes)"
fi

# ─────────────────────────────────────────────
# 5. Extension-only middleware check
# ─────────────────────────────────────────────
bold "5. Checking httpguard.ExtensionOnly middleware on ingest endpoints..."

# These endpoints MUST have httpguard.ExtensionOnly (extension-facing, data ingest)
MUST_HAVE_EXTENSION_ONLY=(
  "/websocket-events"
  "/network-bodies"
  "/network-waterfall"
  "/query-result"
  "/enhanced-actions"
  "/performance-snapshots"
  "/sync"
  "/screenshots"
  "/logs"
  "/draw-mode/complete"
  "/shutdown"
  "/recordings/save"
  "/recordings/storage"
  "/recordings/reveal"
  "/snapshot"
  "/clear"
  "/test-boundary"
)

EXT_ONLY_OK=true
for ep in "${MUST_HAVE_EXTENSION_ONLY[@]}"; do
  # Find the HandleFunc line for this endpoint
  line=$(grep -h "HandleFunc(\"${ep}\"" $SERVER_ROUTE_FILES || true)
  if [ -z "$line" ]; then
    continue  # Route not found (might be registered differently)
  fi
  if ! echo "$line" | grep -q 'httpguard\.ExtensionOnly'; then
    fail "Endpoint $ep missing httpguard.ExtensionOnly middleware"
    EXT_ONLY_OK=false
  fi
done

if [ "$EXT_ONLY_OK" = true ]; then
  pass "All ingest/extension endpoints have httpguard.ExtensionOnly middleware"
fi

# ─────────────────────────────────────────────
# 6. Empty error responses (WriteHeader without body)
# ─────────────────────────────────────────────
bold "6. Checking for empty error responses..."

# Find WriteHeader with error status that's immediately followed by return
EMPTY_ERRORS=$(grep -rn -A1 'w\.WriteHeader(http\.Status' cmd/browser-agent/ internal/ \
  --include='*.go' \
  | grep -v '_test.go' \
  | grep -B1 'return$' \
  | grep 'WriteHeader' \
  | grep -vE 'StatusOK|StatusNoContent|StatusMethodNotAllowed' \
  || true)

if [ -n "$EMPTY_ERRORS" ]; then
  warn "Error responses without JSON body (consider adding error JSON):"
  echo "$EMPTY_ERRORS" | while IFS= read -r line; do echo "    $line"; done
else
  pass "All error responses include a body"
fi

# ─────────────────────────────────────────────
# 7. SafeGo closures must not capture struct fields by reference
# ─────────────────────────────────────────────
bold "7. Checking SafeGo closures for struct field capture..."

SAFEGO_CAPTURES=""
for f in $(find cmd/browser-agent/ internal/ -name '*.go' ! -name '*_test.go'); do
  while IFS= read -r line_info; do
    lineno=$(echo "$line_info" | cut -d: -f1)
    # Check next 10 lines for struct field references without local capture
    end=$((lineno + 10))
    context=$(sed -n "${lineno},${end}p" "$f")
    if echo "$context" | grep -qE '\bcb\.[a-zA-Z]|\bc\.ext\.\b|\bc\.elb\.\b'; then
      if ! echo "$context" | grep -q 'lint:safe-closure'; then
        SAFEGO_CAPTURES="${SAFEGO_CAPTURES}    ${f}:${lineno}: SafeGo closure may capture struct fields by reference\n"
      fi
    fi
  done < <(grep -n 'SafeGo(func()' "$f" 2>/dev/null || true)
done

if [ -n "$SAFEGO_CAPTURES" ]; then
  fail "SafeGo closures capturing struct fields (copy to locals or add // lint:safe-closure):"
  printf "%b" "$SAFEGO_CAPTURES"
else
  pass "All SafeGo closures properly capture values"
fi

# ─────────────────────────────────────────────
# 8. Queue overflow drops must be logged
# ─────────────────────────────────────────────
bold "8. Checking queue overflow logging..."

OVERFLOW_DROPS=$(grep -rn 'pendingQueries\[1:\]' cmd/browser-agent/ internal/ \
  --include='*.go' \
  | grep -v '_test.go' \
  || true)

if [ -n "$OVERFLOW_DROPS" ]; then
  OVERFLOW_OK=true
  while IFS= read -r line_info; do
    file=$(echo "$line_info" | cut -d: -f1)
    lineno=$(echo "$line_info" | cut -d: -f2)
    # Check 5 lines before for logging
    start=$((lineno > 5 ? lineno - 5 : 1))
    context=$(sed -n "${start},${lineno}p" "$file")
    if ! echo "$context" | grep -qE 'Fprintf|log\.|emit|Stderr'; then
      fail "Queue overflow at ${file}:${lineno} missing log/emit"
      OVERFLOW_OK=false
    fi
  done <<< "$OVERFLOW_DROPS"
  if [ "$OVERFLOW_OK" = true ]; then
    pass "All queue overflow drops are logged"
  fi
else
  pass "No queue overflow patterns found (or already cleaned up)"
fi

# ─────────────────────────────────────────────
# 9. Lock() without defer Unlock() (panic-unsafe)
# ─────────────────────────────────────────────
bold "9. Checking for Lock() without defer Unlock()..."

MANUAL_UNLOCK=""
for f in $(find cmd/browser-agent/ internal/ -name '*.go' ! -name '*_test.go'); do
  while IFS= read -r line_info; do
    lineno=$(echo "$line_info" | cut -d: -f1)
    # Check next 3 lines for defer Unlock
    end=$((lineno + 3))
    context=$(sed -n "${lineno},${end}p" "$f")
    if ! echo "$context" | grep -q 'defer.*Unlock'; then
      if ! echo "$context" | grep -q '// lint:manual-unlock'; then
        MANUAL_UNLOCK="${MANUAL_UNLOCK}    ${f}:${lineno}: Lock() without defer Unlock()\n"
      fi
    fi
  done < <(grep -n '\.Lock()$' "$f" 2>/dev/null | grep -v 'RLock' || true)
done

if [ -n "$MANUAL_UNLOCK" ]; then
  warn "Lock() without defer Unlock() (add defer or // lint:manual-unlock):"
  printf "%b" "$MANUAL_UNLOCK"
else
  pass "All Lock() calls use defer Unlock()"
fi

# ─────────────────────────────────────────────
# 10. resp.Body.Close() without defer
# ─────────────────────────────────────────────
bold "10. Checking for resp.Body.Close() without defer..."

# Check each Body.Close() and look at the line above for 'defer func()'
BODY_CLOSE_NO_DEFER=""
for f in $(find cmd/browser-agent/ internal/ -name '*.go' ! -name '*_test.go'); do
  while IFS= read -r line_info; do
    lineno=$(echo "$line_info" | cut -d: -f1)
    line_text=$(echo "$line_info" | cut -d: -f2-)
    # Skip if 'defer' is on the same line
    if echo "$line_text" | grep -q 'defer'; then continue; fi
    if echo "$line_text" | grep -q '// lint:body-close-ok'; then continue; fi
    if echo "$line_text" | grep -q '//nolint'; then continue; fi
    # Check line above for 'defer func()'
    prev=$((lineno - 1))
    prev_line=$(sed -n "${prev}p" "$f")
    if echo "$prev_line" | grep -q 'defer func()'; then continue; fi
    BODY_CLOSE_NO_DEFER="${BODY_CLOSE_NO_DEFER}    ${f}:${lineno}: ${line_text}\n"
  done < <(grep -n '\.Body\.Close()' "$f" 2>/dev/null || true)
done

if [ -n "$BODY_CLOSE_NO_DEFER" ]; then
  warn "resp.Body.Close() without defer (use defer or add // lint:body-close-ok):"
  echo "$BODY_CLOSE_NO_DEFER" | while IFS= read -r line; do echo "    $line"; done
else
  pass "All resp.Body.Close() calls use defer"
fi

# ─────────────────────────────────────────────
# 11. http.Error() in handler files (should use jsonResponse)
# ─────────────────────────────────────────────
bold "11. Checking for http.Error() in handler files..."

HTTP_ERROR_CALLS=$(grep -rn 'http\.Error(' cmd/browser-agent/ \
  --include='*.go' \
  | grep -v '_test.go' \
  | grep -v 'internal/httpguard/middleware.go' \
  | grep -v '// lint:http-error-ok' \
  || true)

if [ -n "$HTTP_ERROR_CALLS" ]; then
  warn "http.Error() used instead of jsonResponse (use jsonResponse or add // lint:http-error-ok):"
  echo "$HTTP_ERROR_CALLS" | while IFS= read -r line; do echo "    $line"; done
else
  pass "No http.Error() in handler files (all use jsonResponse)"
fi

# ─────────────────────────────────────────────
# 12. signal() must be called before return in bridge dispatch
# ─────────────────────────────────────────────
bold "12. Checking bridge dispatch signal() coverage..."

BRIDGE_FILE="cmd/browser-agent/internal/bridge/bridge.go"
if [ -f "$BRIDGE_FILE" ]; then
  BRIDGE_SIGNAL_ISSUES=""
  FUNC_START=$(grep -nE 'func (\([^)]*\) )?bridgeForwardRequest' "$BRIDGE_FILE" | head -1 | cut -d: -f1)
  if [ -n "$FUNC_START" ]; then
    FUNC_END=$(awk -v start="$FUNC_START" 'NR>=start { if (/^}/) { print NR; exit } }' "$BRIDGE_FILE")
    if [ -n "$FUNC_END" ]; then
      # Extract function body, find return lines, check for signal() in context
      RETURN_LINES=$(sed -n "${FUNC_START},${FUNC_END}p" "$BRIDGE_FILE" | grep -n 'return$' | cut -d: -f1 || true)
      for rel_line in $RETURN_LINES; do
        abs_line=$((FUNC_START + rel_line - 1))
        ctx_start=$((abs_line > 3 ? abs_line - 3 : FUNC_START))
        context=$(sed -n "${ctx_start},${abs_line}p" "$BRIDGE_FILE")
        if ! echo "$context" | grep -q 'signal()'; then
          BRIDGE_SIGNAL_ISSUES="${BRIDGE_SIGNAL_ISSUES}    ${BRIDGE_FILE}:${abs_line}: return without signal()\n"
        fi
      done
    fi
  fi

  if [ -n "$BRIDGE_SIGNAL_ISSUES" ]; then
    fail "bridgeDispatchRequest has return paths without signal():"
    printf "%b" "$BRIDGE_SIGNAL_ISSUES"
  else
    pass "All bridge dispatch return paths call signal()"
  fi
else
  # Do NOT pass here. This check silently reported "PASS: bridge.go not found
  # (skipped)" for ~4 months after the file moved to internal/bridge/, so a
  # green run meant nothing. A check whose target has vanished has not passed —
  # it has stopped running, and that must be loud (repo rule 25).
  fail "bridge dispatch check cannot run: $BRIDGE_FILE not found (did it move?)"
fi

# ─────────────────────────────────────────────
# 13. Nil map write after conditional init
# ─────────────────────────────────────────────
bold "13. Checking for nil map write after conditional init..."

NIL_MAP_ISSUES=""
for f in $(find cmd/browser-agent/ internal/ -name '*.go' ! -name '*_test.go'); do
  # Find 'var XXX map[' declarations and check if they're always initialized before use
  while IFS= read -r line_info; do
    lineno=$(echo "$line_info" | cut -d: -f1)
    # Extract variable name
    varname=$(echo "$line_info" | sed 's/.*var \([a-zA-Z_]*\) map.*/\1/')
    if [ -z "$varname" ]; then continue; fi
    # Check next 20 lines for unconditional write to the map
    end=$((lineno + 20))
    context=$(sed -n "$((lineno+1)),${end}p" "$f")
    # Look for map write (varname[) that's NOT inside an if block after initialization
    if echo "$context" | grep -q "${varname}\[" && \
       echo "$context" | grep -q "if len(" && \
       ! echo "$context" | grep -q "if ${varname} == nil"; then
      # Potential nil map write — only flag if the init is conditional
      if echo "$context" | grep -qE "if len\(args\)|if len\(raw\)"; then
        NIL_MAP_ISSUES="${NIL_MAP_ISSUES}    ${f}:${lineno}: '${varname}' may be nil when written\n"
      fi
    fi
  done < <(grep -n 'var [a-zA-Z_]* map\[' "$f" 2>/dev/null || true)
done

if [ -n "$NIL_MAP_ISSUES" ]; then
  fail "Possible nil map write (add nil guard or initialize unconditionally):"
  printf "%b" "$NIL_MAP_ISSUES"
else
  pass "No nil map write patterns detected"
fi

# ─────────────────────────────────────────────
# 14. MCP stdout invariant: no direct stdout writes in production code
# ─────────────────────────────────────────────
bold "14. Checking for forbidden stdout writes in production code..."

# Files that are ALLOWED to write to stdout (transport layer only):
#   bridge_adapter.go  — centralized writeMCPPayload()
#   bridge_io_isolation*.go — FD duplication setup
#   cli.go / cli_output.go — CLI mode (not MCP)
#   main.go            — startup / background spawn
#   main_connection.go — MCP pipe writes
#   main_connection_stop*.go — CLI stop/force command output
STDOUT_ALLOWLIST="bridge_io_isolation|cli\.go|cli_output\.go|config\.go|main(_.*)?\.go|main_connection_stop(_.*)?\.go|main_helpers\.go|doctor(_.*)?\.go"

# Pattern: fmt.Print/Println/Printf (writes to stdout), or direct os.Stdout usage
# Excludes: fmt.Fprintf(os.Stderr, ...) which is safe, and fmt.Sprintf which returns a string
STDOUT_VIOLATIONS=$(grep -rn -E 'fmt\.(Print|Println|Printf)\(' cmd/browser-agent/ \
  --include='*.go' \
  | grep -v '_test.go' \
  | grep -v "// lint:stdout-ok" \
  | grep -vE "$STDOUT_ALLOWLIST" \
  || true)

if [ -n "$STDOUT_VIOLATIONS" ]; then
  fail "Direct stdout writes via fmt.Print*/Println/Printf (use stderrf for logging, writeMCPPayload for MCP):"
  echo "$STDOUT_VIOLATIONS" | while IFS= read -r line; do echo "    $line"; done
else
  pass "No forbidden fmt.Print*/Println/Printf in production code"
fi

# ─────────────────────────────────────────────
# 15. Daemon-mode stderr: must use stderrf() not fmt.Fprintf(os.Stderr)
# ─────────────────────────────────────────────
bold "15. Checking daemon-mode files use stderrf() for stderr..."

# Daemon/server files that MUST use stderrf() instead of fmt.Fprintf(os.Stderr)
DAEMON_FILES=(
  "handler.go"
  "server.go"
  "server_routes.go"
  "server_routes_media.go"
  "openapi.go"
  "dashboard.go"
  "noise_autorun.go"
)

STDERR_VIOLATIONS=""
for fname in "${DAEMON_FILES[@]}"; do
  fpath="cmd/browser-agent/$fname"
  if [ ! -f "$fpath" ]; then continue; fi
  HITS=$(grep -n 'fmt\.Fprintf(os\.Stderr' "$fpath" \
    | grep -v '// lint:stderr-ok' \
    || true)
  if [ -n "$HITS" ]; then
    while IFS= read -r line; do
      STDERR_VIOLATIONS="${STDERR_VIOLATIONS}    ${fpath}:${line}\n"
    done <<< "$HITS"
  fi
done

if [ -n "$STDERR_VIOLATIONS" ]; then
  fail "Daemon-mode files using fmt.Fprintf(os.Stderr) instead of stderrf():"
  printf "%b" "$STDERR_VIOLATIONS"
else
  pass "All daemon-mode files use stderrf() for stderr output"
fi

# ─────────────────────────────────────────────
# 16. Eval packages must not be imported by production binaries
# ─────────────────────────────────────────────
bold "16. Checking eval packages are not imported by production code..."

EVAL_IMPORTS=$(grep -rn 'hook/eval' cmd/ internal/ \
  --include='*.go' \
  | grep -v '_test.go' \
  | grep -v 'internal/hook/eval/' \
  || true)

if [ -n "$EVAL_IMPORTS" ]; then
  fail "Production code imports eval package (eval must stay test-only):"
  echo "$EVAL_IMPORTS" | while IFS= read -r line; do echo "    $line"; done
else
  pass "Eval packages are test-only — not imported by any production binary"
fi

# ─────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────
echo ""
bold "─── Summary ───"
if [ "$FAIL" -gt 0 ]; then
  red "$FAIL failure(s), $WARNINGS warning(s)"
  exit 1
elif [ "$WARNINGS" -gt 0 ]; then
  yellow "0 failures, $WARNINGS warning(s)"
  exit 0
else
  green "All hardening checks passed"
  exit 0
fi

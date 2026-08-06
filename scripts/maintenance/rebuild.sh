#!/bin/bash
# rebuild.sh — Kill all daemons, remove stale binaries, rebuild from source, install.
# Usage: ./scripts/maintenance/rebuild.sh [--no-install]
#   --no-install  Skip copying to /usr/local/bin (local ./kaboom-agentic-browser only)
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/../.."
cd "$PROJECT_ROOT"
CMD_PKG="${KABOOM_CMD_PKG:-./cmd/browser-agent}"
CMD_DIR="${CMD_PKG#./}"
VERSION_RAW="$(tr -d '[:space:]' < "$PROJECT_ROOT/VERSION" 2>/dev/null || true)"
VERSION_TAG="$(echo "$VERSION_RAW" | tr -cd '0-9')"
if [ -z "$VERSION_TAG" ]; then
    VERSION_TAG="dev"
fi
VERSIONED_BIN_NAME="kaboom-agentic-browser-$VERSION_TAG"
VERSIONED_LOCAL_PATH="$PROJECT_ROOT/$VERSIONED_BIN_NAME"
VERSIONED_INSTALL_PATH="/usr/local/bin/$VERSIONED_BIN_NAME"

# Colors (if TTY)
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    G='\033[0;32m'; R='\033[0;31m'; Y='\033[0;33m'; B='\033[0;34m'; X='\033[0m'
else
    G=''; R=''; Y=''; B=''; X=''
fi

step() { echo -e "${B}▸${X} $1"; }
ok()   { echo -e "  ${G}✓${X} $1"; }
warn() { echo -e "  ${Y}⚠${X} $1"; }
err()  { echo -e "  ${R}✗${X} $1"; }

INSTALL=true
[ "${1:-}" = "--no-install" ] && INSTALL=false

# ── Step 1: Kill all running daemons ─────────────────────
step "Killing all Kaboom daemon processes..."
killed=0

# Kill by canonical process name.
process_pattern="kaboom-agentic-browser"
if pgrep -f "$process_pattern" >/dev/null 2>&1; then
    pids=$(pgrep -f "$process_pattern" | tr '\n' ' ')
    kill $pids 2>/dev/null || true
    sleep 0.3
    if pgrep -f "$process_pattern" >/dev/null 2>&1; then
        kill -9 $(pgrep -f "$process_pattern") 2>/dev/null || true
    fi
    killed=1
fi

# Kill anything on port 7890
if lsof -ti :7890 >/dev/null 2>&1; then
    lsof -ti :7890 | xargs kill -9 2>/dev/null || true
    killed=1
fi

if [ "$killed" = "1" ]; then
    ok "All Kaboom daemon processes killed"
else
    ok "No running processes found"
fi

# ── Step 2: Remove stale binaries ────────────────────────
step "Removing stale binaries..."

# Local project binaries
local_bin="./kaboom-agentic-browser"
if [ -f "$local_bin" ]; then
    rm -f "$local_bin"
    ok "Removed $local_bin"
fi

# Local versioned binary
if [ -f "$VERSIONED_LOCAL_PATH" ]; then
    rm -f "$VERSIONED_LOCAL_PATH"
    ok "Removed $VERSIONED_LOCAL_PATH"
fi

# System binaries
system_bin="/usr/local/bin/kaboom-agentic-browser"
if [ -f "$system_bin" ]; then
    rm -f "$system_bin"
    ok "Removed $system_bin"
fi

# Remove stale system versioned binaries
for stale_glob in /usr/local/bin/kaboom-agentic-browser-[0-9]* /usr/local/bin/kaboom-agentic-browser-dev; do
    if [ -e "$stale_glob" ]; then
        rm -f "$stale_glob"
        ok "Removed $stale_glob"
    fi
done

# ── Step 3: Compile TypeScript (if src/ changed) ─────────
if [ -d "src" ]; then
    src_newest_ts=$(find src -name '*.ts' -newer extension/background.js 2>/dev/null | head -1)
    if [ -n "$src_newest_ts" ]; then
        step "TypeScript sources changed, recompiling..."
        if make compile-ts >/dev/null 2>&1; then
            ok "TypeScript compiled"
        else
            warn "TypeScript compilation failed (non-fatal)"
        fi
    else
        ok "TypeScript up to date"
    fi
fi

# ── Step 4: Rebuild from source ──────────────────────────
step "Building from source..."
if ! go build -o kaboom-agentic-browser "$CMD_PKG"; then
    err "Build failed!"
    exit 1
fi

cp ./kaboom-agentic-browser "$VERSIONED_LOCAL_PATH"
ok "Created ./$VERSIONED_BIN_NAME"

# Verify the binary runs
build_version=$(./kaboom-agentic-browser --version 2>&1 || true)
ok "Built ./kaboom-agentic-browser — ${build_version}"

# ── Step 5: Install to PATH ─────────────────────────────
if [ "$INSTALL" = "true" ]; then
    step "Installing to /usr/local/bin..."
    # Symlink directly to the project binary — no copy needed on rebuild.
    ABSOLUTE_BIN="$(cd "$PROJECT_ROOT" && pwd)/$VERSIONED_BIN_NAME"
    ln -sfn "$ABSOLUTE_BIN" /usr/local/bin/kaboom-agentic-browser
    ok "Symlinked /usr/local/bin/kaboom-agentic-browser -> $ABSOLUTE_BIN"
fi

# ── Step 6: Verify ──────────────────────────────────────
step "Verifying..."
ok "Binary: $(which kaboom-agentic-browser 2>/dev/null || echo './kaboom-agentic-browser')"
ok "Points to: $(readlink /usr/local/bin/kaboom-agentic-browser 2>/dev/null || echo 'direct binary')"

# Source vs binary timestamp check
src_newest=$(find "$CMD_DIR" -name '*.go' -newer ./kaboom-agentic-browser 2>/dev/null | head -1)
if [ -n "$src_newest" ]; then
    warn "Source file newer than binary: $src_newest (this should not happen after fresh build)"
else
    ok "Binary is up to date with source"
fi

# ── Step 7: Restart daemon ──────────────────────────────
step "Restarting daemon..."
kaboom-agentic-browser --stop 2>/dev/null || true
sleep 0.5
kaboom-agentic-browser --daemon &
sleep 1
if pgrep -f "kaboom-agentic-browser" >/dev/null 2>&1; then
    ok "Daemon running (PID $(pgrep -f 'kaboom-agentic-browser' | head -1))"
else
    warn "Daemon may not have started — check logs"
fi

echo ""
echo -e "${G}Done.${X} Rebuilt, installed, and restarted."

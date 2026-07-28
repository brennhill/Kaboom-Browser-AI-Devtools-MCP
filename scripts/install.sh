#!/bin/bash
# Kaboom - The Ultimate One-liner Installer
# https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP
#
# PURPOSE:
# This script provides a zero-dependency, platform-aware installation flow for Kaboom.
# It handles binary acquisition, extension staging, and native configuration in one go.
#
# USAGE:
#   curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash
#   curl -sSL ... | bash -s -- --hooks-only   # Install only the hooks binary
#
# NOTE: this script requires bash (set -o pipefail, echo -e, == tests) — never pipe it to sh.

# Fail immediately if a command fails (-e), an unset variable is used (-u),
# or a command in a pipeline fails (-o pipefail). This is critical for installer safety.
set -euo pipefail

# ─────────────────────────────────────────────────────────────
# CLI flag parsing
# ─────────────────────────────────────────────────────────────

HOOKS_ONLY="${KABOOM_HOOKS_ONLY:-0}"
for arg in "$@"; do
    case "$arg" in
        --hooks-only) HOOKS_ONLY=1 ;;
    esac
done

# Configuration: Define the single source of truth for paths and repository metadata.
REPO="brennhill/Kaboom-Browser-AI-Devtools-MCP"
INSTALL_DIR="$HOME/.kaboom"
BIN_DIR="$INSTALL_DIR/bin"
EXT_DIR="${KABOOM_EXTENSION_DIR:-$HOME/KaboomAgenticDevtoolExtension}"
STAGE_EXT_DIR="$INSTALL_DIR/.extension-stage-$$"
BACKUP_EXT_DIR="$INSTALL_DIR/.extension-backup-$$"
# The VERSION file on the STABLE branch is the source of truth for the latest release.
VERSION_URL="https://raw.githubusercontent.com/$REPO/STABLE/VERSION"
STRICT_CHECKSUM="${KABOOM_INSTALL_STRICT:-0}"
# Minimum plausible binary sizes. Catches truncated downloads and HTML error pages.
MIN_BINARY_BYTES=5000000
MIN_HOOKS_BINARY_BYTES=2000000

# UI: Define colors for high-visibility terminal output.
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
ORANGE='\033[38;5;208m'
BOLD='\033[1m'
NC='\033[0m' # No Color (Reset)

# Anonymous install error beacon (disable: KABOOM_TELEMETRY=off).
# Fire-and-forget, never blocks, never fails the install.
beacon_error() {
    local step="${1:-unknown}"
    if [ "${KABOOM_TELEMETRY:-}" = "off" ]; then return; fi
    curl -s --max-time 2 -X POST "https://t.gokaboom.dev/v1/event" \
        -H "Content-Type: application/json" \
        -d "{\"event\":\"install_error\",\"v\":\"${VERSION:-unknown}\",\"os\":\"$(uname -s)-$(uname -m)\",\"props\":{\"step\":\"${step}\",\"method\":\"curl\"}}" \
        > /dev/null 2>&1 || true
}

# Cleanup: Ensure temporary files are removed even if the script crashes or is interrupted.
# Uses mktemp to prevent predictable filename attacks.
TEMP_ROOT=$(mktemp -d)
cleanup() {
    # An interrupt during extension promotion can leave the backup as the ONLY
    # copy of the user's extension (EXT_DIR was already moved aside). Restore
    # it instead of deleting it; debris is only removed once EXT_DIR exists.
    if [ ! -d "$EXT_DIR" ] && [ -d "$BACKUP_EXT_DIR" ]; then
        mv "$BACKUP_EXT_DIR" "$EXT_DIR" 2>/dev/null || true
    fi
    rm -rf "$STAGE_EXT_DIR" "$BACKUP_EXT_DIR"
    rm -rf "$TEMP_ROOT"
}
trap cleanup EXIT

echo -e "${ORANGE}${BOLD}"
cat <<'EOF'
  _  __     ____   ___   ___  __  __ _
 | |/ /__ _| __ ) / _ \ / _ \|  \/  | |
 | ' // _` |  _ \| | | | | | | |\/| | |
 | . \ (_| | |_) | |_| | |_| | |  | |_|
 |_|\_\__,_|____/ \___/ \___/|_|  |_(_)
EOF
echo -e "${NC}"
if [ "$HOOKS_ONLY" = "1" ]; then
    echo -e "${ORANGE}${BOLD}KaBOOM! Hooks Installer${NC} (hooks-only mode)"
else
    echo -e "${ORANGE}${BOLD}KaBOOM! Installer${NC}"
fi
echo -e "${BLUE}--------------------------------------------------${NC}"
if [ "$STRICT_CHECKSUM" = "1" ]; then
    echo -e "Strict checksum mode enabled (KABOOM_INSTALL_STRICT=1)"
fi

# ─────────────────────────────────────────────────────────────
# Prerequisite Checks
# ─────────────────────────────────────────────────────────────

check_prerequisites() {
    local missing=""

    if ! command -v curl >/dev/null 2>&1; then
        missing="${missing}  - curl (required for downloads)\n"
    fi
    if [ "$HOOKS_ONLY" != "1" ] && ! command -v unzip >/dev/null 2>&1; then
        missing="${missing}  - unzip (required for extension extraction)\n"
    fi

    if [ -n "$missing" ]; then
        echo -e "${RED}Missing required tools:${NC}"
        echo -e "$missing"
        echo -e "Install them with your package manager and re-run."
        exit 1
    fi
}

check_disk_space() {
    # Full install: ~50 MB (binary + extension + temp files).
    # Hooks only:   ~15 MB (hooks binary + temp files).
    local required_mb=50
    if [ "$HOOKS_ONLY" = "1" ]; then
        required_mb=15
    fi
    local available_mb=0

    if command -v df >/dev/null 2>&1; then
        # df -Pm gives POSIX-portable megabyte output; grab the mount containing $HOME.
        available_mb=$(df -Pm "$HOME" 2>/dev/null | awk 'NR==2 {print $4}' || echo "0")
    fi

    if [ "${available_mb:-0}" -gt 0 ] && [ "$available_mb" -lt "$required_mb" ]; then
        echo -e "${RED}Insufficient disk space: ${available_mb} MB available, need ${required_mb} MB.${NC}"
        echo -e "Free up space in $HOME and re-run."
        exit 1
    fi
}

check_network_connectivity() {
    # Quick connectivity probe — catch proxy/firewall/offline errors early
    # with a clear message instead of a raw curl failure later.
    if ! curl -fsSL --max-time 10 -o /dev/null "https://github.com" 2>/dev/null; then
        echo -e "${YELLOW}Cannot reach github.com — check your network connection or proxy settings.${NC}"
        echo -e "If you are behind a corporate proxy, set https_proxy before running."
        exit 1
    fi
}

check_prerequisites
check_disk_space
check_network_connectivity

# ─────────────────────────────────────────────────────────────
# Retry-capable download helper
# ─────────────────────────────────────────────────────────────

# curl_retry wraps curl with automatic retry for transient network errors.
# Usage: curl_retry <output_file> <url> [extra_curl_flags...]
# Retries up to 3 times with exponential backoff (2s, 4s, 8s).
curl_retry() {
    local output="$1"
    local url="$2"
    shift 2
    local max_attempts=3
    local attempt=1
    local delay=2

    while [ $attempt -le $max_attempts ]; do
        if curl -fsSL --connect-timeout 15 --max-time 120 "$@" "$url" -o "$output" 2>/dev/null; then
            return 0
        fi
        if [ $attempt -lt $max_attempts ]; then
            echo -e "${YELLOW}  Download attempt $attempt/$max_attempts failed; retrying in ${delay}s...${NC}"
            sleep $delay
            delay=$((delay * 2))
        fi
        attempt=$((attempt + 1))
    done
    return 1
}

# assert_safe_ext_dir refuses obviously catastrophic extension targets before
# any destructive rm/mv of EXT_DIR — KABOOM_EXTENSION_DIR is taken verbatim
# from the environment (mirrors safe_rm_rf in scripts/uninstall.sh).
assert_safe_ext_dir() {
    case "${EXT_DIR:-}" in
        ""|"/"|"$HOME"|"$HOME/")
            echo -e "${RED}Refusing to modify unsafe extension path: '${EXT_DIR:-}'${NC}"
            exit 1
            ;;
        /*) ;;
        *)
            echo -e "${RED}Refusing to modify relative extension path: '${EXT_DIR:-}'${NC}"
            exit 1
            ;;
    esac
}

# sweep_stale_staging_dirs removes PID-suffixed staging/backup debris orphaned
# by previous interrupted runs (e.g. SIGKILL before the EXIT trap could fire).
# This run's own directories are always preserved.
sweep_stale_staging_dirs() {
    local stale=""
    for stale in "$INSTALL_DIR"/.extension-stage-* "$INSTALL_DIR"/.extension-backup-*; do
        [ -e "$stale" ] || continue
        [ "$stale" = "$STAGE_EXT_DIR" ] && continue
        [ "$stale" = "$BACKUP_EXT_DIR" ] && continue
        rm -rf "$stale" 2>/dev/null || true
    done
}

prepare_extension_stage() {
    rm -rf "$STAGE_EXT_DIR"
    mkdir -p "$STAGE_EXT_DIR"
}

validate_extension_stage() {
    local base_dir="${1:-$EXT_DIR}"
    [ -f "$base_dir/manifest.json" ] || return 1

    [ -f "$base_dir/background.js" ] &&
    [ -f "$base_dir/content.bundled.js" ] &&
    [ -f "$base_dir/inject.bundled.js" ] &&
    [ -f "$base_dir/early-patch.bundled.js" ]
}

promote_extension_stage() {
    # Guard before any destructive rm/mv that targets EXT_DIR.
    assert_safe_ext_dir

    if ! validate_extension_stage "$STAGE_EXT_DIR"; then
        echo -e "${RED}Extension staging failed: required module files are missing from staging.${NC}"
        exit 1
    fi

    rm -rf "$BACKUP_EXT_DIR"
    if [ -d "$EXT_DIR" ]; then
        mv "$EXT_DIR" "$BACKUP_EXT_DIR"
    fi

    mkdir -p "$(dirname "$EXT_DIR")"

    if ! mv "$STAGE_EXT_DIR" "$EXT_DIR"; then
        echo -e "${RED}Failed to promote staged extension directory.${NC}"
        if [ -d "$BACKUP_EXT_DIR" ]; then
            mv "$BACKUP_EXT_DIR" "$EXT_DIR" || true
        fi
        exit 1
    fi

    if ! validate_extension_stage "$EXT_DIR"; then
        echo -e "${RED}Promoted extension failed validation; restoring previous extension.${NC}"
        rm -rf "$EXT_DIR"
        if [ -d "$BACKUP_EXT_DIR" ]; then
            mv "$BACKUP_EXT_DIR" "$EXT_DIR" || true
        fi
        exit 1
    fi

    rm -rf "$BACKUP_EXT_DIR"
}

stage_extension_from_source_zip() {
    local source_zip_url="$1"
    local temp_extract="$TEMP_ROOT/ext_extract"

    rm -rf "$temp_extract"
    mkdir -p "$temp_extract"

    if ! curl_retry "$TEMP_ZIP" "$source_zip_url"; then
        return 1
    fi

    prepare_extension_stage
    if ! unzip -q "$TEMP_ZIP" -d "$temp_extract"; then
        return 1
    fi

    # The source zip root folder is typically 'repo-branch'.
    local extract_root
    extract_root=$(find "$temp_extract" -mindepth 1 -maxdepth 1 -type d | head -n 1)
    if [ -z "$extract_root" ] || [ ! -d "$extract_root/extension" ]; then
        return 1
    fi

    if ! cp -r "$extract_root/extension/." "$STAGE_EXT_DIR/"; then
        return 1
    fi
    if ! validate_extension_stage "$STAGE_EXT_DIR"; then
        return 1
    fi
    return 0
}

kill_stale_kaboom_processes() {
    # Kill any running Kaboom daemons before replacing the binary.
    # This avoids "text file busy" on Linux and ensures a clean upgrade.
    # Anchored to the canonical binary name. Keep this pattern in sync with
    # scripts/uninstall.sh.
    local proc_pattern='kaboom-(agentic-browser|hooks)|\.kaboom/bin/'
    local killed=0
    local pids=""

    if command -v pgrep >/dev/null 2>&1; then
        pids=$(pgrep -f "$proc_pattern" 2>/dev/null || true)
    elif command -v pkill >/dev/null 2>&1; then
        # pgrep not available but pkill is — just send TERM directly.
        pkill -f "$proc_pattern" 2>/dev/null || true
        sleep 0.5
        pkill -9 -f "$proc_pattern" 2>/dev/null || true
        return 0
    fi

    if [ -n "$pids" ]; then
        echo -e "  Stopping running Kaboom processes..."
        for pid in $pids; do
            # Don't kill ourselves.
            if [ "$pid" != "$$" ]; then
                kill "$pid" 2>/dev/null && killed=$((killed + 1)) || true
            fi
        done

        if [ $killed -gt 0 ]; then
            sleep 1
            # Force-kill any survivors.
            for pid in $pids; do
                if [ "$pid" != "$$" ] && kill -0 "$pid" 2>/dev/null; then
                    kill -9 "$pid" 2>/dev/null || true
                fi
            done
        fi
    fi
}

# ─────────────────────────────────────────────────────────────
# 1. Platform Detection
# ─────────────────────────────────────────────────────────────

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  darwin)  PLATFORM="darwin" ;; # macOS
  linux)   PLATFORM="linux" ;;  # Linux
  mingw*|cygwin*) PLATFORM="win32" ;; # Windows (Git Bash/Cygwin)
  *) echo -e "${RED}Unsupported OS: $OS${NC}"; exit 1 ;;
esac

# Normalize architecture strings to match release asset naming conventions (x64 vs arm64).
case "$ARCH" in
  x86_64|amd64) E_ARCH="x64" ;;
  arm64|aarch64) E_ARCH="arm64" ;;
  *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

# Windows-specific binary suffix and architecture enforcement.
if [ "$PLATFORM" == "win32" ]; then
    E_ARCH="x64"
    BINARY_EXT=".exe"
else
    BINARY_EXT=""
fi

# ─────────────────────────────────────────────────────────────
# 2. Version Check
# ─────────────────────────────────────────────────────────────

echo -e "Checking for updates..."
VERSION=$(curl -sSL --fail --max-time 15 "$VERSION_URL" | tr -d '[:space:]' || true)
if [ -z "$VERSION" ]; then
    echo -e "${RED}Failed to fetch latest version info from $VERSION_URL${NC}"
    echo -e "Check your network connection and try again."
    exit 1
fi

# ─────────────────────────────────────────────────────────────
# 3. Detect install vs upgrade
# ─────────────────────────────────────────────────────────────

CANONICAL_KABOOM_BIN="$BIN_DIR/kaboom-agentic-browser$BINARY_EXT"
KABOOM_HOOKS_BIN="$BIN_DIR/kaboom-hooks$BINARY_EXT"
IS_UPGRADE=0
PREVIOUS_VERSION=""

if [ "$HOOKS_ONLY" = "1" ]; then
    if [ -x "$KABOOM_HOOKS_BIN" ]; then
        PREVIOUS_VERSION=$("$KABOOM_HOOKS_BIN" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
        IS_UPGRADE=1
    fi
else
    if [ -x "$CANONICAL_KABOOM_BIN" ]; then
        PREVIOUS_VERSION=$("$CANONICAL_KABOOM_BIN" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
        IS_UPGRADE=1
    fi
fi

if [ "$IS_UPGRADE" = "1" ] && [ -n "$PREVIOUS_VERSION" ]; then
    echo -e "Upgrading: v$PREVIOUS_VERSION -> v$VERSION ($PLATFORM-$E_ARCH)"
else
    echo -e "Installing: v$VERSION ($PLATFORM-$E_ARCH)"
fi

# ─────────────────────────────────────────────────────────────
# 4. Directory Setup
# ─────────────────────────────────────────────────────────────

mkdir -p "$BIN_DIR"
mkdir -p "$INSTALL_DIR"

# Verify the directory is writable (catches permission issues early).
if ! touch "$INSTALL_DIR/.write-test" 2>/dev/null; then
    echo -e "${RED}Cannot write to $INSTALL_DIR — check directory permissions.${NC}"
    echo -e "If this was installed with sudo previously, run: sudo chown -R \$USER $INSTALL_DIR"
    exit 1
fi
rm -f "$INSTALL_DIR/.write-test"

# Remove staging/backup debris orphaned by previous interrupted runs.
sweep_stale_staging_dirs

echo -e "Install root: $INSTALL_DIR"

# ─────────────────────────────────────────────────────────────
# 5. Stop stale processes before binary replacement
# ─────────────────────────────────────────────────────────────

# Hooks-only installs don't run a daemon — no processes to stop.
if [ "$HOOKS_ONLY" != "1" ]; then
    kill_stale_kaboom_processes
fi

# ─────────────────────────────────────────────────────────────
# 6. Binary Installation
# ─────────────────────────────────────────────────────────────

CHECKSUM_URL="https://github.com/$REPO/releases/download/v$VERSION/checksums.txt"

# download_and_verify fetches a binary, validates size, verifies checksum, and installs it.
# Usage: download_and_verify <asset_name> <dest_path> <min_bytes> <label>
download_and_verify() {
    local asset_name="$1"
    local dest_path="$2"
    local min_bytes="$3"
    local label="$4"
    local dl_url="https://github.com/$REPO/releases/download/v$VERSION/$asset_name"
    local dl_path="$TEMP_ROOT/${asset_name}_dl"

    echo -e "Downloading ${label}..."
    if ! curl_retry "$dl_path" "$dl_url"; then
        echo -e "${RED}Download failed after 3 attempts.${NC}"
        echo -e "URL: $dl_url"
        echo -e "Check your network connection, proxy settings, or try again later."
        beacon_error "download_failed"
        exit 1
    fi

    # Validate binary size — catch truncated downloads and HTML error pages.
    local dl_size
    dl_size=$(wc -c < "$dl_path" | tr -d ' ')
    if [ "$dl_size" -lt "$min_bytes" ]; then
        echo -e "${RED}Downloaded file is too small (${dl_size} bytes, expected >${min_bytes}).${NC}"
        echo -e "The download may have been truncated or intercepted by a proxy."
        exit 1
    fi

    # Integrity Verification (SHA-256).
    if [ -f "$TEMP_ROOT/checksums.txt" ]; then
        # Exact field match — an unanchored grep could multi-match assets
        # whose names contain this asset's name as a substring.
        local expected_hash
        expected_hash=$(awk -v f="$asset_name" '$2 == f || $2 == "*" f {print $1; exit}' "$TEMP_ROOT/checksums.txt" || true)
        local actual_hash=""

        if [ -z "$expected_hash" ]; then
            if [ "$STRICT_CHECKSUM" = "1" ]; then
                echo -e "${RED}Strict checksum mode: checksums.txt missing entry for $asset_name.${NC}"
                exit 1
            fi
        elif command -v shasum >/dev/null 2>&1; then
            actual_hash=$(shasum -a 256 "$dl_path" | awk '{print $1}')
        elif command -v sha256sum >/dev/null 2>&1; then
            actual_hash=$(sha256sum "$dl_path" | awk '{print $1}')
        else
            if [ "$STRICT_CHECKSUM" = "1" ]; then
                echo -e "${RED}Strict checksum mode: no SHA-256 tool found.${NC}"
                exit 1
            fi
        fi

        if [ -n "${actual_hash:-}" ]; then
            if [ "$expected_hash" != "$actual_hash" ]; then
                echo -e "${RED}Checksum verification failed for ${label}!${NC}"
                echo -e "Expected: $expected_hash"
                echo -e "Actual:   $actual_hash"
                exit 1
            fi
            echo -e "${GREEN}  Checksum verified.${NC}"
        fi
    fi

    mv "$dl_path" "$dest_path"
    chmod 755 "$dest_path"

    # Quick smoke test.
    if ! "$dest_path" --version >/dev/null 2>&1; then
        echo -e "${RED}${label} smoke test failed — the binary cannot execute.${NC}"
        echo -e "Platform: $PLATFORM-$E_ARCH"
        exit 1
    fi
}

# Fetch checksums once for all binaries.
if curl -fsSL --max-time 15 "$CHECKSUM_URL" -o "$TEMP_ROOT/checksums.txt" 2>/dev/null; then
    :
else
    if [ "$STRICT_CHECKSUM" = "1" ]; then
        echo -e "${RED}Strict checksum mode: failed to download checksum manifest.${NC}"
        exit 1
    fi
    echo -e "${YELLOW}  Checksum verification skipped (could not fetch manifest).${NC}"
fi

# --- Install main binary (skip for --hooks-only) ---
if [ "$HOOKS_ONLY" != "1" ]; then
    BINARY_NAME="kaboom-agentic-browser-$PLATFORM-$E_ARCH$BINARY_EXT"
    download_and_verify "$BINARY_NAME" "$CANONICAL_KABOOM_BIN" "$MIN_BINARY_BYTES" "kaboom binary"

    # Stamp this install's epoch next to the binary so the daemon's single-instance
    # takeover has a deterministic tiebreaker at equal versions: the LATEST install
    # wins (see install_epoch.go). Without it, two same-version installs (e.g. this
    # one and an npm-global copy) thrash — a takeover war that crash-loops the daemon.
    # Unit is UnixNano to match the binary-mtime fallback; GNU date gives it via %N,
    # BSD/macOS date lacks %N so we fall back to whole seconds scaled to nanoseconds.
    INSTALL_EPOCH_NS="$(date +%s%N 2>/dev/null)"
    case "$INSTALL_EPOCH_NS" in
        ''|*[!0-9]*) INSTALL_EPOCH_NS="$(date +%s)000000000" ;;
    esac
    if printf '%s\n' "$INSTALL_EPOCH_NS" > "$BIN_DIR/.kaboom-install-epoch" 2>/dev/null; then
        echo -e "${GREEN}Install epoch stamped (latest install wins).${NC}"
    else
        echo -e "${YELLOW}  Could not write install epoch stamp; daemon will fall back to binary mtime.${NC}"
    fi
fi

# --- Always install hooks binary ---
HOOKS_BINARY_NAME="kaboom-hooks-$PLATFORM-$E_ARCH$BINARY_EXT"
download_and_verify "$HOOKS_BINARY_NAME" "$KABOOM_HOOKS_BIN" "$MIN_HOOKS_BINARY_BYTES" "kaboom-hooks binary"
echo -e "${GREEN}kaboom-hooks installed.${NC}"

if [ "$HOOKS_ONLY" = "1" ]; then
    echo -e "Skipping extension, daemon, and MCP config (hooks-only mode)."
else

echo -e "Refreshing browser extension..."
EXT_ZIP_NAME="kaboom-extension-v$VERSION.zip"
EXT_ZIP_URL="https://github.com/$REPO/releases/download/v$VERSION/$EXT_ZIP_NAME"
TEMP_ZIP="$TEMP_ROOT/extension.zip"

if curl_retry "$TEMP_ZIP" "$EXT_ZIP_URL"; then
    # Dedicated extension zip exists (faster); validate required module files after extract.
    prepare_extension_stage
    if unzip -q -o "$TEMP_ZIP" -d "$STAGE_EXT_DIR" && validate_extension_stage "$STAGE_EXT_DIR"; then
        promote_extension_stage
        echo -e "${GREEN}Extension staged: $EXT_DIR${NC}"
    else
        echo -e "${YELLOW}  Release extension zip missing required modules; falling back to source zip...${NC}"
        SOURCE_ZIP_URL="https://github.com/$REPO/archive/refs/heads/STABLE.zip"
        if stage_extension_from_source_zip "$SOURCE_ZIP_URL"; then
            promote_extension_stage
            echo -e "${GREEN}Extension staged: $EXT_DIR${NC}"
        else
            echo -e "${RED}Failed to download extension source archive.${NC}"
            exit 1
        fi
    fi
else
    # Fallback to source zip extraction (covers older releases and bad extension zip assets)
    SOURCE_ZIP_URL="https://github.com/$REPO/archive/refs/heads/STABLE.zip"
    if stage_extension_from_source_zip "$SOURCE_ZIP_URL"; then
        promote_extension_stage
        echo -e "${GREEN}Extension staged: $EXT_DIR${NC}"
    else
        echo -e "${RED}Failed to download extension after multiple attempts.${NC}"
        echo -e "URL: $EXT_ZIP_URL"
        exit 1
    fi
fi

# ─────────────────────────────────────────────────────────────
# 7b. Native Configuration (Go binary --install)
# ─────────────────────────────────────────────────────────────

echo -e "Finalizing configuration..."
if ! "$CANONICAL_KABOOM_BIN" --install; then
    echo -e "${YELLOW}Native configuration returned an error.${NC}"
    echo -e "The binary and extension were installed successfully."
    echo -e "You may need to manually configure your MCP clients."
    echo -e "Run: $CANONICAL_KABOOM_BIN --install"
    # Don't exit 1 here — the core install succeeded, only config auto-detection had issues.
fi

# ─────────────────────────────────────────────────────────────
# 8. Post-install health verification
# ─────────────────────────────────────────────────────────────

# Give the daemon a moment to start.
sleep 1
HEALTH_OK=0
HEALTH_RESPONSE=$(curl -sS --max-time 5 "http://127.0.0.1:7890/health" 2>/dev/null || true)
# Require kaboom identity, not just any responder with a "status" field —
# /health returns "name":"kaboom-browser-devtools" (handleHealth in
# cmd/browser-agent/server_routes_health_diagnostics.go).
if echo "$HEALTH_RESPONSE" | grep -q '"status"' 2>/dev/null && \
   echo "$HEALTH_RESPONSE" | grep -q 'kaboom-browser-devtools' 2>/dev/null; then
    HEALTH_OK=1
fi

if [ "$HEALTH_OK" = "1" ]; then
    echo -e "${GREEN}Server health check passed (port 7890).${NC}"
else
    echo -e "${YELLOW}  Server not yet responding on port 7890 — may still be starting.${NC}"
    echo -e "  Verify: curl http://127.0.0.1:7890/health"
fi

# ─────────────────────────────────────────────────────────────
# 9. Register start-on-login
# ─────────────────────────────────────────────────────────────

register_autostart() {
    if [ "$PLATFORM" = "darwin" ]; then
        local plist_dir="$HOME/Library/LaunchAgents"
        local plist_path="$plist_dir/com.kaboom.daemon.plist"
        mkdir -p "$plist_dir"

        cat > "$plist_path" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.kaboom.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>$CANONICAL_KABOOM_BIN</string>
        <string>--daemon</string>
        <string>--port</string>
        <string>7890</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/dev/null</string>
    <key>StandardErrorPath</key>
    <string>/dev/null</string>
</dict>
</plist>
PLIST

        # Unload previous registration (if any), then register fresh.
        launchctl bootout "gui/$(id -u)/com.kaboom.daemon" 2>/dev/null || true
        if launchctl bootstrap "gui/$(id -u)" "$plist_path" 2>/dev/null || \
           launchctl load "$plist_path" 2>/dev/null; then
            echo -e "${GREEN}Registered to start on login (LaunchAgent).${NC}"
        else
            echo -e "${YELLOW}  Could not register LaunchAgent automatically.${NC}"
            echo -e "  To start on login manually: launchctl load $plist_path"
        fi

    elif [ "$PLATFORM" = "linux" ]; then
        if command -v systemctl >/dev/null 2>&1 && systemctl --user status >/dev/null 2>&1; then
            local service_dir="$HOME/.config/systemd/user"
            local service_path="$service_dir/kaboom.service"
            mkdir -p "$service_dir"

            cat > "$service_path" <<SERVICE
[Unit]
Description=KaBOOM! Browser AI Devtools Daemon
After=network.target

[Service]
Type=simple
ExecStart=$CANONICAL_KABOOM_BIN --daemon --port 7890
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
SERVICE

            systemctl --user daemon-reload 2>/dev/null || true
            if systemctl --user enable kaboom.service 2>/dev/null; then
                echo -e "${GREEN}Registered to start on login (systemd user service).${NC}"
            else
                echo -e "${YELLOW}  Could not enable systemd service.${NC}"
                echo -e "  To start on login manually: systemctl --user enable kaboom.service"
            fi
        else
            # Fallback: XDG autostart for non-systemd desktops.
            local autostart_dir="$HOME/.config/autostart"
            local desktop_path="$autostart_dir/kaboom.desktop"
            mkdir -p "$autostart_dir"

            cat > "$desktop_path" <<DESKTOP
[Desktop Entry]
Type=Application
Name=KaBOOM! Daemon
Exec=$CANONICAL_KABOOM_BIN --daemon --port 7890
Hidden=false
NoDisplay=true
X-GNOME-Autostart-enabled=true
DESKTOP

            echo -e "${GREEN}Registered to start on login (XDG autostart).${NC}"
        fi
    fi
}

register_autostart

fi # end HOOKS_ONLY guard

# ─────────────────────────────────────────────────────────────
# 10. PATH registration
# ─────────────────────────────────────────────────────────────

register_path() {
    case ":${PATH}:" in
        *":$BIN_DIR:"*) return 0 ;;
    esac

    local shell_name=""
    shell_name=$(basename "${SHELL:-/bin/bash}")
    local rc_file=""
    case "$shell_name" in
        zsh)  rc_file="$HOME/.zshrc" ;;
        bash) rc_file="$HOME/.bashrc" ;;
        fish) rc_file="$HOME/.config/fish/config.fish" ;;
        *)    rc_file="$HOME/.profile" ;;
    esac

    # Check if already present in rc file (idempotent).
    if [ -f "$rc_file" ] && grep -qF "$BIN_DIR" "$rc_file" 2>/dev/null; then
        return 0
    fi

    local path_line=""
    if [ "$shell_name" = "fish" ]; then
        path_line="fish_add_path $BIN_DIR # kaboom"
    else
        path_line="export PATH=\"$BIN_DIR:\$PATH\" # kaboom"
    fi

    # The rc-file directory may not exist yet (e.g. fish: ~/.config/fish on a
    # machine where fish was never started) — create it or set -e aborts here.
    mkdir -p "$(dirname "$rc_file")"
    echo "" >> "$rc_file"
    echo "$path_line" >> "$rc_file"
    echo -e "${GREEN}Added $BIN_DIR to PATH in $rc_file${NC}"
    echo -e "Restart your terminal or run: ${BOLD}source $rc_file${NC}"
}

register_path

# ─────────────────────────────────────────────────────────────
# 13. Anonymous telemetry (disable: KABOOM_TELEMETRY=off)
# ─────────────────────────────────────────────────────────────

if [ "${KABOOM_TELEMETRY:-}" != "off" ]; then
    curl -s --max-time 2 -X POST "https://t.gokaboom.dev/v1/event" \
        -H "Content-Type: application/json" \
        -d "{\"event\":\"install_complete\",\"v\":\"${VERSION}\",\"os\":\"$(uname -s)-$(uname -m)\",\"props\":{\"method\":\"curl\"}}" \
        > /dev/null 2>&1 &
fi

# ─────────────────────────────────────────────────────────────
# 14. Final summary
# ─────────────────────────────────────────────────────────────

echo ""
if [ "$HOOKS_ONLY" = "1" ]; then
    if [ "$IS_UPGRADE" = "1" ] && [ -n "$PREVIOUS_VERSION" ]; then
        echo -e "${GREEN}${BOLD}kaboom-hooks upgraded: v$PREVIOUS_VERSION -> v$VERSION${NC}"
    else
        echo -e "${GREEN}${BOLD}kaboom-hooks v$VERSION installed successfully.${NC}"
    fi
    echo ""
    echo -e "Add quality gates to your Claude Code project:"
    echo -e "  kaboom-hooks quality-gate   (check code against project standards)"
    echo -e "  kaboom-hooks compress-output (compress verbose test/build output)"
    echo ""
    echo -e "Want the full KaBOOM! suite (browser devtools, MCP server, extension)?"
    echo -e "  curl -fsSL https://gokaboom.dev/install.sh | bash"
elif [ "$IS_UPGRADE" = "1" ] && [ -n "$PREVIOUS_VERSION" ]; then
    echo -e "${GREEN}${BOLD}KaBOOM! upgraded: v$PREVIOUS_VERSION -> v$VERSION${NC}"
else
    echo -e "${GREEN}${BOLD}KaBOOM! v$VERSION installed successfully.${NC}"
fi

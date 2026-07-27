#!/bin/bash
# Kaboom - Clean Uninstaller (counterpart to install.sh)
# https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP
#
# PURPOSE:
# Reverses every artifact created by scripts/install.sh and by
# `kaboom-agentic-browser --install`: binaries, extension files, autostart
# registrations, shell PATH lines, MCP client config entries, managed agent
# skills, and daemon runtime state. See
# docs/architecture/uninstall-and-cleanup.md for the artifact map.
#
# USAGE:
#   curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/uninstall.sh | bash -s -- --yes
#   ./scripts/uninstall.sh                # interactive (prompts before removing)
#   ./scripts/uninstall.sh --dry-run      # show what would be removed
#   ./scripts/uninstall.sh --keep-data    # keep logs/recordings/project state
#
# Windows users: use scripts/uninstall.ps1 instead.

set -euo pipefail

# ─────────────────────────────────────────────────────────────
# CLI flag parsing
# ─────────────────────────────────────────────────────────────

ASSUME_YES=0
DRY_RUN=0
KEEP_DATA=0
for arg in "$@"; do
    case "$arg" in
        --yes|-y)     ASSUME_YES=1 ;;
        --dry-run)    DRY_RUN=1 ;;
        --keep-data)  KEEP_DATA=1 ;;
        --help|-h)
            grep '^#' "$0" | head -20
            exit 0
            ;;
    esac
done

# Configuration: mirrors the single source of truth in install.sh.
INSTALL_DIR="$HOME/.kaboom"
BIN_DIR="$INSTALL_DIR/bin"
EXT_DIR="${KABOOM_EXTENSION_DIR:-$HOME/KaboomAgenticDevtoolExtension}"

# Canonical MCP server key plus every legacy key older installs may have
# written (must match installerLegacyServerKeys in internal/nativeinstall/installer.go).
SERVER_NAMES="kaboom-browser-devtools kaboom-agentic-browser kaboom gasoline-browser-devtools gasoline-agentic-browser gasoline strum-browser-devtools strum-agentic-browser strum"

# Managed skill files start with one of these markers (see lib/skills.js).
SKILL_MARKER_RE='<!-- (kaboom|gasoline|strum)-managed-skill'

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
ORANGE='\033[38;5;208m'
BOLD='\033[1m'
NC='\033[0m'

MANUAL_CONFIGS=""

# ─────────────────────────────────────────────────────────────
# Platform detection
# ─────────────────────────────────────────────────────────────

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    darwin) PLATFORM="darwin" ;;
    linux)  PLATFORM="linux" ;;
    mingw*|cygwin*)
        echo -e "${YELLOW}On Windows, run scripts/uninstall.ps1 from PowerShell instead.${NC}"
        exit 1
        ;;
    *) PLATFORM="other" ;;
esac

# ─────────────────────────────────────────────────────────────
# Helpers
# ─────────────────────────────────────────────────────────────

# run executes a mutation, or skips it entirely in dry-run mode.
run() {
    if [ "$DRY_RUN" = "1" ]; then
        return 0
    fi
    "$@"
}

log_removed() {
    if [ "$DRY_RUN" = "1" ]; then
        echo -e "  [dry-run] Would remove: $1"
    else
        echo -e "  Removed: $1"
    fi
}

# safe_rm_rf refuses obviously catastrophic targets before deleting.
safe_rm_rf() {
    local target="${1:-}"
    case "$target" in
        ""|"/"|"$HOME"|"$HOME/")
            echo -e "  ${YELLOW}Refusing to remove unsafe path: '$target'${NC}"
            return 0
            ;;
        /*) ;;
        *)
            echo -e "  ${YELLOW}Refusing to remove relative path: '$target'${NC}"
            return 0
            ;;
    esac
    run rm -rf -- "$target"
}

remove_path() {
    local target="$1"
    if [ -e "$target" ] || [ -L "$target" ]; then
        safe_rm_rf "$target"
        log_removed "$target"
    fi
}

# ─────────────────────────────────────────────────────────────
# Banner + confirmation
# ─────────────────────────────────────────────────────────────

echo -e "${ORANGE}${BOLD}"
cat <<'EOF'
  _  __     ____   ___   ___  __  __ _
 | |/ /__ _| __ ) / _ \ / _ \|  \/  | |
 | ' // _` |  _ \| | | | | | | |\/| | |
 | . \ (_| | |_) | |_| | |_| | |  | |_|
 |_|\_\__,_|____/ \___/ \___/|_|  |_(_)
EOF
echo -e "${NC}"
echo -e "${ORANGE}${BOLD}KaBOOM! Uninstaller${NC}"
echo -e "${BLUE}--------------------------------------------------${NC}"
echo -e "This will remove:"
echo -e "  - Binaries and state in $INSTALL_DIR"
echo -e "  - Browser extension files in $EXT_DIR"
echo -e "  - Start-on-login registration (LaunchAgent / systemd / XDG autostart)"
echo -e "  - Kaboom entries in MCP client configs (Claude, Cursor, Zed, ...)"
echo -e "  - Kaboom PATH lines from shell rc files"
echo -e "  - Kaboom-managed agent skills (~/.claude, ~/.codex, ~/.gemini)"
if [ "$KEEP_DATA" = "1" ]; then
    echo -e "  ${GREEN}(logs, recordings, and project data will be kept: --keep-data)${NC}"
fi
if [ "$DRY_RUN" = "1" ]; then
    echo -e "${YELLOW}Dry run: nothing will actually be removed.${NC}"
fi
echo ""

if [ "$DRY_RUN" != "1" ] && [ "$ASSUME_YES" != "1" ]; then
    if [ -t 0 ]; then
        printf "Remove all Kaboom components? [y/N] "
        read -r reply
        case "$reply" in
            y|Y|yes|YES) ;;
            *) echo "Aborted. Nothing was removed."; exit 1 ;;
        esac
    else
        echo -e "${RED}Non-interactive shell detected and --yes was not passed.${NC}"
        echo -e "Re-run with explicit consent:"
        echo -e "  curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/uninstall.sh | bash -s -- --yes"
        exit 1
    fi
fi

# Capture the version for the telemetry beacon before the binary is deleted.
VERSION="unknown"
if [ -x "$BIN_DIR/kaboom-agentic-browser" ]; then
    VERSION=$("$BIN_DIR/kaboom-agentic-browser" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
    [ -n "$VERSION" ] || VERSION="unknown"
fi

# ─────────────────────────────────────────────────────────────
# 1. Stop running daemons
# ─────────────────────────────────────────────────────────────

stop_kaboom_processes() {
    echo -e "${BLUE}Stopping Kaboom processes...${NC}"
    if [ "$DRY_RUN" = "1" ]; then
        echo -e "  [dry-run] Would stop running Kaboom daemons."
        return 0
    fi
    local pids=""
    if command -v pgrep >/dev/null 2>&1; then
        # Anchored to full binary names — never bare substrings like 'strum',
        # which would match unrelated processes (e.g. 'instrument').
        pids=$(pgrep -f '(kaboom|gasoline|strum)-(agentic-browser|agentic-devtools|hooks)|\.kaboom/bin/' 2>/dev/null || true)
    fi
    [ -n "$pids" ] || return 0
    local pid
    for pid in $pids; do
        [ "$pid" != "$$" ] || continue
        kill "$pid" 2>/dev/null || true
    done
    sleep 1
    for pid in $pids; do
        [ "$pid" != "$$" ] || continue
        if kill -0 "$pid" 2>/dev/null; then
            kill -9 "$pid" 2>/dev/null || true
        fi
    done
    echo -e "  Stopped."
}

stop_kaboom_processes

# ─────────────────────────────────────────────────────────────
# 2. Unregister start-on-login
# ─────────────────────────────────────────────────────────────

echo -e "${BLUE}Removing start-on-login registration...${NC}"
if [ "$PLATFORM" = "darwin" ]; then
    PLIST="$HOME/Library/LaunchAgents/com.kaboom.daemon.plist"
    if [ -f "$PLIST" ]; then
        run launchctl bootout "gui/$(id -u)/com.kaboom.daemon" 2>/dev/null || true
        remove_path "$PLIST"
    fi
elif [ "$PLATFORM" = "linux" ]; then
    UNIT="$HOME/.config/systemd/user/kaboom.service"
    if [ -f "$UNIT" ]; then
        if command -v systemctl >/dev/null 2>&1; then
            run systemctl --user disable --now kaboom.service 2>/dev/null || true
        fi
        remove_path "$UNIT"
        remove_path "$HOME/.config/systemd/user/default.target.wants/kaboom.service"
        if command -v systemctl >/dev/null 2>&1; then
            run systemctl --user daemon-reload 2>/dev/null || true
        fi
    fi
    remove_path "$HOME/.config/autostart/kaboom.desktop"
fi

# ─────────────────────────────────────────────────────────────
# 3. Remove MCP client config entries
# ─────────────────────────────────────────────────────────────

JSON_TOOL=""
if command -v python3 >/dev/null 2>&1; then
    JSON_TOOL="python3"
elif command -v node >/dev/null 2>&1; then
    JSON_TOOL="node"
fi

# strip_mcp_entries removes Kaboom server keys from one client config file.
# Files are edited in place (with a .kaboom-uninstall.bak backup) and NEVER
# deleted — Zed/Gemini/OpenCode configs hold unrelated user settings.
strip_mcp_entries() {
    local file="$1" key="$2"
    [ -f "$file" ] || return 0
    grep -qE 'kaboom|gasoline|strum' "$file" 2>/dev/null || return 0
    if [ "$DRY_RUN" = "1" ]; then
        echo -e "  [dry-run] Would remove Kaboom MCP entries from: $file"
        return 0
    fi
    if [ -z "$JSON_TOOL" ]; then
        MANUAL_CONFIGS="${MANUAL_CONFIGS}    ${file}\n"
        return 0
    fi
    local status=0
    if [ "$JSON_TOOL" = "python3" ]; then
        # shellcheck disable=SC2086 # SERVER_NAMES is intentionally word-split
        python3 - "$file" "$key" $SERVER_NAMES <<'PYEOF' || status=$?
import json, shutil, sys
path, key = sys.argv[1], sys.argv[2]
names = sys.argv[3:]
try:
    with open(path) as fh:
        data = json.load(fh)
except Exception:
    sys.exit(3)
servers = data.get(key)
if not isinstance(servers, dict):
    sys.exit(0)
removed = [n for n in names if n in servers]
if not removed:
    sys.exit(0)
shutil.copyfile(path, path + ".kaboom-uninstall.bak")
for n in removed:
    del servers[n]
with open(path, "w") as fh:
    json.dump(data, fh, indent=2)
    fh.write("\n")
sys.exit(10)
PYEOF
    else
        # shellcheck disable=SC2086
        node -e '
const fs = require("fs");
const [file, key, ...names] = process.argv.slice(1);
let data;
try { data = JSON.parse(fs.readFileSync(file, "utf8")); } catch { process.exit(3); }
const servers = data[key];
if (!servers || typeof servers !== "object") process.exit(0);
const removed = names.filter((n) => Object.prototype.hasOwnProperty.call(servers, n));
if (!removed.length) process.exit(0);
fs.copyFileSync(file, file + ".kaboom-uninstall.bak");
for (const n of removed) delete servers[n];
fs.writeFileSync(file, JSON.stringify(data, null, 2) + "\n");
process.exit(10);
' "$file" "$key" $SERVER_NAMES || status=$?
    fi
    case "$status" in
        10) echo -e "  ${GREEN}Removed Kaboom entries:${NC} $file (backup: ${file}.kaboom-uninstall.bak)" ;;
        3)  echo -e "  ${YELLOW}Could not parse $file — remove Kaboom entries manually.${NC}" ;;
        0)  ;;
        *)  echo -e "  ${YELLOW}Failed to update $file — remove Kaboom entries manually.${NC}" ;;
    esac
}

echo -e "${BLUE}Removing MCP client configurations...${NC}"
if command -v claude >/dev/null 2>&1; then
    if [ "$DRY_RUN" = "1" ]; then
        echo -e "  [dry-run] Would run: claude mcp remove --scope user <each kaboom server name>"
    else
        for name in $SERVER_NAMES; do
            CLAUDECODE= claude mcp remove --scope user "$name" >/dev/null 2>&1 || true
        done
        echo -e "  Claude Code entries removed (claude CLI)."
    fi
fi
strip_mcp_entries "$HOME/.cursor/mcp.json" mcpServers
strip_mcp_entries "$HOME/.codeium/windsurf/mcp_config.json" mcpServers
strip_mcp_entries "$HOME/.gemini/settings.json" mcpServers
strip_mcp_entries "$HOME/.gemini/antigravity/mcp_config.json" mcpServers
strip_mcp_entries "$HOME/.config/opencode/opencode.json" mcp
strip_mcp_entries "$HOME/.config/zed/settings.json" context_servers
if [ "$PLATFORM" = "darwin" ]; then
    strip_mcp_entries "$HOME/Library/Application Support/Claude/claude_desktop_config.json" mcpServers
    # VS Code mcp.json uses the "servers" key; older installs wrote "mcpServers".
    strip_mcp_entries "$HOME/Library/Application Support/Code/User/mcp.json" servers
    strip_mcp_entries "$HOME/Library/Application Support/Code/User/mcp.json" mcpServers
elif [ "$PLATFORM" = "linux" ]; then
    strip_mcp_entries "$HOME/.config/Code/User/mcp.json" servers
    strip_mcp_entries "$HOME/.config/Code/User/mcp.json" mcpServers
fi

# ─────────────────────────────────────────────────────────────
# 4. Remove managed agent skills
# ─────────────────────────────────────────────────────────────

remove_managed_skills() {
    local root="$1"
    [ -d "$root" ] || return 0
    # The dedicated kaboom skill folder (claude_skill/install.sh) is always ours.
    if [ -d "$root/kaboom" ]; then
        remove_path "$root/kaboom"
    fi
    local entry
    # Directory-per-skill layout (Codex): <root>/<id>/SKILL.md
    for entry in "$root"/*/; do
        [ -d "$entry" ] || continue
        if [ -f "${entry}SKILL.md" ] && head -3 "${entry}SKILL.md" 2>/dev/null | grep -qE "$SKILL_MARKER_RE"; then
            remove_path "${entry%/}"
        fi
    done
    # Flat-file layout (Claude/Gemini): <root>/<id>.md
    for entry in "$root"/*.md; do
        [ -f "$entry" ] || continue
        if head -3 "$entry" 2>/dev/null | grep -qE "$SKILL_MARKER_RE"; then
            remove_path "$entry"
        fi
    done
}

echo -e "${BLUE}Removing Kaboom-managed agent skills...${NC}"
remove_managed_skills "${KABOOM_CLAUDE_SKILLS_DIR:-$HOME/.claude/skills}"
remove_managed_skills "${CODEX_HOME:-$HOME/.codex}/skills"
remove_managed_skills "${GEMINI_HOME:-$HOME/.gemini}/skills"

# ─────────────────────────────────────────────────────────────
# 5. Remove PATH registration from shell rc files
# ─────────────────────────────────────────────────────────────

# install.sh marks its PATH lines with a trailing '# kaboom'. A user may have
# switched shells between installs, so every candidate rc file is cleaned.
clean_path_lines() {
    local rc="$1"
    [ -f "$rc" ] || return 0
    grep -q '# kaboom$' "$rc" 2>/dev/null || return 0
    if [ "$DRY_RUN" = "1" ]; then
        echo -e "  [dry-run] Would remove Kaboom PATH line from: $rc"
        return 0
    fi
    local tmp
    tmp=$(mktemp)
    awk '!/# kaboom$/' "$rc" > "$tmp" && cat "$tmp" > "$rc"
    rm -f "$tmp"
    echo -e "  Removed PATH entry from: $rc"
}

echo -e "${BLUE}Cleaning shell PATH entries...${NC}"
clean_path_lines "$HOME/.zshrc"
clean_path_lines "$HOME/.bashrc"
clean_path_lines "$HOME/.profile"
clean_path_lines "$HOME/.config/fish/config.fish"

# ─────────────────────────────────────────────────────────────
# 6. Remove extension, binaries, and state
# ─────────────────────────────────────────────────────────────

echo -e "${BLUE}Removing Kaboom files...${NC}"

remove_path "$EXT_DIR"

# remove_state_root clears a runtime state root (binaries always; data
# only without --keep-data). Roots: ~/.kaboom plus KABOOM_STATE_DIR /
# XDG_STATE_HOME overrides honored by the daemon (internal/state/paths.go).
remove_state_root() {
    local root="$1"
    [ -n "$root" ] || return 0
    [ -d "$root" ] || return 0
    if [ "$KEEP_DATA" = "1" ]; then
        remove_path "$root/bin"
        remove_path "$root/run"
        echo -e "  ${GREEN}Kept data in $root (--keep-data)${NC}"
    else
        remove_path "$root"
    fi
}

remove_state_root "$INSTALL_DIR"
remove_state_root "${KABOOM_STATE_DIR:-}"
if [ -n "${XDG_STATE_HOME:-}" ]; then
    remove_state_root "$XDG_STATE_HOME/kaboom"
fi

if [ "$KEEP_DATA" != "1" ]; then
    # Legacy/runtime artifacts from older versions and daemon defaults.
    remove_path "$HOME/kaboom-upload-dir"
    remove_path "$HOME/kaboom-logs.jsonl"
    remove_path "$HOME/kaboom-crash.log"
    remove_path "$HOME/.kaboom-settings.json"
    for pidfile in "$HOME"/.kaboom-*.pid "$HOME"/.gasoline-*.pid "$HOME"/.strum-*.pid; do
        [ -e "$pidfile" ] || continue
        remove_path "$pidfile"
    done
    remove_path "$HOME/.gasoline"
    remove_path "$HOME/.strum"
    if [ "$PLATFORM" = "darwin" ]; then
        remove_path "$HOME/Library/Application Support/kaboom"
    else
        remove_path "${XDG_CONFIG_HOME:-$HOME/.config}/kaboom"
    fi
fi

# ─────────────────────────────────────────────────────────────
# 7. Anonymous telemetry (disable: KABOOM_TELEMETRY=off)
# ─────────────────────────────────────────────────────────────

if [ "$DRY_RUN" != "1" ] && [ "${KABOOM_TELEMETRY:-}" != "off" ]; then
    curl -s --max-time 2 -X POST "https://t.gokaboom.dev/v1/event" \
        -H "Content-Type: application/json" \
        -d "{\"event\":\"uninstall_complete\",\"v\":\"${VERSION}\",\"os\":\"$(uname -s)-$(uname -m)\",\"props\":{\"method\":\"curl\"}}" \
        > /dev/null 2>&1 || true
fi

# ─────────────────────────────────────────────────────────────
# 8. Final summary
# ─────────────────────────────────────────────────────────────

echo ""
if [ "$DRY_RUN" = "1" ]; then
    echo -e "${GREEN}${BOLD}Dry run complete — nothing was removed.${NC}"
else
    echo -e "${GREEN}${BOLD}KaBOOM! has been uninstalled.${NC}"
fi
if [ -n "$MANUAL_CONFIGS" ]; then
    echo ""
    echo -e "${YELLOW}python3/node not found — remove Kaboom entries manually from:${NC}"
    echo -e "$MANUAL_CONFIGS"
fi
echo ""
echo -e "${BOLD}Manual steps that cannot be automated:${NC}"
echo -e "  1) Open chrome://extensions (or brave://extensions) and Remove the Kaboom extension."
echo -e "  2) Restart your terminal so PATH changes take effect."
if [ "$DRY_RUN" != "1" ]; then
    echo -e "  3) Edited MCP configs were backed up as *.kaboom-uninstall.bak — delete them once verified."
fi
echo ""
echo -e "Changed your mind? Reinstall any time:"
echo -e "  curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash"

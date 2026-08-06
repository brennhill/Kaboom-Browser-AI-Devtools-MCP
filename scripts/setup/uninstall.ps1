# Kaboom - Clean Uninstaller for Windows (PowerShell, counterpart to install.ps1)
# https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP
#
# PURPOSE:
# Reverses every artifact created by scripts/setup/install.ps1 and by
# `kaboom-agentic-browser.exe --install`: binaries, extension files, MCP
# client config entries, managed agent skills, and daemon runtime state.
# install.ps1 creates no registry keys, PATH entries, scheduled tasks, or
# startup shortcuts on Windows, so none need to be reverted here.
#
# USAGE:
#   irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/setup/uninstall.ps1 -OutFile uninstall.ps1; ./uninstall.ps1 -Yes
#   ./scripts/setup/uninstall.ps1            # interactive (prompts before removing)
#   ./scripts/setup/uninstall.ps1 -DryRun    # show what would be removed
#   ./scripts/setup/uninstall.ps1 -KeepData  # keep logs/recordings/project state

param(
    [switch]$Yes,
    [switch]$DryRun,
    [switch]$KeepData
)

$ErrorActionPreference = "Stop"

# Configuration: mirrors install.ps1.
$INSTALL_DIR = Join-Path $HOME ".kaboom"
$BIN_DIR = Join-Path $INSTALL_DIR "bin"
$EXT_DIR = if ($env:KABOOM_EXTENSION_DIR) { $env:KABOOM_EXTENSION_DIR } else { Join-Path $HOME "KaboomAgenticDevtoolExtension" }
$APPDATA_DIR = if ($env:APPDATA) { $env:APPDATA } else { Join-Path $HOME "AppData\Roaming" }

$SERVER_NAMES = @('kaboom-browser-devtools')

# Managed skill files start with one of these markers (see lib/installation/skills.js).
$SKILL_MARKER = 'kaboom-managed-skill'

Write-Host ""
Write-Host "KaBOOM! Uninstaller (Windows)" -ForegroundColor Yellow
Write-Host "--------------------------------------------------" -ForegroundColor Blue
Write-Host "This will remove:"
Write-Host "  - Binaries and state in $INSTALL_DIR"
Write-Host "  - Browser extension files in $EXT_DIR"
Write-Host "  - Kaboom entries in MCP client configs (Claude, Cursor, Zed, ...)"
Write-Host "  - Kaboom-managed agent skills (~\.claude, ~\.codex, ~\.gemini)"
if ($KeepData) { Write-Host "  (logs, recordings, and project data will be kept: -KeepData)" -ForegroundColor Green }
if ($DryRun) { Write-Host "Dry run: nothing will actually be removed." -ForegroundColor Yellow }
Write-Host ""

if (-not $DryRun -and -not $Yes) {
    $reply = Read-Host "Remove all Kaboom components? [y/N]"
    if ($reply -notmatch '^(y|yes)$') {
        Write-Host "Aborted. Nothing was removed."
        exit 1
    }
}

function Remove-Target {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return }
    if (-not (Test-Path $Path)) { return }
    if ($DryRun) {
        Write-Host "  [dry-run] Would remove: $Path"
        return
    }
    Remove-Item -Path $Path -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "  Removed: $Path"
}

# ─────────────────────────────────────────────────────────────
# 1. Stop running daemons (match full binary names only)
# ─────────────────────────────────────────────────────────────

Write-Host "Stopping Kaboom processes..." -ForegroundColor Blue
if (-not $DryRun) {
    $patterns = 'kaboom-agentic-browser|kaboom-hooks'
    Get-Process -ErrorAction SilentlyContinue | Where-Object {
        $_.ProcessName -match $patterns
    } | ForEach-Object {
        Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
        Write-Host "  Stopped process: $($_.ProcessName) (PID $($_.Id))"
    }
} else {
    Write-Host "  [dry-run] Would stop running Kaboom daemons."
}

# ─────────────────────────────────────────────────────────────
# 2. Remove MCP client config entries
# ─────────────────────────────────────────────────────────────

# Edits configs in place (with a .kaboom-uninstall.bak backup) and NEVER
# deletes them — Zed/Gemini/OpenCode configs hold unrelated user settings.
function Remove-McpEntries {
    param([string]$Path, [string]$Key)
    if (-not (Test-Path $Path)) { return }
    $raw = Get-Content -Path $Path -Raw -ErrorAction SilentlyContinue
    if (-not $raw -or $raw -notmatch 'kaboom-browser-devtools') { return }
    if ($DryRun) {
        Write-Host "  [dry-run] Would remove Kaboom MCP entries from: $Path"
        return
    }
    try {
        $data = $raw | ConvertFrom-Json
    } catch {
        Write-Host "  Could not parse $Path - remove Kaboom entries manually." -ForegroundColor Yellow
        return
    }
    $servers = $data.$Key
    if (-not $servers) { return }
    $removed = @()
    foreach ($name in $SERVER_NAMES) {
        if ($servers.PSObject.Properties.Name -contains $name) {
            $servers.PSObject.Properties.Remove($name)
            $removed += $name
        }
    }
    if ($removed.Count -eq 0) { return }
    Copy-Item -Path $Path -Destination "$Path.kaboom-uninstall.bak" -Force
    $data | ConvertTo-Json -Depth 32 | Set-Content -Path $Path -Encoding UTF8
    Write-Host "  Removed Kaboom entries ($($removed -join ', ')) from: $Path" -ForegroundColor Green
}

Write-Host "Removing MCP client configurations..." -ForegroundColor Blue
if (Get-Command claude -ErrorAction SilentlyContinue) {
    if ($DryRun) {
        Write-Host "  [dry-run] Would run: claude mcp remove --scope user <each kaboom server name>"
    } else {
        foreach ($name in $SERVER_NAMES) {
            & claude mcp remove --scope user $name 2>$null | Out-Null
        }
        Write-Host "  Claude Code entries removed (claude CLI)."
    }
}
Remove-McpEntries -Path (Join-Path $HOME ".cursor\mcp.json") -Key "mcpServers"
Remove-McpEntries -Path (Join-Path $HOME ".codeium\windsurf\mcp_config.json") -Key "mcpServers"
Remove-McpEntries -Path (Join-Path $HOME ".gemini\settings.json") -Key "mcpServers"
Remove-McpEntries -Path (Join-Path $HOME ".gemini\antigravity\mcp_config.json") -Key "mcpServers"
Remove-McpEntries -Path (Join-Path $HOME ".config\opencode\opencode.json") -Key "mcp"
Remove-McpEntries -Path (Join-Path $HOME ".config\zed\settings.json") -Key "context_servers"
Remove-McpEntries -Path (Join-Path $APPDATA_DIR "Claude\claude_desktop_config.json") -Key "mcpServers"
# VS Code mcp.json uses the "servers" key.
Remove-McpEntries -Path (Join-Path $APPDATA_DIR "Code\User\mcp.json") -Key "servers"

# ─────────────────────────────────────────────────────────────
# 3. Remove managed agent skills
# ─────────────────────────────────────────────────────────────

function Remove-ManagedSkills {
    param([string]$Root)
    if (-not (Test-Path $Root)) { return }
    # The dedicated kaboom skill folder is always ours.
    Remove-Target (Join-Path $Root "kaboom")
    # Directory-per-skill layout (Codex): <root>/<id>/SKILL.md
    Get-ChildItem -Path $Root -Directory -ErrorAction SilentlyContinue | ForEach-Object {
        $skillFile = Join-Path $_.FullName "SKILL.md"
        if (Test-Path $skillFile) {
            $head = Get-Content -Path $skillFile -TotalCount 3 -ErrorAction SilentlyContinue
            if ($head -match $SKILL_MARKER) { Remove-Target $_.FullName }
        }
    }
    # Flat-file layout (Claude/Gemini): <root>/<id>.md
    Get-ChildItem -Path $Root -Filter "*.md" -File -ErrorAction SilentlyContinue | ForEach-Object {
        $head = Get-Content -Path $_.FullName -TotalCount 3 -ErrorAction SilentlyContinue
        if ($head -match $SKILL_MARKER) { Remove-Target $_.FullName }
    }
}

Write-Host "Removing Kaboom-managed agent skills..." -ForegroundColor Blue
Remove-ManagedSkills -Root (Join-Path $HOME ".claude\skills")
Remove-ManagedSkills -Root (Join-Path $HOME ".codex\skills")
Remove-ManagedSkills -Root (Join-Path $HOME ".gemini\skills")

# ─────────────────────────────────────────────────────────────
# 4. Remove extension, binaries, and state
# ─────────────────────────────────────────────────────────────

Write-Host "Removing Kaboom files..." -ForegroundColor Blue
Remove-Target $EXT_DIR
if ($KeepData) {
    Remove-Target $BIN_DIR
    Remove-Target (Join-Path $INSTALL_DIR "run")
    Write-Host "  Kept data in $INSTALL_DIR (-KeepData)" -ForegroundColor Green
} else {
    Remove-Target $INSTALL_DIR
}

# ─────────────────────────────────────────────────────────────
# 5. Final summary
# ─────────────────────────────────────────────────────────────

Write-Host ""
if ($DryRun) {
    Write-Host "Dry run complete - nothing was removed." -ForegroundColor Green
} else {
    Write-Host "KaBOOM! has been uninstalled." -ForegroundColor Green
}
Write-Host ""
Write-Host "Manual steps that cannot be automated:"
Write-Host "  1) Open chrome://extensions (or brave://extensions) and Remove the Kaboom extension."
if (-not $DryRun) {
    Write-Host "  2) Edited MCP configs were backed up as *.kaboom-uninstall.bak - delete them once verified."
}
Write-Host ""
Write-Host "Changed your mind? Reinstall any time:"
Write-Host "  irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/setup/install.ps1 | iex"

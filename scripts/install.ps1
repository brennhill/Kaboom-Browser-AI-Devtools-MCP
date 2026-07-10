# Kaboom - Ultimate Windows Installer (PowerShell)
# https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP
#
# PURPOSE:
# This PowerShell script provides a native, one-liner installation for Windows users.
# It avoids external dependencies like bash/curl by using built-in .NET/PowerShell features.
#
# USAGE:
#   irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.ps1 | iex

# Stop the script if any command results in an error. Equivalent to 'set -e'.
$ErrorActionPreference = "Stop"

# Configuration: Single source of truth for repository and local paths.
$REPO = "brennhill/Kaboom-Browser-AI-Devtools-MCP"
$INSTALL_DIR = Join-Path $HOME ".kaboom"
$BIN_DIR = Join-Path $INSTALL_DIR "bin"
$EXT_DIR = if ($env:KABOOM_EXTENSION_DIR) { $env:KABOOM_EXTENSION_DIR } else { Join-Path $HOME "KaboomAgenticDevtoolExtension" }
$KABOOM_BIN = Join-Path $BIN_DIR "kaboom-agentic-browser.exe"
# Release version source of truth.
$VERSION_URL = "https://raw.githubusercontent.com/$REPO/STABLE/VERSION"
# Checksums are verified by DEFAULT (every real release ships checksums.txt). An
# unverifiable download aborts, turning an HTML/404 body into a hard error instead of a
# silently-broken install. Opt out only for offline/mirror installs: KABOOM_INSTALL_STRICT=0.
$STRICT_CHECKSUM = $env:KABOOM_INSTALL_STRICT -ne "0"
$TEMP_TOKEN = [Guid]::NewGuid().ToString("N")
$STAGE_EXT_DIR = Join-Path $INSTALL_DIR ".extension-stage-$TEMP_TOKEN"
$BACKUP_EXT_DIR = Join-Path $INSTALL_DIR ".extension-backup-$TEMP_TOKEN"
$INSTALL_WARNINGS = New-Object System.Collections.Generic.List[string]
$script:WARNINGS_PRINTED = $false

function Add-InstallWarning {
    param([string]$Message)
    if (-not [string]::IsNullOrWhiteSpace($Message)) {
        [void]$INSTALL_WARNINGS.Add($Message)
    }
}

function Show-InstallWarnings {
    if ($script:WARNINGS_PRINTED -or $INSTALL_WARNINGS.Count -eq 0) {
        return
    }
    $script:WARNINGS_PRINTED = $true

    Write-Host ""
    Write-Host "============================================================" -ForegroundColor Red
    Write-Host "🚨 INSTALL WARNING: MANUAL ACTION REQUIRED" -ForegroundColor Red
    foreach ($warning in $INSTALL_WARNINGS) {
        Write-Host " - $warning" -ForegroundColor Yellow
    }
    Write-Host ""
    Write-Host "The old server may still be running. Kill it manually:" -ForegroundColor Red
    Write-Host "  Get-Process kaboom-agentic-browser -ErrorAction SilentlyContinue | Stop-Process -Force" -ForegroundColor Yellow
    Write-Host "  taskkill /F /IM kaboom-agentic-browser.exe /T" -ForegroundColor Yellow
    Write-Host "  Remove-Item `"$KABOOM_BIN`" -Force" -ForegroundColor Yellow
    Write-Host "Then re-run installer:" -ForegroundColor Red
    Write-Host "  irm https://raw.githubusercontent.com/$REPO/STABLE/scripts/install.ps1 | iex" -ForegroundColor Yellow
    Write-Host "============================================================" -ForegroundColor Red
}

function Get-KaboomServerPids {
    $pids = @()
    $targetPath = [System.IO.Path]::GetFullPath($KABOOM_BIN).ToLowerInvariant()
    $processes = Get-CimInstance Win32_Process -Filter "Name = 'kaboom-agentic-browser.exe' OR Name = 'kaboom.exe' OR Name = 'gasoline.exe' OR Name = 'strum.exe'" -ErrorAction SilentlyContinue

    foreach ($proc in $processes) {
        if (-not $proc.ProcessId) { continue }

        if ([string]::IsNullOrWhiteSpace($proc.ExecutablePath)) {
            # No path info: still kill to avoid stale lock survivors.
            $pids += [int]$proc.ProcessId
            continue
        }

        try {
            $procPath = [System.IO.Path]::GetFullPath($proc.ExecutablePath).ToLowerInvariant()
            # Catch Kaboom plus legacy Gasoline and STRUM processes.
            if ($procPath -eq $targetPath -or $procPath -match "kaboom\.exe$" -or $procPath -match "gasoline\.exe$" -or $procPath -match "strum\.exe$") {
                $pids += [int]$proc.ProcessId
            }
        } catch {
            $pids += [int]$proc.ProcessId
        }
    }

    return @($pids | Sort-Object -Unique)
}

function Stop-KaboomServerProcesses {
    $targetPids = @(Get-KaboomServerPids)
    if ($targetPids.Count -eq 0) {
        return $true
    }

    Write-Host "🛑 Stopping running Kaboom/legacy server: PID(s) $($targetPids -join ', ')"
    foreach ($procId in $targetPids) {
        Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
    }

    Start-Sleep -Milliseconds 350
    $remaining = @(Get-KaboomServerPids)
    if ($remaining.Count -eq 0) {
        return $true
    }

    Write-Host "⚠️  Escalating termination with taskkill..." -ForegroundColor Yellow
    foreach ($procId in $remaining) {
        & taskkill /F /PID $procId /T *> $null
    }

    Start-Sleep -Milliseconds 500
    $remaining = @(Get-KaboomServerPids)
    if ($remaining.Count -eq 0) {
        return $true
    }

    Add-InstallWarning "Old server is still running after forced stop attempt (PID(s): $($remaining -join ', '))."
    return $false
}

function Replace-KaboomBinary {
    param(
        [string]$StagePath,
        [string]$LivePath
    )

    $maxAttempts = 4
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        [void](Stop-KaboomServerProcesses)
        try {
            if (Test-Path $LivePath) {
                Remove-Item -Path $LivePath -Force -ErrorAction Stop
            }
            Move-Item -Path $StagePath -Destination $LivePath -Force -ErrorAction Stop
            return $true
        } catch {
            if ($attempt -lt $maxAttempts) {
                Write-Host "⚠️  Binary replace attempt $attempt/$maxAttempts failed; retrying..." -ForegroundColor Yellow
                Start-Sleep -Milliseconds (400 * $attempt)
                continue
            }
            Add-InstallWarning "Could not replace $LivePath due to an active file/process lock."
            return $false
        }
    }

    return $false
}

Write-Host ""
Write-Host " _  __     _    ____    ___    ___   __  __ " -ForegroundColor DarkYellow
Write-Host "| |/ /    / \  | __ )  / _ \  / _ \ |  \/  |" -ForegroundColor DarkYellow
Write-Host "| ' /    / _ \ |  _ \ | | | || | | || |\/| |" -ForegroundColor DarkYellow
Write-Host "| . \   / ___ \| |_) || |_| || |_| || |  | |" -ForegroundColor DarkYellow
Write-Host "|_|\_\ /_/   \_\____/  \___/  \___/ |_|  |_|" -ForegroundColor DarkYellow
Write-Host ""
Write-Host "🎸 Kaboom Installer" -ForegroundColor DarkYellow
Write-Host "--------------------------------------------------" -ForegroundColor DarkYellow
if (-not $STRICT_CHECKSUM) {
    Write-Host "⚠️  Checksum verification disabled (KABOOM_INSTALL_STRICT=0) — downloads will NOT be integrity-checked." -ForegroundColor Yellow
}

function New-ExtensionStage {
    if (Test-Path $STAGE_EXT_DIR) {
        Remove-Item -Path $STAGE_EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
    }
    New-Item -Path $STAGE_EXT_DIR -ItemType Directory -Force | Out-Null
}

function Test-ExtensionStage {
    param(
        [string]$BaseDir = $EXT_DIR
    )

    if (-not (Test-Path (Join-Path $BaseDir "manifest.json"))) {
        return $false
    }

    # Support both modern bundled extension layout and legacy modular layout
    # (mirrors validate_extension_stage in scripts/install.sh). Each group
    # requires at least one of its alternatives to be present.
    $alternativeGroups = @(
        @("background.js", "background\init.js"),
        @("content.bundled.js", "content\script-injection.js"),
        @("inject.bundled.js", "inject\index.js"),
        @("early-patch.bundled.js", "theme-bootstrap.js")
    )
    foreach ($group in $alternativeGroups) {
        $found = $false
        foreach ($candidate in $group) {
            if (Test-Path (Join-Path $BaseDir $candidate)) {
                $found = $true
                break
            }
        }
        if (-not $found) {
            return $false
        }
    }
    return $true
}

function Promote-ExtensionStage {
    if (-not (Test-ExtensionStage -BaseDir $STAGE_EXT_DIR)) {
        throw "Extension staging failed: required module files are missing from staging."
    }

    if (Test-Path $BACKUP_EXT_DIR) {
        Remove-Item -Path $BACKUP_EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
    }

    if (Test-Path $EXT_DIR) {
        Move-Item -Path $EXT_DIR -Destination $BACKUP_EXT_DIR -Force
    }

    $extensionParentDir = Split-Path -Path $EXT_DIR -Parent
    if (-not [string]::IsNullOrWhiteSpace($extensionParentDir) -and -not (Test-Path $extensionParentDir)) {
        New-Item -Path $extensionParentDir -ItemType Directory -Force | Out-Null
    }

    try {
        Move-Item -Path $STAGE_EXT_DIR -Destination $EXT_DIR -Force
    } catch {
        if (Test-Path $BACKUP_EXT_DIR) {
            Move-Item -Path $BACKUP_EXT_DIR -Destination $EXT_DIR -Force -ErrorAction SilentlyContinue
        }
        throw
    }

    if (-not (Test-ExtensionStage -BaseDir $EXT_DIR)) {
        Remove-Item -Path $EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
        if (Test-Path $BACKUP_EXT_DIR) {
            Move-Item -Path $BACKUP_EXT_DIR -Destination $EXT_DIR -Force -ErrorAction SilentlyContinue
        }
        throw "Promoted extension failed validation; previous extension restored."
    }

    if (Test-Path $BACKUP_EXT_DIR) {
        Remove-Item -Path $BACKUP_EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# 1. Fetch Version: pinned KABOOM_VERSION wins; otherwise the latest tag from STABLE.
if ($env:KABOOM_VERSION) {
    $VERSION = $env:KABOOM_VERSION.Trim()
    Write-Host "🔍 Installing pinned version v$VERSION (KABOOM_VERSION)..."
} else {
    Write-Host "🔍 Checking for updates..."
    $VERSION = (Invoke-RestMethod -Uri $VERSION_URL).Trim()
}
Write-Host "✨ Version: v$VERSION (win32-x64)"

# 2. Directory Setup: Ensure the target installation folders exist on the filesystem.
if (-not (Test-Path $BIN_DIR)) { New-Item -Path $BIN_DIR -ItemType Directory -Force }
if (-not (Test-Path $INSTALL_DIR)) { New-Item -Path $INSTALL_DIR -ItemType Directory -Force | Out-Null }
Write-Host "📁 Install root: $INSTALL_DIR"

# 3. Binary Installation: Download the Windows-native executable.
$INSTALL_BIN = $KABOOM_BIN
$BINARY_NAME = "kaboom-agentic-browser-win32-x64.exe"
$BINARY_URL = "https://github.com/$REPO/releases/download/v$VERSION/$BINARY_NAME"
$CHECKSUM_URL = "https://github.com/$REPO/releases/download/v$VERSION/checksums.txt"
$STAGED_BIN = "$KABOOM_BIN.tmp.$TEMP_TOKEN"

Write-Host "⬇️  Downloading latest binary..."
# Download to a temporary '.tmp' file to ensure an atomic replacement later.
if (Test-Path $STAGED_BIN) {
    Remove-Item -Path $STAGED_BIN -Force -ErrorAction SilentlyContinue
}
Invoke-WebRequest -Uri $BINARY_URL -OutFile $STAGED_BIN

# 4. Integrity Verification: Verify the SHA-256 hash against the official release manifest.
 $checksumVerified = $false
try {
    # Fetch the checksum manifest and parse the hash for the windows binary.
    $checksums = Invoke-RestMethod -Uri $CHECKSUM_URL
    $expectedLine = ($checksums -split "`n") | Where-Object { $_ -match $BINARY_NAME }
    if (-not $expectedLine) {
        throw "checksums.txt did not include $BINARY_NAME"
    }

    $expectedHash = ($expectedLine -split "\s+")[0].ToLower()
    # Calculate the hash of the downloaded file using built-in Windows security tools.
    $actualHash = (Get-FileHash $STAGED_BIN -Algorithm SHA256).Hash.ToLower()
    if ($expectedHash -ne $actualHash) {
        throw "Checksum mismatch for $BINARY_NAME"
    }

    $checksumVerified = $true
    Write-Host "✅ Checksum verified."
} catch {
    $msg = $_.Exception.Message
    if ($msg -like "*mismatch*") {
        throw "❌ Checksum verification failed! The download may be corrupted or tampered with."
    }
    if ($STRICT_CHECKSUM) {
        throw "❌ Strict checksum mode: $msg"
    }
    # Non-fatal warning if checksums cannot be verified (e.g., firewall issues) and strict mode is disabled.
    Write-Host "⚠️  Checksum verification skipped: $msg" -ForegroundColor Yellow
}

if ($STRICT_CHECKSUM -and -not $checksumVerified) {
    throw "❌ Strict checksum mode: verification did not complete successfully."
}

# Force-stop old server, then replace binary with retries for lock contention.
if (-not (Replace-KaboomBinary -StagePath $STAGED_BIN -LivePath $KABOOM_BIN)) {
    $FALLBACK_BIN = Join-Path $BIN_DIR "kaboom-agentic-browser.new.exe"
    try {
        Move-Item -Path $STAGED_BIN -Destination $FALLBACK_BIN -Force -ErrorAction Stop
        $INSTALL_BIN = $FALLBACK_BIN
        Add-InstallWarning "Using fallback binary $FALLBACK_BIN because kaboom-agentic-browser.exe could not be replaced."
    } catch {
        Add-InstallWarning "Downloaded update could not be installed. kaboom-agentic-browser.exe is likely still locked by a running process."
    }
} else {
    Write-Host "✅ Binary replaced: $KABOOM_BIN"
}

# 5. Extension Staging: Refresh the browser extension files.
# Tries the optimized release asset first, falling back to the full source zip if missing.
Write-Host "⬇️  Refreshing browser extension..."
$EXT_ZIP_NAME = "kaboom-extension-v$VERSION.zip"
$EXT_ZIP_URL = "https://github.com/$REPO/releases/download/v$VERSION/$EXT_ZIP_NAME"
$TEMP_ZIP = Join-Path $env:TEMP "kaboom-ext-$TEMP_TOKEN.zip"
$TEMP_EXTRACT = Join-Path $env:TEMP "kaboom-ext-src-$TEMP_TOKEN"

try {
    Invoke-WebRequest -Uri $EXT_ZIP_URL -OutFile $TEMP_ZIP
    New-ExtensionStage
    Expand-Archive -Path $TEMP_ZIP -DestinationPath $STAGE_EXT_DIR -Force
    if (-not (Test-ExtensionStage -BaseDir $STAGE_EXT_DIR)) {
        throw "Release extension zip missing required module files"
    }
} catch {
    # Fallback logic for older releases or bad extension zip assets.
    Write-Host "⚠️  Falling back to source zip due to missing/incomplete extension zip" -ForegroundColor Yellow
    $SOURCE_ZIP_URL = "https://github.com/$REPO/archive/refs/heads/STABLE.zip"
    Invoke-WebRequest -Uri $SOURCE_ZIP_URL -OutFile $TEMP_ZIP
    if (Test-Path $TEMP_EXTRACT) { Remove-Item -Path $TEMP_EXTRACT -Recurse -Force }
    Expand-Archive -Path $TEMP_ZIP -DestinationPath $TEMP_EXTRACT -Force
    # Find the extracted folder (named repo-version) and copy the extension subdirectory.
    $extractRoot = Get-ChildItem -Path $TEMP_EXTRACT | Where-Object { $_.PSIsContainer } | Select-Object -First 1
    if (-not $extractRoot) {
        throw "Source zip extraction failed: missing root directory."
    }

    $sourceExtensionDir = Join-Path $extractRoot.FullName "extension"
    if (-not (Test-Path $sourceExtensionDir)) {
        throw "Source zip extraction failed: missing extension directory."
    }

    New-ExtensionStage
    Copy-Item -Path (Join-Path $sourceExtensionDir "*") -Destination $STAGE_EXT_DIR -Recurse -Force
    if (-not (Test-ExtensionStage -BaseDir $STAGE_EXT_DIR)) {
        throw "Extension staging failed: required module files are missing."
    }
}

Promote-ExtensionStage
Write-Host "✅ Staged extension directory: $EXT_DIR"

# Clean up staging temp directories.
if (Test-Path $TEMP_EXTRACT) {
    Remove-Item -Path $TEMP_EXTRACT -Recurse -Force -ErrorAction SilentlyContinue
}
if (Test-Path $STAGE_EXT_DIR) {
    Remove-Item -Path $STAGE_EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
}
if (Test-Path $BACKUP_EXT_DIR) {
    Remove-Item -Path $BACKUP_EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
}
# Cleanup the temporary zip file.
Remove-Item -Path $TEMP_ZIP -ErrorAction SilentlyContinue

# 6. Native Configuration: Execute the Go binary to handle complex client configuration.
# The binary's --install flag will:
#   - Detect all installed MCP clients (Claude, Cursor, VS Code, etc.).
#   - Safely update JSON configuration files with Windows-aware paths.
#   - Reset any running Kaboom and legacy processes.
#   - Display final success message and extension instructions.
Write-Host "🚀 Finalizing configuration..."
if (-not (Test-Path $INSTALL_BIN)) {
    Add-InstallWarning "Installer could not locate an executable to run for --install."
    Show-InstallWarnings
    throw "Kaboom binary install failed. See warning panel for manual recovery steps."
}

& $INSTALL_BIN --install
[void](Stop-KaboomServerProcesses)
Show-InstallWarnings

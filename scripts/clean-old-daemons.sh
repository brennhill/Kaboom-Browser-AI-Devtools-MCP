#!/bin/bash
# Clean up Kaboom daemons before upgrading
# Usage: ./scripts/clean-old-daemons.sh
# Or: kaboom --force

set -euo pipefail

echo "🧹 Kaboom Daemon Cleanup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

KILLED=0
_FAILED=0  # reserved for future error reporting

# Function to kill a process safely
kill_process() {
  local pid=$1
  local name=$2

  if kill -0 "$pid" 2>/dev/null; then
    echo "  → Killing $name (PID $pid)"
    if kill -TERM "$pid" 2>/dev/null; then
      # Wait for graceful exit
      local waited=0
      while kill -0 "$pid" 2>/dev/null && [ $waited -lt 10 ]; do
        sleep 0.1
        ((waited++))
      done
      # If still running, force kill
      if kill -0 "$pid" 2>/dev/null; then
        kill -9 "$pid" 2>/dev/null || true
      fi
      ((KILLED++))
    fi
  fi
}

# Platform-specific cleanup
if [[ "$OSTYPE" == "darwin"* ]]; then
  # macOS: use lsof and pkill
  echo "Platform: macOS"
  echo ""
  echo "Searching for Kaboom processes..."

  PIDS=$(lsof -c "kaboom" -a -d cwd 2>/dev/null | tail -n +2 | awk '{print $2}' | sort -u || true)

  if [ -z "$PIDS" ]; then
    echo "  No Kaboom processes found"
  else
    for pid in $PIDS; do
      kill_process "$pid" "Kaboom process"
    done
  fi

  # Also try pkill as fallback
  pkill -9 -f "kaboom.*--daemon" 2>/dev/null || true

elif [[ "$OSTYPE" == "linux"* ]]; then
  # Linux: use pgrep/pkill
  echo "Platform: Linux"
  echo ""
  echo "Searching for Kaboom processes..."

  PIDS=$(pgrep -f "kaboom.*--daemon" 2>/dev/null | sort -u || true)

  if [ -z "$PIDS" ]; then
    echo "  No Kaboom processes found"
  else
    for pid in $PIDS; do
      kill_process "$pid" "Kaboom process"
    done
  fi

elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
  # Windows
  echo "Platform: Windows"
  echo ""
  echo "Searching for Kaboom processes..."

  if taskkill /F /IM "kaboom-agentic-browser.exe" 2>/dev/null; then
    ((KILLED++))
  else
    echo "  No Kaboom processes found"
  fi
fi

# Clean up PID files
echo ""
echo "Cleaning up PID files..."
state_root="${KABOOM_STATE_DIR:-${XDG_STATE_HOME:+$XDG_STATE_HOME/kaboom}}"
state_root="${state_root:-$HOME/.kaboom}"
for port in {7890..7910}; do
  pid_file="$state_root/run/kaboom-$port.pid"
  if [ -f "$pid_file" ]; then
    rm -f "$pid_file"
    echo "  Removed $pid_file"
  fi
done

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$KILLED" -gt 0 ]; then
  echo "✓ Killed $KILLED Kaboom process(es)"
  else
  echo "✓ No running Kaboom processes found"
  fi

  echo "Safe to install or upgrade Kaboom now:"
  echo "  npm install -g kaboom-agentic-browser@latest"
echo ""

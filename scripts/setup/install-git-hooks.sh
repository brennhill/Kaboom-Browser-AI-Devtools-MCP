#!/bin/bash
# Install git hooks for Kaboom project
# Run this once: ./scripts/setup/install-git-hooks.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

echo "=== Installing Kaboom Git Hooks ==="
echo ""

# Create pre-commit hook
cat > "$HOOKS_DIR/pre-commit" << 'EOF'
#!/bin/bash
# Pre-commit hook for Kaboom
# Prevents commits with uncompiled TypeScript or failing tests

set -e

echo "🔍 Pre-commit checks..."
CMD_PKG="${KABOOM_CMD_PKG:-./cmd/browser-agent}"

# Check if TypeScript source was modified
if git diff --cached --name-only | grep -q "^src/.*\.ts$"; then
  echo "  ⚙️  TypeScript files changed, compiling..."

  # Compile TypeScript
  if ! make compile-ts; then
    echo ""
    echo "❌ TypeScript compilation failed!"
    echo "   Fix the errors above before committing."
    exit 1
  fi

  # Stage compiled output if compilation succeeded
  git add extension/

  echo "  ✅ TypeScript compiled successfully"
fi

# Run quick tests
echo "  🧪 Running quick tests..."
if ! go vet "${CMD_PKG}/" >/dev/null 2>&1; then
  echo ""
  echo "❌ go vet failed!"
  echo "   Run 'go vet ${CMD_PKG}/' to see errors"
  exit 1
fi

echo "  ✅ All pre-commit checks passed"
echo ""
EOF

chmod +x "$HOOKS_DIR/pre-commit"

echo "✅ Pre-commit hook installed at: $HOOKS_DIR/pre-commit"
echo ""
echo "The hook will:"
echo "  1. Detect TypeScript changes in src/"
echo "  2. Compile TypeScript automatically"
echo "  3. Stage compiled output"
echo "  4. Run go vet"
echo "  5. Block commit if anything fails"
echo ""
echo "To bypass the hook (NOT recommended):"
echo "  git commit --no-verify"
echo ""

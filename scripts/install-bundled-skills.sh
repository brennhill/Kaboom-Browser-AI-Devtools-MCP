#!/usr/bin/env bash
# install-bundled-skills.sh
# Purpose: Install bundled managed skills from source for manual/local Kaboom builds.
# Why: Gives source-build users the same skill availability as npm/PyPI install flows.
# Docs: docs/features/feature/enhanced-cli-config/index.md
# Install bundled Kaboom skills from source tree for manual/local builds.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_SRC_DIR="${KABOOM_BUNDLED_SKILLS_DIR:-$PROJECT_ROOT/npm/kaboom-agentic-browser/skills}"
MARKER="<!-- kaboom-managed-skill"

SCOPE="${KABOOM_SKILL_SCOPE:-global}"
TARGETS_RAW="${KABOOM_SKILL_TARGETS:-${KABOOM_SKILL_TARGET:-claude,codex,gemini}}"
PROJECT_SCOPE_ROOT="${KABOOM_PROJECT_ROOT:-$(pwd)}"

CREATED=0
UPDATED=0
UNCHANGED=0
SKIPPED=0
ERRORS=0

case "$SCOPE" in
  global|project|all) ;;
  *)
    echo "Invalid KABOOM_SKILL_SCOPE='$SCOPE' (expected: global, project, all)" >&2
    exit 1
    ;;
esac

if [ ! -d "$SKILLS_SRC_DIR" ] || [ ! -f "$SKILLS_SRC_DIR/skills.json" ]; then
  echo "Bundled skills directory not found: $SKILLS_SRC_DIR" >&2
  exit 1
fi

# Emit "id<TAB>version" lines from skills.json so installs use per-skill
# versions from the manifest (never a hardcoded version) and only install
# manifest-listed skills. jq-free: prefers node, falls back to python3.
manifest_entries() {
  local manifest="$SKILLS_SRC_DIR/skills.json"
  if command -v node >/dev/null 2>&1; then
    node -e '
      const manifest = require(process.argv[1]);
      for (const skill of manifest.skills || []) {
        if (skill && typeof skill.id === "string" && skill.id) {
          console.log(`${skill.id}\t${skill.version || 1}`);
        }
      }
    ' "$manifest"
  elif command -v python3 >/dev/null 2>&1; then
    python3 -c '
import json, sys
with open(sys.argv[1]) as fh:
    manifest = json.load(fh)
for skill in manifest.get("skills", []):
    if isinstance(skill, dict) and skill.get("id"):
        print("%s\t%s" % (skill["id"], skill.get("version", 1)))
' "$manifest"
  else
    echo "install-bundled-skills.sh requires node or python3 to parse skills.json" >&2
    return 1
  fi
}

agent_global_root() {
  local agent="$1"
  case "$agent" in
    claude)
      printf "%s\n" "${KABOOM_CLAUDE_SKILLS_DIR:-$HOME/.claude/skills}"
      ;;
    codex)
      local codex_home="${CODEX_HOME:-$HOME/.codex}"
      printf "%s\n" "${KABOOM_CODEX_SKILLS_DIR:-$codex_home/skills}"
      ;;
    gemini)
      local gemini_home="${GEMINI_HOME:-$HOME/.gemini}"
      printf "%s\n" "${KABOOM_GEMINI_SKILLS_DIR:-$gemini_home/skills}"
      ;;
    *)
      return 1
      ;;
  esac
}

agent_project_root() {
  local agent="$1"
  case "$agent" in
    claude)
      printf "%s\n" "$PROJECT_SCOPE_ROOT/.claude/skills"
      ;;
    codex)
      printf "%s\n" "$PROJECT_SCOPE_ROOT/.codex/skills"
      ;;
    gemini)
      printf "%s\n" "$PROJECT_SCOPE_ROOT/.gemini/skills"
      ;;
    *)
      return 1
      ;;
  esac
}

skill_dest_path() {
  local agent="$1"
  local root="$2"
  local skill_id="$3"
  if [ "$agent" = "codex" ]; then
    printf "%s\n" "$root/$skill_id/SKILL.md"
  else
    printf "%s\n" "$root/$skill_id.md"
  fi
}

install_skill() {
  local agent="$1"
  local root="$2"
  local skill_id="$3"
  local version="$4"
  local src_file="$SKILLS_SRC_DIR/$skill_id/SKILL.md"

  if [ ! -f "$src_file" ]; then
    return
  fi

  local dest
  dest="$(skill_dest_path "$agent" "$root" "$skill_id")"
  mkdir -p "$(dirname "$dest")"

  local tmp_file
  tmp_file="$(mktemp)"
  local managed_marker="$MARKER id:$skill_id version:$version -->"
  if [ "$agent" = "codex" ] && [ "$(head -n 1 "$src_file")" = "---" ]; then
    awk -v marker="$managed_marker" '
      { print }
      NR > 1 && $0 == "---" && !inserted {
        print marker
        inserted = 1
      }
    ' "$src_file" >"$tmp_file"
  else
    {
      printf "%s\n" "$managed_marker"
      cat "$src_file"
    } >"$tmp_file"
  fi

  if [ -f "$dest" ]; then
    if cmp -s "$tmp_file" "$dest"; then
      UNCHANGED=$((UNCHANGED + 1))
      rm -f "$tmp_file"
      return
    fi
    if ! grep -Fq "$MARKER" "$dest"; then
      SKIPPED=$((SKIPPED + 1))
      rm -f "$tmp_file"
      return
    fi
    if cp "$tmp_file" "$dest"; then
      UPDATED=$((UPDATED + 1))
    else
      ERRORS=$((ERRORS + 1))
    fi
  else
    if cp "$tmp_file" "$dest"; then
      CREATED=$((CREATED + 1))
    else
      ERRORS=$((ERRORS + 1))
    fi
  fi

  rm -f "$tmp_file"
}

# Resolve manifest once; iterate manifest-listed skills (never raw directory globs).
MANIFEST_ENTRIES="$(manifest_entries)" || exit 1

for agent in $(printf "%s" "$TARGETS_RAW" | tr ',' ' '); do
  case "$agent" in
    claude|codex|gemini) ;;
    *)
      echo "Skipping unknown agent: $agent"
      continue
      ;;
  esac

  roots=""
  if [ "$SCOPE" = "global" ] || [ "$SCOPE" = "all" ]; then
    roots="$(agent_global_root "$agent")"
  fi
  if [ "$SCOPE" = "project" ] || [ "$SCOPE" = "all" ]; then
    if [ -n "$roots" ]; then
      roots="$roots
$(agent_project_root "$agent")"
    else
      roots="$(agent_project_root "$agent")"
    fi
  fi

  while IFS= read -r root; do
    [ -z "$root" ] && continue
    while IFS=$'\t' read -r skill_id skill_version; do
      [ -z "$skill_id" ] && continue
      install_skill "$agent" "$root" "$skill_id" "${skill_version:-1}"
    done <<EOF_SKILLS
$MANIFEST_ENTRIES
EOF_SKILLS
  done <<EOF_ROOTS
$roots
EOF_ROOTS

done

echo "Skills installed (${TARGETS_RAW} / ${SCOPE}): created=${CREATED} updated=${UPDATED} unchanged=${UNCHANGED} skipped=${SKIPPED} errors=${ERRORS}"

if [ "$ERRORS" -gt 0 ]; then
  exit 1
fi

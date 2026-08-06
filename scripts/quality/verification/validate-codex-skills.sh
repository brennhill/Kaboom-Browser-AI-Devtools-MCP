#!/bin/bash
# validate-codex-skills.sh
# Ensures repo-local Codex skills are parseable at startup.

set -euo pipefail

SKILLS_DIR=".codex/skills"
status=0

if [ ! -d "$SKILLS_DIR" ]; then
  echo "No $SKILLS_DIR directory; skipping Codex skill validation."
  exit 0
fi

echo "Validating Codex skill frontmatter..."

for file in "$SKILLS_DIR"/*/SKILL.md; do
  if [ ! -f "$file" ]; then
    continue
  fi

  rel="${file#./}"
  first_line="$(sed -n '1p' "$file")"
  if [ "$first_line" != "---" ]; then
    echo "❌ $rel: first line must be --- (YAML frontmatter start)"
    status=1
    continue
  fi

  end_line="$(awk 'NR > 1 && $0 == "---" { print NR; exit }' "$file")"
  if [ -z "${end_line}" ]; then
    echo "❌ $rel: missing closing --- for YAML frontmatter"
    status=1
    continue
  fi

  frontmatter="$(sed -n "2,$((end_line - 1))p" "$file")"
  if ! printf '%s\n' "$frontmatter" | grep -q '^name:[[:space:]]*[^[:space:]]'; then
    echo "❌ $rel: frontmatter missing non-empty name"
    status=1
  fi
  if ! printf '%s\n' "$frontmatter" | grep -q '^description:[[:space:]]*[^[:space:]]'; then
    echo "❌ $rel: frontmatter missing non-empty description"
    status=1
  fi
done

if [ "$status" -ne 0 ]; then
  echo "Codex skill validation failed."
  exit 1
fi

echo "✅ Codex skill frontmatter valid"

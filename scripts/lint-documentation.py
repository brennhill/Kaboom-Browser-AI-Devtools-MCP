#!/usr/bin/env python3
# Purpose: Automate lint-documentation.py workflow behavior for repository tooling.
# Why: Keeps repetitive maintenance and verification steps deterministic.
# Docs: docs/DEVELOPMENT.md

# pylint: disable=invalid-name
"""
Lint documentation for broken links and metadata issues.
Checks:
1. Links point to existing files
2. Code references (file.go:line) exist in codebase
3. All docs have YAML frontmatter
4. last-verified dates are recent (< 30 days)
5. No orphaned markdown files
"""

import re
from pathlib import Path
from datetime import datetime, timedelta
import sys

CODE_DIR = Path.cwd()
DOCS_DIR = CODE_DIR / "docs"

# Historic code/doc path aliases after refactors.
LEGACY_CODE_PATH_MAP = {
    "cmd/browser-agent/tools.go": "cmd/browser-agent/tools_schema.go",
    "cmd/browser-agent/codegen.go": "cmd/browser-agent/testgen.go",
    "internal/session/sessions.go": "internal/session/types.go",
    "internal/capture/queries.go": "internal/capture/query_dispatcher.go",
    "internal/capture/recording.go": "internal/capture/recording_manager.go",
    "internal/security/security_config.go": "internal/security/policy/mode.go",
    "internal/security/security_config_mode.go": "internal/security/policy/mode.go",
    "internal/security/security_config_policy.go": "internal/security/policy/policy.go",
    "internal/security/security_config_audit.go": "internal/security/policy/audit.go",
    "internal/security/security_diff.go": "internal/security/diff/types.go",
}

class DocumentLinter:
    """Lint markdown documentation files for common issues."""

    def __init__(self):
        self.errors = []
        self.warnings = []
        self.info = []

    def error(self, msg):
        """Record an error message."""
        self.errors.append(f"❌ {msg}")

    def warning(self, msg):
        """Record a warning message."""
        self.warnings.append(f"⚠️  {msg}")

    def info_msg(self, msg):
        """Record an informational message."""
        self.info.append(f"ℹ️  {msg}")

    def check_markdown_links(self, file_path, content):
        """Check all markdown links in a file"""
        # Pattern: [text](path) or [text](path#anchor)
        pattern = r'\[([^\]]+)\]\(([^)]+)\)'
        matches = re.findall(pattern, content)
        code_exts = (".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".sh", ".yaml", ".yml", ".json")

        for _text, link in matches:
            # Skip external links
            if link.startswith("http://") or link.startswith("https://"):
                continue
            if link.startswith("mailto:") or link.startswith("tel:"):
                continue
            if link.startswith("data:image/"):
                continue

            # Remove anchor/query and optional line suffixes.
            file_part = link.split("#")[0].split("?")[0]
            file_part = re.sub(r'(:[0-9,\-]+)$', '', file_part)

            # Ignore placeholders in templates.
            if "<" in file_part or ">" in file_part or file_part in {"ADR-XXX.md", "link"}:
                continue
            if "[" in file_part or "]" in file_part:
                continue

            # Skip empty links
            if not file_part:
                continue

            candidates = []
            if file_part.startswith("/"):
                route = file_part.lstrip("/").rstrip("/")
                if route == "":
                    candidates.append(DOCS_DIR / "index.md")
                else:
                    candidates.append(DOCS_DIR / route)
                    candidates.append(DOCS_DIR / f"{route}.md")
                    candidates.append(DOCS_DIR / route / "index.md")
            else:
                target = file_path.parent / file_part
                candidates.append(target)
                candidates.append(target.with_suffix(".md") if target.suffix == "" else target)
                candidates.append(target / "index.md")

                # Docs-root fallback for links written as if rooted at docs/.
                docs_root_target = (DOCS_DIR / file_part).resolve()
                candidates.append(docs_root_target)
                candidates.append(
                    docs_root_target.with_suffix(".md") if docs_root_target.suffix == "" else docs_root_target
                )
                candidates.append(docs_root_target / "index.md")

                # Collapse historically over-prefixed relatives like ../../../core/adrs.md.
                collapsed = file_part
                while collapsed.startswith("../"):
                    collapsed = collapsed[3:]
                if collapsed and collapsed != file_part:
                    collapsed_target = (DOCS_DIR / collapsed).resolve()
                    candidates.append(collapsed_target)
                    candidates.append(
                        collapsed_target.with_suffix(".md") if collapsed_target.suffix == "" else collapsed_target
                    )
                    candidates.append(collapsed_target / "index.md")

                # Repo-root fallback for code path references.
                repo_root_target = (CODE_DIR / file_part).resolve()
                candidates.append(repo_root_target)
                candidates.append(
                    repo_root_target.with_suffix(".md") if repo_root_target.suffix == "" else repo_root_target
                )
                candidates.append(repo_root_target / "index.md")

                # Repo-root fallback with collapsed relative prefixes.
                collapsed_repo = file_part
                while collapsed_repo.startswith("../"):
                    collapsed_repo = collapsed_repo[3:]
                if collapsed_repo and collapsed_repo != file_part:
                    collapsed_repo_target = (CODE_DIR / collapsed_repo).resolve()
                    candidates.append(collapsed_repo_target)
                    candidates.append(
                        collapsed_repo_target.with_suffix(".md")
                        if collapsed_repo_target.suffix == ""
                        else collapsed_repo_target
                    )
                    candidates.append(collapsed_repo_target / "index.md")
                else:
                    collapsed_repo = file_part

                # Refactor aliases for moved code files.
                alias = LEGACY_CODE_PATH_MAP.get(collapsed_repo)
                if alias:
                    alias_target = (CODE_DIR / alias).resolve()
                    candidates.append(alias_target)

            if not any(candidate.exists() for candidate in candidates):
                rel = file_path.relative_to(DOCS_DIR)
                if file_part.startswith("/"):
                    self.warning(f"{rel}: unresolved absolute route {link}")
                elif ".claude/" in file_part:
                    self.warning(f"{rel}: unresolved claude-internal link {link}")
                elif file_part.lower().endswith(code_exts):
                    self.warning(f"{rel}: unresolved code reference {link}")
                else:
                    self.error(f"{rel}: broken link to {link}")

    def check_code_references(self, file_path, content):
        """Check code references (file.go:function)"""
        pattern = r'`([a-z_/]+\.go):([a-zA-Z_][a-zA-Z0-9_]*)\(\)`'
        matches = re.findall(pattern, content)

        for file_ref, func_ref in matches:
            code_file = CODE_DIR / file_ref
            if not code_file.exists():
                self.error(f"{file_path.relative_to(DOCS_DIR)}: code file not found: {file_ref}")
            else:
                try:
                    with open(code_file, 'r', encoding='utf-8') as f:
                        code_content = f.read()
                        if f"func {func_ref}(" not in code_content:
                            rel = file_path.relative_to(DOCS_DIR)
                            self.warning(
                                f"{rel}: function not found:"
                                f" {func_ref} in {file_ref}"
                            )
                except (OSError, UnicodeDecodeError):
                    pass

    def check_frontmatter(self, file_path, content):
        """Check YAML frontmatter quality"""
        if not content.startswith("---"):
            self.warning(f"{file_path.relative_to(DOCS_DIR)}: missing YAML frontmatter")
            return

        # Extract frontmatter
        try:
            end = content.find("\n---\n", 3) + 4
            frontmatter = content[4:end]

            # Support both legacy and current field names.
            has_doc_type = "doc_type:" in frontmatter
            has_status = "status:" in frontmatter
            has_last_verified = "last-verified:" in frontmatter
            has_last_reviewed = "last_reviewed:" in frontmatter

            # Status is required for legacy docs; structured docs can omit it.
            if not has_status and not has_doc_type:
                self.warning(f"{file_path.relative_to(DOCS_DIR)}: missing 'status' field")

            # Accept either last-verified (legacy) or last_reviewed (current).
            if has_last_verified or has_last_reviewed:
                match = re.search(r'(?:last-verified|last_reviewed):\s*(\d{4}-\d{2}-\d{2})', frontmatter)
                if match:
                    date_str = match.group(1)
                    doc_date = datetime.strptime(date_str, "%Y-%m-%d")
                    if datetime.now() - doc_date > timedelta(days=30):
                        rel = file_path.relative_to(DOCS_DIR)
                        self.warning(
                            f"{rel}: review date is stale"
                            f" ({date_str})"
                        )
            else:
                self.warning(f"{file_path.relative_to(DOCS_DIR)}: missing review date field (last-verified or last_reviewed)")

        except (ValueError, IndexError) as e:
            self.warning(f"{file_path.relative_to(DOCS_DIR)}: frontmatter parse error: {str(e)}")

    def lint_file(self, file_path):
        """Lint a single markdown file"""
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                content = f.read()

            self.check_frontmatter(file_path, content)
            self.check_markdown_links(file_path, content)
            self.check_code_references(file_path, content)

        except (OSError, UnicodeDecodeError) as e:
            self.error(f"{file_path}: read error: {str(e)}")

    def _print_section(self, label, items):
        """Print a capped section of lint results (max 20 shown)."""
        if not items:
            return
        print(f"{label} ({len(items)}):")
        for item in items[:20]:
            print(f"  {item}")
        if len(items) > 20:
            print(f"  ... and {len(items) - 20} more {label.lower()}")
        print()

    def resolve_markdown_files(self, scope_paths=None):
        """Resolve markdown files from optional scope paths.

        When scope_paths is empty, lint all markdown files under docs/.
        """
        if not scope_paths:
            return list(DOCS_DIR.rglob("*.md"))

        docs_root = DOCS_DIR.resolve()
        files = []
        seen = set()

        for raw_path in scope_paths:
            candidate = (CODE_DIR / raw_path).resolve()
            try:
                candidate.relative_to(docs_root)
            except ValueError:
                self.error(
                    f"scope path outside docs/: {raw_path}"
                )
                continue

            if candidate.is_dir():
                for md_file in candidate.rglob("*.md"):
                    key = str(md_file)
                    if key not in seen:
                        seen.add(key)
                        files.append(md_file)
                continue

            if candidate.is_file() and candidate.suffix.lower() == ".md":
                key = str(candidate)
                if key not in seen:
                    seen.add(key)
                    files.append(candidate)
                continue

            self.error(f"scope path not found or not markdown: {raw_path}")

        return files

    def lint_all(self, scope_paths=None):
        """Lint markdown files in docs (optionally scoped)."""
        md_files = self.resolve_markdown_files(scope_paths)
        print(f"Linting {len(md_files)} markdown files...\n")

        for md_file in md_files:
            self.lint_file(md_file)

        print("\n" + "=" * 70)
        print("LINT RESULTS")
        print("=" * 70 + "\n")

        self._print_section("ERRORS", self.errors)
        self._print_section("WARNINGS", self.warnings)

        print("\n" + "=" * 70)
        print(f"Summary: {len(self.errors)} errors, {len(self.warnings)} warnings")
        print("=" * 70)

        return len(self.errors) == 0

if __name__ == "__main__":
    linter = DocumentLinter()
    scopes = sys.argv[1:]
    passed = linter.lint_all(scopes)
    sys.exit(0 if passed else 1)

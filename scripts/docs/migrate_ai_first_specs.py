# Purpose: Automate migrate_ai_first_specs.py workflow behavior for repository tooling.
# Why: Keeps repetitive maintenance and verification steps deterministic.
# Docs: docs/setup/DEVELOPMENT.md

"""Migrate AI-first tech specs into feature directories."""

from __future__ import annotations

import datetime
from pathlib import Path
import shutil


def title_from_slug(feature_slug: str) -> str:
    """Convert a hyphenated slug to a capitalized title."""
    return " ".join(w.capitalize() for w in feature_slug.split("-") if w)


def ensure_scaffold(feature_dir: Path, feature_slug: str) -> None:
    """Create feature directory scaffold with PRODUCT_SPEC and ADRS files."""
    feature_dir.mkdir(parents=True, exist_ok=True)
    title = title_from_slug(feature_slug)

    product = feature_dir / "PRODUCT_SPEC.md"
    if not product.exists():
        product.write_text(
            "\n".join(
                [
                    f"# Product Spec: {title}",
                    "",
                    f"User-facing requirements, rationale, and deprecations"  # nosemgrep: python.lang.correctness.common-mistakes.string-concat-in-list.string-concat-in-list -- intentional multi-line string concatenation
                    f" for the {title} feature.",
                    "",
                    "- See also: [Tech Spec](TECH_SPEC.md)",
                    f"- See also: [{title} Review]({feature_slug}-review.md)",
                    "- See also: [Core Product Spec](../../../core/PRODUCT_SPEC.md)",
                    "",
                ]
            ),
            encoding="utf-8",
        )

    adrs = feature_dir / "ADRS.md"
    if not adrs.exists():
        adrs.write_text(
            "\n".join(
                [
                    f"# ADRs: {title}",
                    "",
                    f"Architectural decisions for the {title} feature.",
                    "",
                    "- See also: [Product Spec](PRODUCT_SPEC.md)",
                    "- See also: [Tech Spec](TECH_SPEC.md)",
                    f"- See also: [{title} Review]({feature_slug}-review.md)",
                    "",
                ]
            ),
            encoding="utf-8",
        )


def migrate_all(repo_root: Path) -> list[tuple[str, str, Path, Path]]:  # pylint: disable=too-many-locals
    """Migrate all tech specs from ai-first directory to feature directories."""
    docs = repo_root / "docs"
    ai_first = docs / "ai-first"
    archive = docs / "archive" / "ai-first"
    features_root = docs / "features" / "feature"

    today = datetime.date.today().isoformat()

    archive.mkdir(parents=True, exist_ok=True)

    migrated: list[tuple[str, str, Path, Path]] = []
    src_files = sorted(
        p
        for p in ai_first.iterdir()
        if p.is_file() and p.name.startswith("tech-spec-") and p.suffix == ".md"
    )

    for src in src_files:
        src_name = src.name
        src_slug = src_name[len("tech-spec-") : -len(".md")]

        feature_dir = features_root / src_slug
        ensure_scaffold(feature_dir, src_slug)

        dest_path = feature_dir / "TECH_SPEC.md"
        source_text = src.read_text(encoding="utf-8")

        title = title_from_slug(src_slug)
        banner = "\n".join(
            [
                "> **[MIGRATION NOTICE]**",
                "> Canonical location for this tech spec."  # nosemgrep: python.lang.correctness.common-mistakes.string-concat-in-list.string-concat-in-list -- intentional multi-line string concatenation
                f" Migrated from `/docs/ai-first/{src_name}`"
                f" on {today}.",
                "> See also: [Product Spec](PRODUCT_SPEC.md)"  # nosemgrep: python.lang.correctness.common-mistakes.string-concat-in-list.string-concat-in-list -- intentional multi-line string concatenation
                f" and [{title} Review]({src_slug}-review.md).",
                "",
                "",
            ]
        )

        dest_path.write_text(banner + source_text.lstrip("\n"), encoding="utf-8")

        archive_path = archive / src_name
        if archive_path.exists():
            archive_path.unlink()
        shutil.move(str(src), str(archive_path))

        migrated.append((src_name, src_slug, dest_path, archive_path))

    return migrated


if __name__ == "__main__":
    repo = Path(__file__).resolve().parents[1]
    results = migrate_all(repo)
    print(f"Migrated {len(results)} tech specs")
    for name, slug, dest, archived in results:
        print(f"- {name} -> {dest.relative_to(repo)} (archived {archived.relative_to(repo)})")

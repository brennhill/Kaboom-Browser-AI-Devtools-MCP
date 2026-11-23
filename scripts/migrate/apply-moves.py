#!/usr/bin/env python3
"""apply-moves.py — Relocate Go package directories per scripts/migrate/context-moves.tsv and rewrite every path reference.

Usage: scripts/migrate/apply-moves.py [--dry-run] [--skip PATH ...] | --help
Rows listed in scripts/migrate/applied-moves.txt are already done and ignored, so re-running is safe
(a moved package's old path can be reused as a new context root, e.g. internal/lifecycle).
Rows whose note contains REVIEW are skipped (package stays put). Rows with old == new are no-ops
but still shadow shorter parent rows. Directory moves use git mv per directory (files only) via a
temp staging area so a destination never collides with a source still in place.
"""
import os
import re
import subprocess
import sys

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
os.chdir(ROOT)
TSV = "scripts/migrate/context-moves.tsv"
MODULE = "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP"
if "--help" in sys.argv or "-h" in sys.argv:
    print(__doc__)
    sys.exit(0)
DRY = "--dry-run" in sys.argv
APPLIED_FILE = "scripts/migrate/applied-moves.txt"
APPLIED = set()
if os.path.exists(APPLIED_FILE):
    APPLIED = {l.strip() for l in open(APPLIED_FILE, encoding="utf-8") if l.strip() and not l.startswith("#")}
EXTRA_SKIP = set()
if "--skip" in sys.argv:
    EXTRA_SKIP = set(sys.argv[sys.argv.index("--skip") + 1:])

TEXT_EXCLUDE_PREFIXES = ("src/", "docs/audits/historical/", "scripts/migrate/")
TEXT_EXCLUDE_NAMES = ("package-lock.json",)


def load_rows():
    rows = {}
    skipped = []
    for line in open(TSV, encoding="utf-8"):
        if line.startswith("#") or not line.strip():
            continue
        parts = line.rstrip("\n").split("\t")
        old, new = parts[0], parts[1]
        note = parts[3] if len(parts) > 3 else ""
        if old in APPLIED:
            continue  # already moved; its old path may now be a new context root
        if "REVIEW" in note or old in EXTRA_SKIP:
            rows[old] = old  # stays; still shadows parent rows
            skipped.append(old)
        else:
            rows[old] = new
    return rows, skipped


ROWS, SKIPPED = load_rows()


def map_path(p):
    """Longest-prefix directory mapping for a path (dir or file path)."""
    best = None
    for old in ROWS:
        if p == old or p.startswith(old + "/"):
            if best is None or len(old) > len(best):
                best = old
    if best is None:
        return p
    return ROWS[best] + p[len(best):]


def git(*args):
    subprocess.check_call(["git", *args])


def move_dirs():
    # every directory (tracked files only) that holds files
    files = subprocess.check_output(["git", "ls-files", "-z"], text=True).split("\0")
    dirs = {}
    for f in files:
        if not f:
            continue
        d = os.path.dirname(f)
        if d.startswith(("internal/", "cmd/browser-agent/internal/")):
            dirs.setdefault(d, []).append(f)
    plan = []
    for d, fs in sorted(dirs.items()):
        nd = map_path(d)
        if nd != d:
            plan.append((d, nd, fs))
    print(f"{len(plan)} directories to move")
    if DRY:
        for d, nd, _ in plan:
            print(f"  {d} -> {nd}")
        return
    # phase 1: stage everything
    stage = ".migrate_stage"
    for i, (d, nd, fs) in enumerate(plan):
        for f in fs:
            dst = os.path.join(stage, str(i), os.path.basename(f))
            os.makedirs(os.path.dirname(dst), exist_ok=True)
            git("mv", f, dst)
    # phase 2: place
    for i, (d, nd, fs) in enumerate(plan):
        for f in fs:
            src = os.path.join(stage, str(i), os.path.basename(f))
            dst = os.path.join(nd, os.path.basename(f))
            os.makedirs(os.path.dirname(dst), exist_ok=True)
            git("mv", src, dst)
    if os.path.isdir(stage):
        os.rmdir(stage) if not os.listdir(stage) else None
    # remove now-empty old directories
    for d, _, _ in sorted(plan, key=lambda x: -len(x[0])):
        try:
            os.removedirs(d)
        except OSError:
            pass


# Path-string rewriting. Single pass, longest old first, boundary-checked.
_alts = sorted(ROWS, key=len, reverse=True)
_RX = re.compile(r"(?<![A-Za-z0-9_\-.])(" + "|".join(re.escape(o) for o in _alts) + r")(?![A-Za-z0-9_\-])")


def _sub(m):
    s = m.string
    st = m.start()
    # preceded by "/" -> only valid when it is the module path or ./ ../
    if st > 0 and s[st - 1] == "/":
        pre = s[max(0, st - len(MODULE) - 1):st]
        if not (pre.endswith(MODULE + "/") or s[max(0, st - 2):st] == "./"):
            return m.group(0)
    # "internal/x" must not be the tail of "cmd/browser-agent/internal/x"
    if st >= 14 and s[st - 14:st] == "browser-agent/" and m.group(0).startswith("internal/"):
        return m.group(0)
    return map_path(m.group(0))


def is_text(path):
    try:
        with open(path, "rb") as fh:
            return b"\0" not in fh.read(4096)
    except OSError:
        return False


def rewrite_text():
    files = subprocess.check_output(["git", "ls-files", "-z"], text=True).split("\0")
    changed = []
    for f in files:
        if not f or f.startswith(TEXT_EXCLUDE_PREFIXES) or os.path.basename(f) in TEXT_EXCLUDE_NAMES:
            continue
        if not os.path.isfile(f) or os.path.islink(f) or not is_text(f):
            continue
        try:
            src = open(f, encoding="utf-8").read()
        except UnicodeDecodeError:
            continue  # EXPECTED_ABSENCE: is_text() heuristics passed a non-UTF-8 file; it holds no path references
        out = _RX.sub(_sub, src)
        if out != src:
            changed.append(f)
            if not DRY:
                open(f, "w", encoding="utf-8").write(out)
    print(f"{len(changed)} files rewritten")
    return changed


if __name__ == "__main__":
    move_dirs()
    changed = rewrite_text()
    if not DRY:
        moved = sorted(old for old, new in ROWS.items() if new != old)
        if moved:
            with open(APPLIED_FILE, "a", encoding="utf-8") as fh:
                fh.write("\n".join(moved) + "\n")
            print(f"recorded {len(moved)} applied rows in {APPLIED_FILE}")

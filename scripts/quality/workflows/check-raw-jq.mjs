// check-raw-jq.mjs — Ratchets raw `| jq` in UAT scripts down toward the shared helper.
//
// PURPOSE: `printf '%s' "$text" | jq -r '.field' 2>/dev/null` prints nothing
// when the input does not parse, and "" compares equal to an expected-empty
// result. Every control case that expects zero findings is therefore satisfied
// by a response that was never parsed. scripts/tests/framework/json.sh
// distinguishes the two; this contract stops the unsafe form from spreading
// while the existing sites are migrated.
//
// Piped jq only. `jq '.cases | length' "$FILE"` reads a checked-in table whose
// syntax is already guaranteed, so gating it would be noise.

import { readFileSync, writeFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const REPO_ROOT = fileURLToPath(new URL("../../..", import.meta.url));
const SCAN_ROOT = join(REPO_ROOT, "scripts/tests");
const BASELINE_PATH = join(REPO_ROOT, ".raw-jq-baseline.json");

// The helper itself is the sanctioned home for piped jq.
const EXEMPT_FILES = new Set(["scripts/tests/framework/json.sh"]);

// A pipe into jq, with or without arguments between.
const PIPED_JQ = /\|\s*jq(\s|$)/;

export function shellFiles(root) {
  const found = [];
  const walk = (dir) => {
    for (const entry of readdirSync(dir)) {
      if (entry.startsWith(".") || entry === "node_modules") continue;
      const path = join(dir, entry);
      if (statSync(path).isDirectory()) {
        walk(path);
      } else if (entry.endsWith(".sh")) {
        found.push(path);
      }
    }
  };
  walk(root);
  return found.sort();
}

// countPipedJq counts lines piping into jq, skipping comments so the
// explanation of why the form is unsafe does not itself count as a use.
export function countPipedJq(source) {
  let count = 0;
  for (const line of source.split("\n")) {
    if (/^\s*#/.test(line)) continue;
    if (PIPED_JQ.test(line)) count += 1;
  }
  return count;
}

export function scan(root, repoRoot) {
  const counts = {};
  for (const path of shellFiles(root)) {
    const rel = relative(repoRoot, path).split("\\").join("/");
    if (EXEMPT_FILES.has(rel)) continue;
    const count = countPipedJq(readFileSync(path, "utf8"));
    if (count > 0) counts[rel] = count;
  }
  return counts;
}

// evaluate allows any reduction and any file to disappear, and rejects growth
// or a newly introduced file. A ratchet, not a freeze.
export function evaluate(current, baseline) {
  const violations = [];
  for (const [file, count] of Object.entries(current)) {
    const allowed = baseline[file] ?? 0;
    if (count > allowed) {
      violations.push(
        allowed === 0
          ? `${file}: ${count} raw \`| jq\` use(s) in a file that had none. Use json_field/json_payload from scripts/tests/framework/json.sh so a parse failure cannot read as an empty result.`
          : `${file}: raw \`| jq\` uses rose from ${allowed} to ${count}. This budget may only shrink.`,
      );
    }
  }
  return violations;
}

// lowered rewrites the baseline to the current counts, so a migration re-freezes
// at the new floor rather than leaving slack behind.
export function lowered(current, baseline) {
  const next = {};
  for (const [file, count] of Object.entries(current)) {
    next[file] = Math.min(count, baseline[file] ?? count);
  }
  return Object.fromEntries(Object.entries(next).sort(([a], [b]) => a.localeCompare(b)));
}

function readBaseline() {
  try {
    const parsed = JSON.parse(readFileSync(BASELINE_PATH, "utf8"));
    return parsed.files ?? {};
  } catch (error) {
    if (error.code === "ENOENT") {
      // EXPECTED_ABSENCE: with no baseline every file must be at zero, which
      // is the strictest reading and the correct default for a fresh checkout.
      return {};
    }
    throw error;
  }
}

function main() {
  const current = scan(SCAN_ROOT, REPO_ROOT);

  if (process.argv.includes("--update")) {
    const next = lowered(current, readBaseline());
    writeFileSync(
      BASELINE_PATH,
      `${JSON.stringify(
        {
          _comment:
            "Per-file budget for raw `| jq` in scripts/tests. Piped jq prints nothing when the input does not parse, so \"\" reads as an empty result. Migrate to json_field/json_payload in scripts/tests/framework/json.sh. Budgets may only shrink; refreeze with `node scripts/quality/workflows/check-raw-jq.mjs --update`.",
          files: next,
        },
        null,
        2,
      )}\n`,
    );
    const total = Object.values(next).reduce((sum, n) => sum + n, 0);
    console.log(`Rewrote raw-jq baseline: ${Object.keys(next).length} file(s), ${total} use(s).`);
    return;
  }

  const violations = evaluate(current, readBaseline());
  if (violations.length > 0) {
    console.error(`FAIL: ${violations.length} raw-jq violation(s)`);
    for (const violation of violations) console.error(`  - ${violation}`);
    process.exit(1);
  }
  const total = Object.values(current).reduce((sum, n) => sum + n, 0);
  console.log(
    `OK: ${total} raw \`| jq\` use(s) across ${Object.keys(current).length} file(s), none above budget.`,
  );
}

if (process.argv[1] && process.argv[1].endsWith("check-raw-jq.mjs")) {
  main();
}

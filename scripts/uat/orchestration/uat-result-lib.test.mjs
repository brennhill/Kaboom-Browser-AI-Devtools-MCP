// uat-result-lib.test.mjs — Regression contracts for UAT result aggregation helpers.
// Docs: docs/features/feature/self-testing/index.md

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

const library = fileURLToPath(new URL("./uat-result-lib.sh", import.meta.url));

function idsMatch(expected, actual) {
  const output = execFileSync(
    "/bin/bash",
    ["-c", 'source "$1"; if uat_category_ids_match "$2" "$3"; then printf match; else printf mismatch; fi', "test", library, expected, actual],
    { encoding: "utf8" },
  );
  return output === "match";
}

test("zero-padded and numeric category IDs identify the same category", () => {
  assert.equal(idsMatch("01", "1"), true);
  assert.equal(idsMatch("09", "9"), true);
  assert.equal(idsMatch("10", "10"), true);
});

test("different or malformed category IDs remain mismatches", () => {
  assert.equal(idsMatch("01", "10"), false);
  assert.equal(idsMatch("01", ""), false);
  assert.equal(idsMatch("01", "category-1"), false);
});

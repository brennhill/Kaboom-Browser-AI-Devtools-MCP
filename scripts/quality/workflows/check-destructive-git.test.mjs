// check-destructive-git.test.mjs — Pins the clean-tree guard contract.
import { test } from "node:test";
import assert from "node:assert/strict";
import { destructiveUses, hasCleanTreeGuard, evaluate } from "./check-destructive-git.mjs";

test("recognizes the discarding git commands", () => {
  const cases = [
    ['git checkout -- "$FILE"', "git checkout --"],
    ["git checkout -f main", "git checkout --"],
    ['git restore "$FILE"', "git restore"],
    ["git clean -fd", "git clean -f"],
    ["git reset --hard origin/main", "git reset --hard"],
    ["git stash drop", "git stash drop/clear"],
  ];
  for (const [line, name] of cases) {
    const uses = destructiveUses(line);
    assert.equal(uses.length, 1, `expected ${line} to be flagged`);
    assert.equal(uses[0].name, name);
  }
});

// Switching branches does not discard anything; git refuses when it would.
test("ignores a plain branch checkout", () => {
  assert.deepEqual(destructiveUses("git checkout UNSTABLE"), []);
  assert.deepEqual(destructiveUses("git checkout -b feature"), []);
});

test("ignores non-destructive git commands", () => {
  const source = ["git status --short", "git stash list", "git clean -n", "git reset HEAD~1"].join("\n");
  assert.deepEqual(destructiveUses(source), []);
});

// This very contract documents the banned commands in its own header.
test("ignores the pattern inside comments in both syntaxes", () => {
  assert.deepEqual(destructiveUses('# git checkout -- "$FILE" destroys work'), []);
  assert.deepEqual(destructiveUses('// git restore is banned without a guard'), []);
});

test("accepts each recognized guard form", () => {
  assert.ok(hasCleanTreeGuard('if [ -n "$(git status --porcelain)" ]; then exit 1; fi'));
  assert.ok(hasCleanTreeGuard('git diff --quiet || exit 1'));
  assert.ok(hasCleanTreeGuard("require_clean_tree"));
});

test("a guard that is only described in a comment does not count", () => {
  assert.equal(hasCleanTreeGuard("# call git status --porcelain first"), false);
});

test("an unguarded destructive script is a violation", () => {
  const violations = evaluate([{ path: "scripts/mutate.sh", source: 'git checkout -- "$F"' }]);
  assert.equal(violations.length, 1);
  assert.match(violations[0], /scripts\/mutate\.sh:1/);
  assert.match(violations[0], /not recoverable/);
});

test("a guarded destructive script passes", () => {
  const source = ['if [ -n "$(git status --porcelain)" ]; then exit 1; fi', 'git checkout -- "$F"'].join("\n");
  assert.deepEqual(evaluate([{ path: "scripts/mutate.sh", source }]), []);
});

// The guard often lives in a helper called before the destructive command, so
// requiring textual precedence would reward moving the call over adding a check.
test("a guard appearing after the command still counts", () => {
  const source = ['git checkout -- "$F"', 'git status --porcelain'].join("\n");
  assert.deepEqual(evaluate([{ path: "scripts/mutate.sh", source }]), []);
});

test("a script with no destructive command needs no guard", () => {
  assert.deepEqual(evaluate([{ path: "scripts/build.sh", source: "go build ./..." }]), []);
});

test("reports the first offending line number", () => {
  const source = ["#!/bin/bash", "echo hi", 'git restore "$F"'].join("\n");
  const violations = evaluate([{ path: "s.sh", source }]);
  assert.match(violations[0], /s\.sh:3/);
});

// category_resolution_test.go — Holds a UAT category id to naming exactly one script.
//
// PURPOSE: the comprehensive runner and the transcript recorder both locate a
// category by globbing cat-<id>-*.sh. Both resolved a multi-match with
// `find | head -n 1`, so which script ran depended on the order the filesystem
// returned paths in.
//
// It mattered. `cat-33-expectations.sh` — a table sourced by
// cat-33-connected-action-coverage.sh — sat in the same namespace, and find
// returned it first. Run as a script it defines variables and exits 0, so
// category 33, the category that invokes every live MCP mode, ran nothing.
//
// CONTRACT: every id the runner lists resolves to exactly one script, and the
// shared resolver refuses an ambiguous or absent id rather than choosing.

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	runnerFile   = "scripts/uat/runners/test-all-tools-comprehensive.sh"
	resolverFile = "scripts/uat/orchestration/uat-category-script.sh"
	testsRoot    = "scripts/tests"
)

// runnerCategoryIDs reads the id lists out of the runner itself, so a category
// added there is covered here without a second list to keep in step.
func runnerCategoryIDs(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), runnerFile))
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`(?m)^(?:OFFLINE|CONNECTED)_CAT_IDS="([^"]*)"`)
	matches := pattern.FindAllStringSubmatch(string(raw), -1)
	if len(matches) != 2 {
		t.Fatalf("found %d category id lists in %s, want 2 (OFFLINE and CONNECTED); this test would otherwise check nothing", len(matches), runnerFile)
	}
	var ids []string
	for _, match := range matches {
		ids = append(ids, strings.Fields(match[1])...)
	}
	if len(ids) == 0 {
		t.Fatal("the runner lists no categories, so this test would pass vacuously")
	}
	return ids
}

func TestEveryCategoryIDNamesExactlyOneScript(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	var ambiguous, missing []string
	for _, id := range runnerCategoryIDs(t) {
		found, err := filepath.Glob(filepath.Join(root, testsRoot, "*", "cat-"+id+"-*.sh"))
		if err != nil {
			t.Fatal(err)
		}
		switch len(found) {
		case 1:
		case 0:
			missing = append(missing, id)
		default:
			for i, path := range found {
				found[i], _ = filepath.Rel(root, path)
			}
			ambiguous = append(ambiguous, id+": "+strings.Join(found, ", "))
		}
	}

	if len(ambiguous) > 0 {
		t.Errorf("%d category id(s) match more than one script, so which one runs depends on filesystem order:\n  %s\nA sourced helper must not live in the cat-<id>-* namespace.",
			len(ambiguous), strings.Join(ambiguous, "\n  "))
	}
	if len(missing) > 0 {
		t.Errorf("%d category id(s) the runner lists have no script at all, so the suite silently runs fewer categories than it reports:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// resolve runs the shared shell resolver against a directory and reports what a
// caller would see. Exercising the real function is the point: a Go
// reimplementation of the glob would pass while the shell kept picking head -1.
func resolve(t *testing.T, dir, id string) (stdout string, exitCode int) {
	t.Helper()
	script := "set -e\n. " + filepath.Join(repoRoot(t), resolverFile) + "\nuat_resolve_category_script \"$1\" \"$2\""
	cmd := exec.Command("bash", "-c", script, "bash", dir, id)
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return string(out), exitErr.ExitCode()
		}
		t.Fatalf("running the resolver failed for a reason other than its exit status: %v", err)
	}
	return string(out), 0
}

func TestTheResolverRefusesAnAmbiguousID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	category := filepath.Join(dir, "browser")
	if err := os.MkdirAll(category, 0o755); err != nil {
		t.Fatal(err)
	}
	// Exactly the shape that hid the cat-33 bug: a real category beside a helper.
	for _, name := range []string{"cat-33-connected-action-coverage.sh", "cat-33-expectations.sh"} {
		if err := os.WriteFile(filepath.Join(category, name), []byte("#!/bin/bash\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	out, code := resolve(t, dir, "33")
	if code == 0 {
		t.Errorf("the resolver chose %q from two candidates; picking one silently is how category 33 ran a sourced library for as long as it did", strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("a refused resolution printed %q to stdout; a caller capturing it would run that path", strings.TrimSpace(out))
	}
}

func TestTheResolverRefusesAnAbsentID(t *testing.T) {
	t.Parallel()
	out, code := resolve(t, t.TempDir(), "99")
	if code == 0 {
		t.Error("the resolver reported success for a category that does not exist, so the suite would run fewer categories than it claims")
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("stdout was %q for an absent id, want empty", strings.TrimSpace(out))
	}
}

func TestTheResolverReturnsTheOneMatch(t *testing.T) {
	t.Parallel()
	// Control: without this, a resolver that failed unconditionally would satisfy
	// both refusal tests above.
	dir := t.TempDir()
	category := filepath.Join(dir, "browser")
	if err := os.MkdirAll(category, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(category, "cat-33-connected-action-coverage.sh")
	if err := os.WriteFile(want, []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, code := resolve(t, dir, "33")
	if code != 0 {
		t.Fatalf("the resolver refused an unambiguous id (exit %d)", code)
	}
	if strings.TrimSpace(out) != want {
		t.Errorf("resolved to %q, want %q", strings.TrimSpace(out), want)
	}
}

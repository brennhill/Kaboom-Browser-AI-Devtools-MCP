// check_go_test_determinism.go — Rejects wall-clock sleeps in Go tests.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func scanSleepCounts(root string) (map[string]int, error) {
	counts := make(map[string]int)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && ignoredDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		timeNames := make(map[string]bool)
		dotTime := false
		for _, spec := range parsed.Imports {
			if spec.Path.Value != `"time"` {
				continue
			}
			if spec.Name == nil {
				timeNames["time"] = true
			} else if spec.Name.Name == "." {
				dotTime = true
			} else if spec.Name.Name != "_" {
				timeNames[spec.Name.Name] = true
			}
		}
		count := 0
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Sleep" {
				if owner, ok := selector.X.(*ast.Ident); ok && timeNames[owner.Name] {
					count++
				}
			}
			if ident, ok := call.Fun.(*ast.Ident); ok && dotTime && ident.Name == "Sleep" {
				count++
			}
			return true
		})
		if count > 0 {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			counts[filepath.ToSlash(relative)] = count
		}
		return nil
	})
	return counts, err
}

func ignoredDirectory(name string) bool {
	switch name {
	case ".git", ".beads", "node_modules", "dist", "coverage", "vendor":
		return true
	default:
		return false
	}
}

func sleepViolations(counts map[string]int) []string {
	var violations []string
	for path, count := range counts {
		violations = append(violations, fmt.Sprintf("%s: %d time.Sleep call(s)", path, count))
	}
	sort.Strings(violations)
	return violations
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, arg := range os.Args[1:] {
		root = arg
	}
	counts, err := scanSleepCounts(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	violations := sleepViolations(counts)
	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "Go test determinism gate failed:")
		for _, violation := range violations {
			fmt.Fprintln(os.Stderr, "- "+violation)
		}
		fmt.Fprintln(os.Stderr, "Replace wall-clock sleeps with controlled synchronization, fake clocks, or explicit process/transport seams.")
		os.Exit(1)
	}
	fmt.Println("Go test determinism gate passed (zero time.Sleep calls)")
}

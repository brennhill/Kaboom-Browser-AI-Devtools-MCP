// check_go_test_determinism.go — Ratchets wall-clock sleeps out of Go tests.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const sleepBaselineName = ".go-test-sleep-baseline.json"

type sleepBaseline struct {
	Version int            `json:"version"`
	Files   map[string]int `json:"files"`
}

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

func evaluateSleepRatchet(counts, baseline map[string]int) []string {
	var violations []string
	for path, count := range counts {
		allowed := baseline[path]
		if count > allowed {
			violations = append(violations, fmt.Sprintf("%s: %d time.Sleep call(s), baseline allows %d", path, count, allowed))
		}
	}
	sort.Strings(violations)
	return violations
}

func loadSleepBaseline(path string) (map[string]int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var baseline sleepBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, err
	}
	if baseline.Version != 1 || baseline.Files == nil {
		return nil, fmt.Errorf("unsupported or incomplete sleep baseline")
	}
	return baseline.Files, nil
}

func writeSleepBaseline(path string, counts map[string]int) error {
	data, err := json.MarshalIndent(sleepBaseline{Version: 1, Files: counts}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	update := false
	for _, arg := range os.Args[1:] {
		if arg == "--update" {
			update = true
			continue
		}
		root = arg
	}
	counts, err := scanSleepCounts(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	baselinePath := filepath.Join(root, sleepBaselineName)
	if update {
		if err := writeSleepBaseline(baselinePath, counts); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Go test sleep baseline updated: %d file(s) retain wall-clock debt\n", len(counts))
		return
	}
	baseline, err := loadSleepBaseline(baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load %s: %v\n", sleepBaselineName, err)
		os.Exit(1)
	}
	violations := evaluateSleepRatchet(counts, baseline)
	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "Go test determinism gate failed:")
		for _, violation := range violations {
			fmt.Fprintln(os.Stderr, "- "+violation)
		}
		fmt.Fprintln(os.Stderr, "Replace wall-clock sleeps with controlled synchronization; baseline updates may only ratchet counts down.")
		os.Exit(1)
	}
	remaining := 0
	for _, count := range counts {
		remaining += count
	}
	fmt.Printf("Go test determinism ratchet passed (%d existing sleep call(s) across %d file(s); no increase)\n", remaining, len(counts))
}

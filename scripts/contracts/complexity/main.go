// main.go — Fails when any authored Go function exceeds its budgets.
//
// Budgets: cyclomatic complexity ≤ 15 (gocyclo counting), parameters ≤ 6
// (hard limit), body length ≤ 80 lines (ratcheting: existing longer functions
// are frozen at their current line count in .function-length-baseline-go.json
// and may only shrink, mirroring the folder-size gate).
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

const (
	maxComplexity = 15
	maxParams     = 6
	maxLength     = 80
	baselineName  = ".function-length-baseline-go.json"
)

type violation struct {
	file       string
	line       int
	function   string
	complexity int
	params     int
	lines      int
	kind       string
}

// ignoredDir skips trees that are not this checkout's hand-written source.
// Dot-directories are skipped wholesale: git worktrees live under
// .claude/worktrees and would otherwise be scanned as a second copy of the repo.
func ignoredDir(name string) bool {
	switch name {
	case "node_modules", "vendor", "dist", "build", "coverage", "generated", "testdata":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func funcComplexity(fn ast.Node) int {
	complexity := 1
	ast.Inspect(fn, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if typed.Op == token.LAND || typed.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}

func funcParams(fn *ast.FuncDecl) int {
	count := 0
	for _, field := range fn.Type.Params.List {
		if len(field.Names) == 0 {
			count++
			continue
		}
		count += len(field.Names)
	}
	return count
}

type lengthBaselineFile struct {
	Version  int         `json:"version"`
	MaxLines int         `json:"max_lines"`
	Functions map[string]int `json:"functions"`
}

func loadLengthBaseline(path string) (map[string]int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]int{}, nil
		}
		return nil, err
	}
	var parsed lengthBaselineFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if parsed.Version != 1 {
		return nil, fmt.Errorf("unsupported function-length baseline version in %s", path)
	}
	return parsed.Functions, nil
}

func writeLengthBaseline(path string, functions map[string]int) error {
	if functions == nil {
		functions = map[string]int{}
	}
	data, err := json.MarshalIndent(lengthBaselineFile{Version: 1, MaxLines: maxLength, Functions: functions}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// measure walks root and records every authored function's budgets.
func measure(root string, fset *token.FileSet) (map[string]violation, error) {
	measured := make(map[string]violation)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && ignoredDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		for _, declaration := range parsed.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			key := relative + ":" + fn.Name.Name
			measured[key] = violation{
				file:       relative,
				line:       fset.Position(fn.Pos()).Line,
				function:   fn.Name.Name,
				complexity: funcComplexity(fn),
				params:     funcParams(fn),
				lines:      fset.Position(fn.End()).Line - fset.Position(fn.Pos()).Line + 1,
			}
		}
		return nil
	})
	return measured, err
}

// lengthAllowance resolves the maximum body length for a "file:function" key;
// production passes baseline lookups, tests pass their own.
func evaluate(measured map[string]violation, complexityLimit int, lengthAllowance func(string) int) []violation {
	var violations []violation
	for key, fn := range measured {
		if fn.complexity > complexityLimit {
			v := fn
			v.kind = "complexity"
			violations = append(violations, v)
		}
		if fn.params > maxParams {
			v := fn
			v.kind = "params"
			violations = append(violations, v)
		}
		if fn.lines > lengthAllowance(key) {
			v := fn
			v.kind = "length"
			violations = append(violations, v)
		}
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].file != violations[j].file {
			return violations[i].file < violations[j].file
		}
		return violations[i].line < violations[j].line
	})
	return violations
}

// scan walks root and returns every budget violation, complexity judged
// against `limit` (tests pass 0 to measure any function).
func scan(root string, limit int) ([]violation, error) {
	return scanWithAllowance(root, limit, func(string) int { return maxLength })
}

// scanWithAllowance is scan with an explicit length-allowance resolver.
func scanWithAllowance(root string, limit int, allowance func(string) int) ([]violation, error) {
	fset := token.NewFileSet()
	measured, err := measure(root, fset)
	if err != nil {
		return nil, err
	}
	return evaluate(measured, limit, allowance), nil
}

func report(root string, violations []violation, kind string) {
	byKind := make([]violation, 0, len(violations))
	for _, v := range violations {
		if v.kind == kind {
			byKind = append(byKind, v)
		}
	}
	if len(byKind) == 0 {
		return
	}
	switch kind {
	case "complexity":
		fmt.Fprintf(os.Stderr, "❌ %s: %d function(s) exceed cyclomatic complexity %d:\n\n", root, len(byKind), maxComplexity)
		for _, v := range byKind {
			fmt.Fprintf(os.Stderr, "  %3d  %s:%d  %s\n", v.complexity, v.file, v.line, v.function)
		}
	case "params":
		fmt.Fprintf(os.Stderr, "❌ %s: %d function(s) exceed %d parameters (introduce a parameter struct):\n\n", root, len(byKind), maxParams)
		for _, v := range byKind {
			fmt.Fprintf(os.Stderr, "  %3d  %s:%d  %s\n", v.params, v.file, v.line, v.function)
		}
	case "length":
		fmt.Fprintf(os.Stderr, "❌ %s: %d function(s) exceed their length budget (max %d, ratcheting):\n\n", root, len(byKind), maxLength)
		for _, v := range byKind {
			fmt.Fprintf(os.Stderr, "  %3d  %s:%d  %s\n", v.lines, v.file, v.line, v.function)
		}
	}
	fmt.Fprintln(os.Stderr)
}

func run(root string, baseline map[string]int) int {
	violations, err := scanWithAllowance(root, maxComplexity, func(key string) int {
		if allowed, ok := baseline[key]; ok {
			return allowed
		}
		return maxLength
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	kinds := map[string]bool{}
	for _, v := range violations {
		kinds[v.kind] = true
	}
	for _, kind := range []string{"complexity", "params", "length"} {
		report(root, violations, kind)
	}
	if len(violations) == 0 {
		fmt.Printf("✅ %s: all authored Go functions are within budgets (complexity ≤%d, params ≤%d, length ≤%d or ratcheted)\n", root, maxComplexity, maxParams, maxLength)
		return 0
	}
	return 1
}

func updateBaseline(roots []string, path string) error {
	fset := token.NewFileSet()
	next := map[string]int{}
	for _, root := range roots {
		measured, err := measure(root, fset)
		if err != nil {
			return err
		}
		for key, fn := range measured {
			if fn.lines > maxLength {
				next[key] = fn.lines
			}
		}
	}
	return writeLengthBaseline(path, next)
}

func main() {
	roots := []string{"cmd", "internal"}
	mode := "check"
	var args []string
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--update-length-baseline":
			mode = "update"
		default:
			args = append(args, arg)
		}
	}
	if len(args) > 0 {
		roots = args
	}
	baselinePath := baselineName
	if mode == "update" {
		if err := updateBaseline(roots, baselinePath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Length baseline written: %d function(s) still over %d lines.\n", countEntries(baselinePath), maxLength)
		return
	}
	baseline, err := loadLengthBaseline(baselinePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	exit := 0
	for _, root := range roots {
		if code := run(root, baseline); code != 0 {
			exit = code
		}
	}
	os.Exit(exit)
}

func countEntries(path string) int {
	baseline, err := loadLengthBaseline(path)
	if err != nil {
		return 0
	}
	return len(baseline)
}

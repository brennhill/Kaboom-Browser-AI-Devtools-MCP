// main.go — Ratchets mutable Go package state and exported surfaces.
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

const baselineName = ".go-architecture-baseline.json"

type fileInventory struct {
	MutableGlobals int `json:"mutable_globals"`
	Exports        int `json:"exports"`
}

type inventory map[string]fileInventory

type baselineFile struct {
	Version int       `json:"version"`
	Files   inventory `json:"files"`
}

func ignoredDir(name string) bool {
	switch name {
	case ".git", ".beads", "node_modules", "vendor", "dist", "coverage":
		return true
	default:
		return false
	}
}

func immutableVarValue(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	qualified := owner.Name + "." + selector.Sel.Name
	return qualified == "errors.New" || qualified == "regexp.MustCompile"
}

func scan(root string) (inventory, error) {
	result := make(inventory)
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
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		counts := fileInventory{}
		for _, declaration := range parsed.Decls {
			switch typed := declaration.(type) {
			case *ast.FuncDecl:
				if typed.Name.IsExported() {
					counts.Exports++
				}
			case *ast.GenDecl:
				for _, spec := range typed.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if typed.Tok == token.TYPE {
						typeSpec := spec.(*ast.TypeSpec)
						if typeSpec.Name.IsExported() {
							counts.Exports++
						}
						continue
					}
					if !ok {
						continue
					}
					for _, name := range value.Names {
						if name.IsExported() {
							counts.Exports++
						}
					}
					if typed.Tok == token.VAR {
						immutable := len(value.Values) > 0
						for _, expression := range value.Values {
							immutable = immutable && immutableVarValue(expression)
						}
						if !immutable {
							counts.MutableGlobals += len(value.Names)
						}
					}
				}
			}
		}
		if counts.MutableGlobals == 0 && counts.Exports == 0 {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = counts
		return nil
	})
	return result, err
}

func evaluate(current, baseline inventory) []string {
	violations := make([]string, 0)
	for path, counts := range current {
		allowed := baseline[path]
		if counts.MutableGlobals > allowed.MutableGlobals {
			violations = append(violations, fmt.Sprintf("%s: %d mutable global(s), baseline allows %d", path, counts.MutableGlobals, allowed.MutableGlobals))
		}
		if counts.Exports > allowed.Exports {
			violations = append(violations, fmt.Sprintf("%s: %d export(s), baseline allows %d", path, counts.Exports, allowed.Exports))
		}
	}
	sort.Strings(violations)
	return violations
}

func readBaseline(path string) (inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var baseline baselineFile
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, err
	}
	if baseline.Version != 1 || baseline.Files == nil {
		return nil, fmt.Errorf("unsupported or incomplete Go architecture baseline")
	}
	return baseline.Files, nil
}

func writeBaseline(path string, values inventory) error {
	data, err := json.MarshalIndent(baselineFile{Version: 1, Files: values}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func lowered(current, baseline inventory) inventory {
	result := make(inventory)
	for path, allowed := range baseline {
		counts, exists := current[path]
		if !exists {
			continue
		}
		if counts.MutableGlobals < allowed.MutableGlobals {
			allowed.MutableGlobals = counts.MutableGlobals
		}
		if counts.Exports < allowed.Exports {
			allowed.Exports = counts.Exports
		}
		if allowed.MutableGlobals > 0 || allowed.Exports > 0 {
			result[path] = allowed
		}
	}
	return result
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mode := "check"
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--initialize", "--update":
			mode = strings.TrimPrefix(arg, "--")
		default:
			root = arg
		}
	}
	current, err := scan(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	path := filepath.Join(root, baselineName)
	if mode == "initialize" {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintln(os.Stderr, "baseline already exists; use --update to ratchet it down")
			os.Exit(1)
		}
		err = writeBaseline(path, current)
	} else {
		baseline, readErr := readBaseline(path)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, readErr)
			os.Exit(1)
		}
		if violations := evaluate(current, baseline); len(violations) > 0 {
			fmt.Fprintln(os.Stderr, "Go architecture ratchet failed:")
			for _, violation := range violations {
				fmt.Fprintln(os.Stderr, "- "+violation)
			}
			fmt.Fprintln(os.Stderr, "Reduce the surface, or manually review and edit the baseline for intentional growth.")
			os.Exit(1)
		}
		if mode == "update" {
			err = writeBaseline(path, lowered(current, baseline))
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Go architecture ratchet passed (%d reviewed file surface(s))\n", len(current))
}

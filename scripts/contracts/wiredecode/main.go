// main.go — Requires every decode into a Wire* type to go through internal/wirecodec.
//
// PURPOSE: encoding/json ignores unknown fields, so a peer payload that shares
// no keys with its wire type decodes without error into a zero value. Callers
// then read that zero value as a legitimate empty result. internal/wirecodec
// makes that case an error; this contract makes using it non-optional.
//
// The scan is AST-only — no go/types, no external loader — so it stays within
// the zero-dependency rule and runs in milliseconds. It resolves a destination
// type from a var declaration in the same function, which covers every decode
// shape in this repo and fails open rather than guessing.

package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const exemptionsName = ".wire-decode-exemptions.json"

// decoderPackage is the package whose own decodes are the implementation of
// this rule rather than a violation of it.
const decoderPackage = "wirecodec"

// scannedRoots are the trees holding hand-written Go that talks to peers.
func scannedRoots() []string { return []string{"internal", "cmd"} }

// site is one decode into a wire type.
type site struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Type string `json:"type"`
}

// exemption excuses one wire type in one file, with a reason a reviewer reads.
type exemption struct {
	File   string `json:"file"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

type exemptionsFile struct {
	Comment    string      `json:"_comment"`
	Exemptions []exemption `json:"exemptions"`
}

func ignoredDir(name string) bool {
	switch name {
	case "node_modules", "testdata", "vendor", ".git":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// isWireType reports whether a type name names a wire contract. The Wire prefix
// is the repo's own convention (rule 8), so the scanner keys off it directly.
func isWireType(name string) bool {
	if !strings.HasPrefix(name, "Wire") || len(name) == len("Wire") {
		return false
	}
	next := rune(name[len("Wire")])
	return next >= 'A' && next <= 'Z'
}

// wireTypeName extracts the wire type from a type expression, seeing through
// pointers, slices, arrays and package qualifiers.
func wireTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		if isWireType(typed.Name) {
			return typed.Name
		}
	case *ast.SelectorExpr:
		if isWireType(typed.Sel.Name) {
			return typed.Sel.Name
		}
	case *ast.StarExpr:
		return wireTypeName(typed.X)
	case *ast.ArrayType:
		return wireTypeName(typed.Elt)
	}
	return ""
}

// declaredWireTypes maps local variable names to the wire type they hold.
func declaredWireTypes(fn ast.Node) map[string]string {
	declared := make(map[string]string)
	ast.Inspect(fn, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok || spec.Type == nil {
			return true
		}
		name := wireTypeName(spec.Type)
		if name == "" {
			return true
		}
		for _, ident := range spec.Names {
			declared[ident.Name] = name
		}
		return true
	})
	return declared
}

// jsonDecoderVars collects variables assigned from json.NewDecoder, so a
// decoder held in a local is followed as readily as a chained call.
func jsonDecoderVars(fn ast.Node) map[string]bool {
	decoders := make(map[string]bool)
	ast.Inspect(fn, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		ident, isIdent := assign.Lhs[0].(*ast.Ident)
		if !isIdent || !isJSONCall(assign.Rhs[0], "NewDecoder") {
			return true
		}
		decoders[ident.Name] = true
		return true
	})
	return decoders
}

// isJSONCall reports whether expr is a call to encoding/json's named function.
func isJSONCall(expr ast.Expr, function string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != function {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "json"
}

// destinationWireType resolves the wire type a decode destination refers to.
func destinationWireType(arg ast.Expr, declared map[string]string) string {
	unary, ok := arg.(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return ""
	}
	switch target := unary.X.(type) {
	case *ast.Ident:
		return declared[target.Name]
	case *ast.CompositeLit:
		return wireTypeName(target.Type)
	}
	return ""
}

// isDecodeCall reports whether the call decodes into one of its arguments,
// either directly through encoding/json or through a known local helper that
// takes an untyped destination. Helpers are the shape that hid the perftrace
// defect: `dst any` erases the wire type at the decode itself, so the contract
// has to resolve it at the call site instead.
func isDecodeCall(call *ast.CallExpr, decoders map[string]bool, helpers helperSet) ([]ast.Expr, bool) {
	if isJSONCall(call, "Unmarshal") && len(call.Args) == 2 {
		return call.Args[1:], true
	}
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Decode" && len(call.Args) == 1 {
		switch receiver := selector.X.(type) {
		case *ast.Ident:
			if decoders[receiver.Name] {
				return call.Args, true
			}
		case *ast.CallExpr:
			if isJSONCall(receiver, "NewDecoder") {
				return call.Args, true
			}
		}
	}
	if name := calleeName(call); name != "" && helpers[name] {
		return call.Args, true
	}
	return nil, false
}

// calleeName is the bare function name of a call, whether it is package-local
// or qualified. Bare names can collide across packages; the contract accepts
// that, because the failure mode is reporting one extra site for review rather
// than missing one.
func calleeName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

// helperSet holds the names of functions that decode JSON into an untyped
// destination parameter.
type helperSet map[string]bool

// untypedDestinations returns the parameter names of fn declared as `any` or
// `interface{}`.
func untypedDestinations(fn *ast.FuncDecl) map[string]bool {
	destinations := make(map[string]bool)
	if fn.Type.Params == nil {
		return destinations
	}
	for _, field := range fn.Type.Params.List {
		if !isUntyped(field.Type) {
			continue
		}
		for _, name := range field.Names {
			destinations[name.Name] = true
		}
	}
	return destinations
}

func isUntyped(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "any"
	}
	iface, ok := expr.(*ast.InterfaceType)
	return ok && iface.Methods != nil && len(iface.Methods.List) == 0
}

// collectHelpers records functions that decode into one of their own untyped
// parameters. Such a function becomes the decode path for whatever its callers
// hand it, so the wire check has to follow it.
func collectHelpers(parsed *ast.File, helpers helperSet) {
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		destinations := untypedDestinations(fn)
		if len(destinations) == 0 {
			continue
		}
		decoders := jsonDecoderVars(fn)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			args, decodes := isDecodeCall(call, decoders, helperSet{})
			if !decodes {
				return true
			}
			for _, arg := range args {
				if ident, isIdent := arg.(*ast.Ident); isIdent && destinations[ident.Name] {
					helpers[fn.Name.Name] = true
				}
			}
			return true
		})
	}
}

// sourceFile is one parsed file carried between the two passes, so the tree is
// walked and parsed once rather than once per pass.
type sourceFile struct {
	Relative string
	AST      *ast.File
}

// sitesIn finds the decode sites in one already-parsed file.
func sitesIn(fset *token.FileSet, file sourceFile, helpers helperSet) []site {
	var found []site
	for _, decl := range file.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		declared := declaredWireTypes(fn)
		if len(declared) == 0 {
			continue
		}
		decoders := jsonDecoderVars(fn)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			args, decodes := isDecodeCall(call, decoders, helpers)
			if !decodes {
				return true
			}
			for _, arg := range args {
				if name := destinationWireType(arg, declared); name != "" {
					found = append(found, site{File: file.Relative, Line: fset.Position(call.Pos()).Line, Type: name})
				}
			}
			return true
		})
	}
	return found
}

// collectSources parses every scanned file once, dropping tests and the
// decoder's own package.
func collectSources(fset *token.FileSet, root string) ([]sourceFile, error) {
	var files []sourceFile
	for _, tree := range scannedRoots() {
		base := filepath.Join(root, tree)
		if _, err := os.Stat(base); err != nil {
			// EXPECTED_ABSENCE: the scanner's own tests build fixture trees
			// containing only one of the scanned roots.
			continue
		}
		walkErr := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if ignoredDir(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			parsed, parseErr := parser.ParseFile(fset, path, nil, 0)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", path, parseErr)
			}
			if parsed.Name != nil && parsed.Name.Name == decoderPackage {
				return nil
			}
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, sourceFile{Relative: filepath.ToSlash(relative), AST: parsed})
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	return files, nil
}

// scan resolves decode helpers across the whole tree before looking for sites,
// because a helper is usually declared in a different file from its callers.
func scan(root string) ([]site, error) {
	fset := token.NewFileSet()
	files, err := collectSources(fset, root)
	if err != nil {
		return nil, err
	}

	helpers := make(helperSet)
	for _, file := range files {
		collectHelpers(file.AST, helpers)
	}

	var sites []site
	for _, file := range files {
		sites = append(sites, sitesIn(fset, file, helpers)...)
	}
	sort.Slice(sites, func(i, j int) bool {
		if sites[i].File != sites[j].File {
			return sites[i].File < sites[j].File
		}
		return sites[i].Line < sites[j].Line
	})
	return sites, nil
}

func key(file, wireType string) string { return file + "#" + wireType }

// evaluate reports unexempted decode sites and exemptions that no longer match
// one. A stale exemption is a violation because an allow-list that outlives its
// code silently re-permits the next occurrence.
func evaluate(sites []site, exemptions []exemption) []string {
	excused := make(map[string]exemption, len(exemptions))
	for _, entry := range exemptions {
		excused[key(entry.File, entry.Type)] = entry
	}

	var violations []string
	matched := make(map[string]bool, len(exemptions))
	for _, found := range sites {
		id := key(found.File, found.Type)
		entry, isExcused := excused[id]
		if !isExcused {
			violations = append(violations, fmt.Sprintf(
				"%s:%d decodes %s directly; route it through internal/wirecodec so an error envelope cannot become a zero value",
				found.File, found.Line, found.Type))
			continue
		}
		matched[id] = true
		if strings.TrimSpace(entry.Reason) == "" {
			violations = append(violations, fmt.Sprintf(
				"%s exempts %s with no reason; every exemption needs one a reviewer can weigh", found.File, found.Type))
		}
	}

	stale := make([]string, 0)
	for id, entry := range excused {
		if !matched[id] {
			stale = append(stale, fmt.Sprintf(
				"%s no longer decodes %s, so its exemption is stale; delete it from %s", entry.File, entry.Type, exemptionsName))
		}
	}
	sort.Strings(stale)
	return append(violations, stale...)
}

func readExemptions(path string) ([]exemption, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// EXPECTED_ABSENCE: a repo with no exemptions has no file, and an
			// empty allow-list is the strictest possible state.
			return nil, nil
		}
		return nil, err
	}
	var file exemptionsFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse %s: %w", exemptionsName, err)
	}
	return file.Exemptions, nil
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wire-decode contract:", err)
		os.Exit(1)
	}
	sites, err := scan(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wire-decode contract:", err)
		os.Exit(1)
	}
	exemptions, err := readExemptions(filepath.Join(root, exemptionsName))
	if err != nil {
		fmt.Fprintln(os.Stderr, "wire-decode contract:", err)
		os.Exit(1)
	}
	violations := evaluate(sites, exemptions)
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: %d wire-decode violation(s)\n", len(violations))
		for _, violation := range violations {
			fmt.Fprintln(os.Stderr, "  -", violation)
		}
		os.Exit(1)
	}
	fmt.Printf("OK: %d wire decode site(s), all routed through internal/wirecodec or explicitly exempt\n", len(sites))
}

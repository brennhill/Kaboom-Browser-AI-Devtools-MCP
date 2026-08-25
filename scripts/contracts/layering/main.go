// main.go — Enforces the Go layering contract: dependency direction, interface size, interface ownership.
//
// Matrix (hard-fail, all rules measured clean before adoption):
//
//	internal/**        ↛ cmd/**            (composition root is outermost)
//	internal/types      ↛ internal/**       (payload contracts are the innermost leaf)
//	internal/mcp        ↛ internal/tools/**, internal/capture/** (protocol ignorant of domain and ports)
//	internal/schema/**  ↮ internal/tools/** (contract layer and tool layer are siblings)
//	capture.NewCapture   only from cmd/browser-agent (ports are wired by the composition root)
//
// Interfaces:
//
//	size      — exported interface ≤ 7 methods (hard; interface segregation)
//	ownership — exported interfaces must not be producer-owned (only production
//	            implementations live in the declaring package) or dead (no
//	            production implementation anywhere). Same-package test fakes
//	            do not count as implementations. Ratcheting baseline:
//	            .interface-baseline.json, entries only removed, never added.
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
	modulePrefix        = "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/"
	maxInterfaceMethods = 7
	baselineName        = ".interface-baseline.json"
)

type ifaceInfo struct {
	root    string
	pkg     string
	file    string
	name    string
	methods int
}

func (i ifaceInfo) key() string { return i.pkg + "/" + i.file + ":" + i.name }

// absPath resolves the interface's source file against its scan root.
func (i ifaceInfo) absPath() string { return filepath.Join(i.root, i.pkg, i.file) }

type treeScan struct {
	imports    map[string]map[string]bool // "pkg/file.go" -> internal import paths
	prodSets   map[string]map[string]bool // "pkg/TypeName" -> production method names
	testSets   map[string]map[string]bool // "pkg/TypeName" -> test-fake method names
	interfaces []ifaceInfo
}

// ignoredDir skips trees that are not this checkout's hand-written source.
// compositionRootDir returns the directory allowed to construct capture ports:
// <root>/browser-agent when scanning the cmd tree, <root>/cmd/browser-agent
// when scanning a whole checkout (tests scan fixture trees).
func compositionRootDir(root string) string {
	if filepath.Base(root) == "cmd" {
		return filepath.Join(root, "browser-agent")
	}
	return filepath.Join(root, "cmd", "browser-agent")
}

// isPkgOrSub reports whether path is exactly pkg or lives under pkg/.
func isPkgOrSub(path, pkg string) bool {
	return path == pkg || strings.HasPrefix(path, pkg+"/")
}

func ignoredDir(name string) bool {
	switch name {
	case "node_modules", "vendor", "dist", "build", "coverage", "generated", "testdata", "testpages", "terminal_assets":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// scanTree parses every production Go file under root and records imports,
// interface declarations, and concrete method sets. Paths are root-relative.
func scanTree(root string) (*treeScan, error) {
	scanRoot = root
	scan := &treeScan{
		imports:  map[string]map[string]bool{},
		prodSets: map[string]map[string]bool{},
		testSets: map[string]map[string]bool{},
	}
	fset := token.NewFileSet()
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
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		isTestFile := strings.HasSuffix(entry.Name(), "_test.go")
		if isTestFile {
			// Interfaces and imports come from production files only; test
			// files contribute tagged method sets so cross-language seams
			// (production impls outside Go) are not misread as dead contracts.
			parsed, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return nil
			}
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			pkgDir := filepath.ToSlash(filepath.Dir(filepath.ToSlash(relative)))
			for _, declaration := range parsed.Decls {
				fd, ok := declaration.(*ast.FuncDecl)
				if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
					continue
				}
				if typeName := receiverTypeName(fd.Recv.List[0].Type); typeName != "" {
					merge(scan.testSets, pkgDir, typeName, map[string]bool{fd.Name.Name: true})
				}
			}
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		pkgDir := filepath.ToSlash(filepath.Dir(relative))
		fileKey := relative
		scan.imports[fileKey] = map[string]bool{}
		for _, imp := range parsed.Imports {
			scan.imports[fileKey][strings.Trim(imp.Path.Value, `"`)] = true
		}
		for _, declaration := range parsed.Decls {
			gd, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if it, ok := ts.Type.(*ast.InterfaceType); ok {
					if ts.Name.IsExported() {
						count := 0
						for _, field := range it.Methods.List {
							if len(field.Names) > 0 {
								count++
							}
						}
						scan.interfaces = append(scan.interfaces, ifaceInfo{root, pkgDir, filepath.Base(relative), ts.Name.Name, count})
					}
					continue
				}
				if st, ok := ts.Type.(*ast.StructType); ok && ts.Name.IsExported() {
					set := map[string]bool{}
					for _, field := range st.Fields.List {
						if len(field.Names) == 1 {
							set[field.Names[0].Name] = true
						}
					}
					if len(set) > 0 {
						merge(scan.prodSets, pkgDir, ts.Name.Name, set)
					}
				}
			}
		}
		for _, declaration := range parsed.Decls {
			fd, ok := declaration.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			if fd.Recv != nil && len(fd.Recv.List) > 0 {
				if typeName := receiverTypeName(fd.Recv.List[0].Type); typeName != "" {
					merge(scan.prodSets, pkgDir, typeName, map[string]bool{fd.Name.Name: true})
				}
			}
			// D5: capture.NewCapture outside the composition root.
			if filepath.Dir(path) != compositionRootDir(root) {
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewCapture" {
						if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "capture" {
							compositionRootBreaches = append(compositionRootBreaches, fileKey)
						}
					}
					return true
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return scan, nil
}

func merge(sets map[string]map[string]bool, pkgDir, typeName string, methods map[string]bool) {
	key := pkgDir + "/" + typeName
	if sets[key] == nil {
		sets[key] = map[string]bool{}
	}
	for m := range methods {
		sets[key][m] = true
	}
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

var compositionRootBreaches []string

// evaluateMatrix checks every dependency rule and returns violation lines.
func evaluateMatrix(scan *treeScan) []string {
	var violations []string
	for file, imports := range scan.imports {
		for imp := range imports {
			internal, ok := strings.CutPrefix(imp, modulePrefix)
			if !ok || !strings.HasPrefix(internal, "internal/") && !strings.HasPrefix(internal, "cmd/") {
				continue
			}
			switch {
			case strings.HasPrefix(file, "internal/") && strings.HasPrefix(internal, "cmd/"):
				violations = append(violations, fmt.Sprintf("internal->cmd: %s imports %s", file, imp))
			case file == "internal/types/types.go" || strings.HasPrefix(file, "internal/types/") && !strings.Contains(strings.TrimPrefix(file, "internal/types/"), "/"):
				if strings.HasPrefix(internal, "internal/") && imp != modulePrefix+"internal/types" {
					violations = append(violations, fmt.Sprintf("types-leaf: %s imports %s", file, imp))
				}
			case strings.HasPrefix(file, "internal/mcp/") && (isPkgOrSub(internal, "internal/tools") || isPkgOrSub(internal, "internal/capture")):
				violations = append(violations, fmt.Sprintf("mcp->domain: %s imports %s", file, imp))
			case strings.HasPrefix(file, "internal/schema/") && strings.HasPrefix(internal, "internal/tools/"):
				violations = append(violations, fmt.Sprintf("schema->tools: %s imports %s", file, imp))
			case strings.HasPrefix(file, "internal/tools/") && strings.HasPrefix(internal, "internal/schema/"):
				violations = append(violations, fmt.Sprintf("tools->schema: %s imports %s", file, imp))
			}
		}
	}
	for _, file := range compositionRootBreaches {
		violations = append(violations, fmt.Sprintf("composition-root: %s calls capture.NewCapture", file))
	}
	sort.Strings(violations)
	return violations
}

// interfaceMethods returns the method names of an interface declaration by key.
// Interfaces are stored with counts only; method names are re-parsed from the file.
func interfaceMethodNames(scan *treeScan) map[string][]string {
	result := map[string][]string{}
	fset := token.NewFileSet()
	for _, i := range scan.interfaces {
		path := i.absPath()
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}
		for _, declaration := range parsed.Decls {
			gd, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name.Name != i.name {
					continue
				}
				if it, ok := ts.Type.(*ast.InterfaceType); ok {
					var names []string
					for _, field := range it.Methods.List {
						if len(field.Names) > 0 {
							names = append(names, field.Names[0].Name)
						}
					}
					result[i.key()] = names
				}
			}
		}
	}
	return result
}

var scanRoot string

func implementsAll(set map[string]bool, methods []string) bool {
	if len(methods) == 0 {
		return false // unparseable contract: judge nothing as implementing it
	}
	for _, m := range methods {
		if !set[m] {
			return false
		}
	}
	return true
}

type ownership struct {
	key  string
	kind string
}

// ownershipViolations classifies each exported interface as producer-owned or dead.
func ownershipViolations(scan *treeScan) []ownership {
	methodNames := interfaceMethodNames(scan)
	var result []ownership
	for _, i := range scan.interfaces {
		if i.methods == 0 {
			continue
		}
		want := methodNames[i.key()]
		localProd, elsewhereProd, anyImpl := 0, 0, 0
		count := func(sets map[string]map[string]bool, isProd bool) {
			for key, set := range sets {
				if !implementsAll(set, want) {
					continue
				}
				anyImpl++
				pkg := key[:strings.LastIndex(key, "/")] // strip the type name
				if pkg == i.pkg {
					if isProd {
						localProd++
					}
				} else if isProd {
					elsewhereProd++
				}
			}
		}
		count(scan.prodSets, true)
		count(scan.testSets, false)
		switch {
		case localProd > 0 && elsewhereProd == 0:
			result = append(result, ownership{i.key(), "producer-owned"})
		case anyImpl == 0:
			result = append(result, ownership{i.key(), "dead"})
		}
	}
	sort.Slice(result, func(a, b int) bool { return result[a].key < result[b].key })
	return result
}

// evaluateOwnership returns violations not covered by the baseline. Stale
// baseline entries are ignored (fixing and forgetting to prune is fine).
func evaluateOwnership(current []ownership, baseline map[string]string) []string {
	var violations []string
	for _, o := range current {
		if _, allowed := baseline[o.key]; !allowed {
			violations = append(violations, o.kind+": "+o.key)
		}
	}
	return violations
}

func readBaseline(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	var parsed struct {
		Version    int               `json:"version"`
		MaxMethods int               `json:"max_methods"`
		Interfaces map[string]string `json:"interfaces"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if parsed.Version != 1 {
		return nil, fmt.Errorf("unsupported interface baseline version in %s", path)
	}
	return parsed.Interfaces, nil
}

func writeBaseline(path string, current []ownership) error {
	entries := map[string]string{}
	for _, o := range current {
		entries[o.key] = o.kind
	}
	data, err := json.MarshalIndent(struct {
		Version    int               `json:"version"`
		MaxMethods int               `json:"max_methods"`
		Interfaces map[string]string `json:"interfaces"`
	}{1, maxInterfaceMethods, entries}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// scanAll merges the scans of every root so cross-tree implementations are
// visible to interface-ownership classification.
func scanAll(roots []string) (*treeScan, error) {
	merged := &treeScan{
		imports:  map[string]map[string]bool{},
		prodSets: map[string]map[string]bool{},
		testSets: map[string]map[string]bool{},
	}
	for _, root := range roots {
		scan, err := scanTree(root)
		if err != nil {
			return nil, err
		}
		for k, v := range scan.imports {
			merged.imports[k] = v
		}
		for k, v := range scan.prodSets {
			merged.prodSets[k] = v
		}
		for k, v := range scan.testSets {
			merged.testSets[k] = v
		}
		merged.interfaces = append(merged.interfaces, scan.interfaces...)
	}
	return merged, nil
}

func main() {
	roots := []string{"cmd", "internal"}
	mode := "check"
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--update":
			mode = "update"
		default:
			roots = []string{arg}
		}
	}
	if mode == "update" {
		merged, err := scanAll(roots)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		all := ownershipViolations(merged)
		if err := writeBaseline(baselineName, all); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Interface ownership baseline written: %d entr(ies).\n", len(all))
		return
	}

	baseline, err := readBaseline(baselineName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var matrixFailures, sizeFailures, ownershipFailures int
	merged, err := scanAll(roots)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if violations := evaluateMatrix(merged); len(violations) > 0 {
		matrixFailures = len(violations)
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "❌ layering: %s\n", v)
		}
	}
	for _, i := range merged.interfaces {
		if i.methods > maxInterfaceMethods {
			sizeFailures++
			fmt.Fprintf(os.Stderr, "❌ interface size: %s has %d methods (max %d)\n", i.key(), i.methods, maxInterfaceMethods)
		}
	}
	if violations := evaluateOwnership(ownershipViolations(merged), baseline); len(violations) > 0 {
		ownershipFailures = len(violations)
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "❌ interface ownership: %s\n", v)
		}
	}
	if matrixFailures+sizeFailures+ownershipFailures == 0 {
		fmt.Println("✅ Go layering contract holds (dependency matrix, interface size, interface ownership)")
		return
	}
	fmt.Fprintln(os.Stderr, "\nDependency and size rules are hard limits; ownership entries ratchet via .interface-baseline.json (--update to re-freeze after fixing).")
	os.Exit(1)
}

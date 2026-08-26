// main.go — Enforces the Go layering contract: dependency direction, interface size, interface ownership.
//
// Matrix (hard-fail, evaluated on package IMPORT PATHS, never on walk-relative
// file prefixes — the original prefix form silently matched nothing under the
// default `cmd internal` roots and was found in review):
//
//	internal/**        ↛ cmd/**            (composition root is outermost)
//	internal/types      ↛ internal/**       (payload contracts are the innermost leaf)
//	internal/mcp        ↛ internal/tools/**, internal/capture/** (protocol ignorant of domain and ports)
//	internal/schema/**  ↮ internal/tools/** (contract layer and tool layer are siblings)
//	capture.NewCapture   only from cmd/browser-agent (ports are wired by the
//	                     composition root; detection resolves import ALIASES
//	                     and package-level var initializers, not the literal
//	                     identifier "capture")
//
// Interfaces:
//
//	size      — exported interface ≤ 7 methods (hard; interface segregation)
//	ownership — exported interfaces must not be producer-owned (only production
//	            implementations live in the declaring package) or dead (no
//	            production implementation anywhere). Same-package test fakes
//	            do not count as implementations. Generic receivers
//	            (Store[T]) and within-package embedding contribute promoted
//	            methods. Ratcheting baseline: .interface-baseline.json.
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
	// repo is the checkout root the pkg path is relative to (NOT the scan
	// root: joining scan root "internal" with repo-relative "internal/x"
	// duplicated the prefix and made every such interface unparseable).
	repo    string
	pkg     string
	file    string
	name    string
	methods int
}

func (i ifaceInfo) key() string { return i.pkg + "/" + i.file + ":" + i.name }

// absPath resolves the interface's source file against its checkout root.
func (i ifaceInfo) absPath() string { return filepath.Join(i.repo, i.pkg, i.file) }

type treeScan struct {
	// imports maps file -> import path set (production files only).
	imports map[string]map[string]bool
	// alias maps file -> local identifier (alias or package name) -> import path.
	alias map[string]map[string]string
	// pkgOf maps file -> package import path (modulePrefix + repo-relative dir).
	pkgOf map[string]string
	// prodSets maps "pkg/TypeName" -> production method names.
	prodSets map[string]map[string]bool
	// testSets maps "pkg/TypeName" -> test-fake method names.
	testSets map[string]map[string]bool
	// embedded maps "pkg/TypeName" -> embedded type names (same package).
	embedded map[string][]string
	// structs records declared struct type names per package (for embedding resolution).
	structs map[string]bool
	// compositionRootBreaches lists files outside cmd/browser-agent that
	// construct capture.NewCapture (via any import alias).
	compositionRootBreaches []string
	interfaces              []ifaceInfo
}

// ignoredDir skips trees that are not this checkout's hand-written source.
func ignoredDir(name string) bool {
	switch name {
	case "node_modules", "vendor", "dist", "build", "coverage", "generated", "testdata", "testpages", "terminal_assets":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// isPkgOrSub reports whether path is exactly pkg or lives under pkg/.
func isPkgOrSub(path, pkg string) bool {
	return path == pkg || strings.HasPrefix(path, pkg+"/")
}

// repoRoot walks up from dir until a go.mod is found; falls back to dir.
func repoRoot(dir string) string {
	current, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return dir
		}
		current = parent
	}
}

// importLocalName returns the identifier a file uses for an import path.
func importLocalName(imp *ast.ImportSpec) (string, bool) {
	if imp.Name != nil {
		if imp.Name.Name == "_" || imp.Name.Name == "." {
			return "", false
		}
		return imp.Name.Name, true
	}
	clean := strings.Trim(imp.Path.Value, `"`)
	segments := strings.Split(clean, "/")
	if len(segments) == 0 {
		return "", false
	}
	return segments[len(segments)-1], true
}

// captureConstructorRef walks a node for X.NewCapture where X resolves, through
// the file's import alias map, to the capture package.
func captureConstructorRef(node ast.Node, aliases map[string]string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "NewCapture" {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if path, bound := aliases[ident.Name]; bound && path == modulePrefix+"internal/capture" {
			found = true
			return false
		}
		return true
	})
	return found
}

// scanTree parses every Go file under root. File keys and package paths are
// REPO-RELATIVE (resolved via go.mod), so rules fire identically whether root
// is a checkout, "cmd", or "internal".
func scanTree(root string) (*treeScan, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	repo := repoRoot(absRoot)
	scan := &treeScan{
		imports:  map[string]map[string]bool{},
		alias:    map[string]map[string]string{},
		pkgOf:    map[string]string{},
		prodSets: map[string]map[string]bool{},
		testSets: map[string]map[string]bool{},
		embedded: map[string][]string{},
		structs:  map[string]bool{},
	}
	fset := token.NewFileSet()
	err = filepath.WalkDir(absRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != absRoot && ignoredDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}
		repoRel, relErr := filepath.Rel(repo, path)
		if relErr != nil {
			return relErr
		}
		fileKey := filepath.ToSlash(repoRel)
		pkgDir := filepath.ToSlash(filepath.Dir(fileKey))
		pkgPath := modulePrefix + strings.TrimSuffix(pkgDir, "/")
		isTest := strings.HasSuffix(entry.Name(), "_test.go")

		if isTest {
			// Test files contribute method sets (tagged) so cross-boundary
			// seams keep contracts alive; imports and interfaces come from
			// production files only.
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

		scan.imports[fileKey] = map[string]bool{}
		scan.alias[fileKey] = map[string]string{}
		for _, imp := range parsed.Imports {
			clean := strings.Trim(imp.Path.Value, `"`)
			scan.imports[fileKey][clean] = true
			if local, ok := importLocalName(imp); ok {
				scan.alias[fileKey][local] = clean
			}
		}
		scan.pkgOf[fileKey] = pkgPath

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
						scan.interfaces = append(scan.interfaces, ifaceInfo{repo, pkgDir, filepath.Base(fileKey), ts.Name.Name, count})
					}
					continue
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					key := pkgDir + "/" + ts.Name.Name
					scan.structs[key] = true
					for _, field := range st.Fields.List {
						switch {
						case len(field.Names) == 1:
							merge(scan.prodSets, pkgDir, ts.Name.Name, map[string]bool{field.Names[0].Name: true})
						case len(field.Names) == 0:
							// Embedded field: promoted methods are unioned in
							// propagateEmbedded once every type is known.
							if name := embeddedTypeName(field.Type); name != "" {
								scan.embedded[key] = append(scan.embedded[key], name)
							}
						}
					}
				}
			}
		}
		for _, declaration := range parsed.Decls {
			fd, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fd.Recv != nil && len(fd.Recv.List) > 0 {
				if typeName := receiverTypeName(fd.Recv.List[0].Type); typeName != "" {
					merge(scan.prodSets, pkgDir, typeName, map[string]bool{fd.Name.Name: true})
				}
			}
		}
		// Package-level var/const initializers are constructor call sites too.
		for _, declaration := range parsed.Decls {
			gd, ok := declaration.(*ast.GenDecl)
			if !ok || (gd.Tok.String() != "var" && gd.Tok.String() != "const") {
				continue
			}
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for _, value := range vs.Values {
						if captureConstructorRef(value, scan.alias[fileKey]) && !isCompositionRootPkg(pkgPath) {
							scan.compositionRootBreaches = append(scan.compositionRootBreaches, fileKey)
						}
					}
				}
			}
		}
		if !isCompositionRootPkg(pkgPath) {
			for _, declaration := range parsed.Decls {
				fd, ok := declaration.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				if captureConstructorRef(fd.Body, scan.alias[fileKey]) {
					scan.compositionRootBreaches = append(scan.compositionRootBreaches, fileKey)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	propagateEmbedded(scan)
	return scan, nil
}

func isCompositionRootPkg(pkgPath string) bool {
	return pkgPath == modulePrefix+"cmd/browser-agent"
}

// embeddedTypeName extracts the type name of an embedded struct field for
// within-package embedding resolution (pointers unwrapped).
func embeddedTypeName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// propagateEmbedded unions promoted method sets for same-package embedded
// fields until a fixpoint (embedding chains resolve in order).
func propagateEmbedded(scan *treeScan) {
	for changed := true; changed; {
		changed = false
		for key, names := range scan.embedded {
			for _, name := range names {
				embeddedKey := filepath.ToSlash(filepath.Dir(key)) + "/" + name
				if !scan.structs[embeddedKey] {
					continue
				}
				for method := range scan.prodSets[embeddedKey] {
					if !scan.prodSets[key][method] {
						merge(scan.prodSets, filepath.ToSlash(filepath.Dir(key)), filepath.Base(key), map[string]bool{method: true})
						changed = true
					}
				}
			}
		}
	}
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

// receiverTypeName unwraps pointer and generic (IndexExpr/IndexListExpr)
// receivers: func (s *Store[T]) M() belongs to Store.
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	}
	return ""
}

// evaluateMatrix checks every dependency rule on package import paths.
func evaluateMatrix(scan *treeScan) []string {
	var violations []string
	for file, imports := range scan.imports {
		srcPkg := scan.pkgOf[file]
		for imp := range imports {
			internal, ok := strings.CutPrefix(imp, modulePrefix)
			if !ok {
				continue
			}
			if !strings.HasPrefix(internal, "internal/") && !strings.HasPrefix(internal, "cmd/") {
				continue
			}
			switch {
			case strings.HasPrefix(srcPkg, modulePrefix+"internal/") && strings.HasPrefix(internal, "cmd/"):
				violations = append(violations, fmt.Sprintf("internal->cmd: %s imports %s", file, imp))
			case isPkgOrSub(srcPkg, modulePrefix+"internal/mcp") && (isPkgOrSub(internal, "internal/tools") || isPkgOrSub(internal, "internal/capture")):
				violations = append(violations, fmt.Sprintf("mcp->domain: %s imports %s", file, imp))
			case isPkgOrSub(srcPkg, modulePrefix+"internal/schema") && isPkgOrSub(internal, "internal/tools"):
				violations = append(violations, fmt.Sprintf("schema->tools: %s imports %s", file, imp))
			}
		}
	}
	for _, violation := range leafImportViolations(scan) {
		violations = append(violations, violation)
	}
	for _, file := range scan.compositionRootBreaches {
		violations = append(violations, fmt.Sprintf("composition-root: %s constructs capture.NewCapture", file))
	}
	sort.Strings(violations)
	return violations
}

// leafImportViolations reports internal/types files importing internal
// packages that are not leaves (a leaf has no internal imports of its own).
func leafImportViolations(scan *treeScan) []string {
	// Aggregate each package's internal imports across its files.
	pkgImports := map[string]map[string]bool{}
	for file, imports := range scan.imports {
		srcPkg := scan.pkgOf[file]
		if pkgImports[srcPkg] == nil {
			pkgImports[srcPkg] = map[string]bool{}
		}
		for imp := range imports {
			if internal, ok := strings.CutPrefix(imp, modulePrefix); ok && strings.HasPrefix(internal, "internal/") {
				pkgImports[srcPkg][imp] = true
			}
		}
	}
	isLeaf := func(pkg string) bool { return len(pkgImports[pkg]) == 0 }

	var violations []string
	for file, imports := range scan.imports {
		if !isPkgOrSub(scan.pkgOf[file], modulePrefix+"internal/types") {
			continue
		}
		for imp := range imports {
			internal, ok := strings.CutPrefix(imp, modulePrefix)
			if !ok || !strings.HasPrefix(internal, "internal/") || isPkgOrSub(internal, "internal/types") {
				continue
			}
			if !isLeaf(imp) {
				violations = append(violations, fmt.Sprintf("types-leaf: %s imports %s (not a leaf package)", file, imp))
			}
		}
	}
	return violations
}

// interfaceMethodNames returns the method names of each interface by key.
func interfaceMethodNames(scan *treeScan) map[string][]string {
	result := map[string][]string{}
	fset := token.NewFileSet()
	for _, i := range scan.interfaces {
		if _, done := result[i.key()]; done {
			continue
		}
		parsed, err := parser.ParseFile(fset, i.absPath(), nil, 0)
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
		alias:    map[string]map[string]string{},
		pkgOf:    map[string]string{},
		prodSets: map[string]map[string]bool{},
		testSets: map[string]map[string]bool{},
		embedded: map[string][]string{},
		structs:  map[string]bool{},
	}
	for _, root := range roots {
		scan, err := scanTree(root)
		if err != nil {
			return nil, err
		}
		for k, v := range scan.imports {
			merged.imports[k] = v
		}
		for k, v := range scan.alias {
			merged.alias[k] = v
		}
		for k, v := range scan.pkgOf {
			merged.pkgOf[k] = v
		}
		for k, v := range scan.prodSets {
			merged.prodSets[k] = v
		}
		for k, v := range scan.testSets {
			merged.testSets[k] = v
		}
		for k, v := range scan.embedded {
			merged.embedded[k] = v
		}
		for k, v := range scan.structs {
			merged.structs[k] = v
		}
		merged.interfaces = append(merged.interfaces, scan.interfaces...)
		merged.compositionRootBreaches = append(merged.compositionRootBreaches, scan.compositionRootBreaches...)
	}
	return merged, nil
}

func main() {
	roots := []string{}
	mode := "check"
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--update":
			mode = "update"
		default:
			roots = append(roots, arg)
		}
	}
	if len(roots) == 0 {
		roots = []string{"cmd", "internal"}
	}
	baselinePath := baselineName
	if mode == "update" {
		merged, err := scanAll(roots)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		all := ownershipViolations(merged)
		if err := writeBaseline(baselinePath, all); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Interface ownership baseline written: %d entr(ies).\n", len(all))
		return
	}

	baseline, err := readBaseline(baselinePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	merged, err := scanAll(roots)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	matrixFailures, sizeFailures, ownershipFailures := 0, 0, 0
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

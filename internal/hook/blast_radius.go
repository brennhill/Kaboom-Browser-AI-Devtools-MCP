// blast_radius.go — Blast radius analysis for PostToolUse hooks.
// Detects files that import the edited file and warns about downstream impact.

package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/hook/projectscan"
)

const (
	graphCacheFile    = "import_graph.json"
	graphCacheMaxAge  = 5 * time.Minute
	maxImportersShown = 10
	maxFilesForGraph  = 1000
)

// BlastRadiusResult holds the findings from blast radius analysis.
type BlastRadiusResult struct {
	Context   string
	Importers []string
	File      string
}

// FormatContext returns the additionalContext string for the hook output.
func (r *BlastRadiusResult) FormatContext() string {
	return r.Context
}

// ImportGraph maps each file to the list of files that import it.
type ImportGraph struct {
	// Importers maps "imported file" -> list of "files that import it".
	Importers map[string][]string `json:"importers"`
	BuiltAt   time.Time           `json:"built_at"`
}

// RunBlastRadius checks if the edited file is imported by other files.
// Returns nil if no blast radius detected or if the tool is not an edit.
func RunBlastRadius(input Input, projectRoot, sessionDir string) *BlastRadiusResult {
	if !isEditTool(input.ToolName) {
		return nil
	}

	fields := input.ParseToolInput()
	filePath := fields.FilePath
	if filePath == "" || projectRoot == "" {
		return nil
	}

	// Make path relative to project root.
	relPath, err := filepath.Rel(projectRoot, filePath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return nil
	}

	// Check if edit touches an exported symbol (skip internal-only changes).
	if !looksExported(fields.NewString, filePath) {
		return nil
	}

	graph := loadOrBuildGraph(projectRoot, sessionDir)
	if graph == nil {
		return nil
	}

	importers := graph.Importers[relPath]
	if len(importers) == 0 {
		return nil
	}

	// Check which importers are already in the session.
	var inSession, notInSession []string
	if sessionDir != "" {
		for _, imp := range importers {
			absImp := filepath.Join(projectRoot, imp)
			if wasRead, _ := WasFileRead(sessionDir, absImp); wasRead {
				inSession = append(inSession, imp)
			} else {
				notInSession = append(notInSession, imp)
			}
		}
	} else {
		notInSession = importers
	}

	return formatBlastResult(relPath, importers, inSession, notInSession)
}

func formatBlastResult(file string, all, inSession, notInSession []string) *BlastRadiusResult {
	n := len(all)
	var b strings.Builder

	// Graduated injection based on count.
	switch {
	case n >= 16:
		fmt.Fprintf(&b, "[Blast Radius] CRITICAL: %s is imported by %d files. Changes here have wide impact.", file, n)
	case n >= 6:
		fmt.Fprintf(&b, "[Blast Radius] WARNING: %s is imported by %d files.", file, n)
	default:
		fmt.Fprintf(&b, "[Blast Radius] %s is imported by %d file(s).", file, n)
	}

	// Show importers, highlighting those already in session.
	shown := 0
	if len(inSession) > 0 {
		b.WriteString("\n  Already in session:")
		for _, f := range inSession {
			if shown >= maxImportersShown {
				fmt.Fprintf(&b, "\n    ... and %d more", len(inSession)-shown)
				break
			}
			fmt.Fprintf(&b, "\n    %s (already read)", f)
			shown++
		}
	}
	if len(notInSession) > 0 {
		b.WriteString("\n  Not yet reviewed:")
		shown = 0
		for _, f := range notInSession {
			if shown >= maxImportersShown {
				fmt.Fprintf(&b, "\n    ... and %d more", len(notInSession)-shown)
				break
			}
			fmt.Fprintf(&b, "\n    %s", f)
			shown++
		}
	}

	return &BlastRadiusResult{
		Context:   b.String(),
		Importers: all,
		File:      file,
	}
}

// looksExported checks if the edit content appears to modify exported/public symbols.
func looksExported(newContent, filePath string) bool {
	if newContent == "" {
		return true // Can't tell, assume exported.
	}

	ext := filepath.Ext(filePath)
	switch ext {
	case ".go":
		return goHasExported(newContent)
	case ".ts", ".tsx", ".js", ".jsx":
		return tsHasExported(newContent)
	case ".py":
		return pyHasExported(newContent)
	case ".rs":
		return rsHasExported(newContent)
	}
	return true // Unknown language, assume exported.
}

var (
	goExportedPattern = regexp.MustCompile(`\b(func|type|var|const)\s+[A-Z]`)
	tsExportPattern   = regexp.MustCompile(`\b(export\s+(function|class|const|let|var|type|interface|enum|default))`)
	pyPublicPattern   = regexp.MustCompile(`^(def|class)\s+[a-zA-Z]`)
	pyDunderPattern   = regexp.MustCompile(`^(def|class)\s+_`)
	rsPublicPattern   = regexp.MustCompile(`\bpub\s+(fn|struct|enum|trait|mod|type|const|static)`)
)

func goHasExported(content string) bool {
	return goExportedPattern.MatchString(content)
}

func tsHasExported(content string) bool {
	return tsExportPattern.MatchString(content)
}

func pyHasExported(content string) bool {
	// Python: public if not prefixed with underscore.
	if pyDunderPattern.MatchString(content) {
		return false
	}
	return pyPublicPattern.MatchString(content)
}

func rsHasExported(content string) bool {
	return rsPublicPattern.MatchString(content)
}

// isEditTool returns true if the tool name represents a file edit/write.
func isEditTool(name string) bool {
	switch name {
	case "Edit", "Write", "write_file", "replace_in_file", "edit_file":
		return true
	}
	return false
}

// --- Import graph building ---

func loadOrBuildGraph(projectRoot, sessionDir string) *ImportGraph {
	// Try loading from session cache.
	if sessionDir != "" {
		if g := loadCachedGraph(sessionDir); g != nil {
			return g
		}
	}

	g := buildImportGraph(projectRoot)
	if g == nil {
		return nil
	}

	// Cache in session dir.
	if sessionDir != "" {
		saveCachedGraph(sessionDir, g)
	}
	return g
}

func loadCachedGraph(sessionDir string) *ImportGraph {
	path := filepath.Join(sessionDir, graphCacheFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var g ImportGraph
	if json.Unmarshal(data, &g) != nil {
		return nil
	}
	if time.Since(g.BuiltAt) > graphCacheMaxAge {
		return nil
	}
	return &g
}

func saveCachedGraph(sessionDir string, g *ImportGraph) {
	data, err := json.Marshal(g)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(sessionDir, graphCacheFile), data, 0o644)
}

// importResolver memoizes the filesystem lookups that import extraction repeats
// across files. Every importer of a package asked the disk for that package's
// file list, and every relative TypeScript import re-probed the same candidate
// extensions, so one graph build issued thousands of syscalls for a few hundred
// distinct answers. Resolution is pure per (project, path), so caching it is
// behaviour-preserving; the walk is I/O bound, and this is the I/O.
type importResolver struct {
	projectRoot string
	goModPath   string
	pkgFiles    map[string][]string // package directory -> relative .go files
	statted     map[string]bool     // absolute path -> os.Stat succeeded

	// readDirCalls and statCalls count syscalls actually issued (cache misses).
	// The hook's latency budget is a syscall budget, so the regression test
	// asserts on these rather than on wall-clock, which varies with page-cache
	// warmth and machine load.
	readDirCalls int
	statCalls    int
}

func newImportResolver(projectRoot string) *importResolver {
	return &importResolver{
		projectRoot: projectRoot,
		goModPath:   detectGoModulePath(projectRoot),
		pkgFiles:    make(map[string][]string),
		statted:     make(map[string]bool),
	}
}

// goFilesInDir returns the package's non-test .go files, reading each directory
// at most once per graph build.
func (r *importResolver) goFilesInDir(dir string) []string {
	if files, ok := r.pkgFiles[dir]; ok {
		return files
	}
	r.readDirCalls++
	files := findGoFilesInDir(dir, r.projectRoot)
	r.pkgFiles[dir] = files
	return files
}

// exists reports whether path is present, reusing the answer for repeated
// probes of the same candidate.
func (r *importResolver) exists(path string) bool {
	if present, ok := r.statted[path]; ok {
		return present
	}
	r.statCalls++
	_, err := os.Stat(path)
	present := err == nil
	r.statted[path] = present
	return present
}

func buildImportGraph(projectRoot string) *ImportGraph {
	return buildImportGraphWith(newImportResolver(projectRoot))
}

// buildImportGraphWith is the implementation; tests supply their own resolver so
// they can assert how many syscalls one build actually issues.
func buildImportGraphWith(resolver *importResolver) *ImportGraph {
	projectRoot := resolver.projectRoot
	graph := &ImportGraph{
		Importers: make(map[string][]string),
		BuiltAt:   time.Now(),
	}

	filesScanned := 0
	_ = filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if decision, handled := projectscan.DirectoryDecision(d); handled {
			return decision
		}

		ext := filepath.Ext(path)
		if !isSupportedExt(ext) {
			return nil
		}

		if projectscan.TooLarge(d) {
			return nil
		}

		filesScanned++
		if filesScanned > maxFilesForGraph {
			return filepath.SkipAll
		}

		relPath, _ := filepath.Rel(projectRoot, path)
		imports := extractImports(path, ext, resolver)
		for _, imp := range imports {
			graph.Importers[imp] = append(graph.Importers[imp], relPath)
		}

		return nil
	})

	return graph
}

func isSupportedExt(ext string) bool {
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs":
		return true
	}
	return false
}

// extractImports parses import statements from a file and returns relative paths.
func extractImports(filePath, ext string, resolver *importResolver) []string {
	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var imports []string

	switch ext {
	case ".go":
		imports = extractGoImports(scanner, resolver)
	case ".ts", ".tsx", ".js", ".jsx":
		imports = extractTSImports(scanner, filePath, resolver)
	case ".py":
		imports = extractPyImports(scanner, filePath, resolver)
	case ".rs":
		imports = extractRsImports(scanner, filePath, resolver)
	}

	return imports
}

// --- Language-specific import extraction ---

var (
	goImportPattern  = regexp.MustCompile(`^\s*"(.+)"`)
	tsImportPattern  = regexp.MustCompile(`(?:import|from)\s+['"]([^'"]+)['"]`)
	tsRequirePattern = regexp.MustCompile(`require\s*\(\s*['"]([^'"]+)['"]`)
	pyImportPattern  = regexp.MustCompile(`^\s*(?:from\s+(\S+)\s+import|import\s+(\S+))`)
	rsModUsePattern  = regexp.MustCompile(`^\s*(?:mod|use)\s+(?:crate::)?(\S+)`)
)

func extractGoImports(scanner *bufio.Scanner, resolver *importResolver) []string {
	var imports []string
	inImportBlock := false
	goModPath := resolver.goModPath

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "import (" {
			inImportBlock = true
			continue
		}
		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		if inImportBlock || strings.HasPrefix(line, "import ") {
			if m := goImportPattern.FindStringSubmatch(line); len(m) > 1 {
				importPath := m[1]
				if goModPath != "" && strings.HasPrefix(importPath, goModPath) {
					// Local package — resolve to relative path.
					rel := strings.TrimPrefix(importPath, goModPath)
					rel = strings.TrimPrefix(rel, "/")
					// Find .go files in this package directory.
					pkgDir := filepath.Join(resolver.projectRoot, rel)
					imports = append(imports, resolver.goFilesInDir(pkgDir)...)
				}
			}
		}
	}
	return imports
}

func extractTSImports(scanner *bufio.Scanner, filePath string, resolver *importResolver) []string {
	var imports []string
	fileDir := filepath.Dir(filePath)

	for scanner.Scan() {
		line := scanner.Text()

		for _, pat := range []*regexp.Regexp{tsImportPattern, tsRequirePattern} {
			matches := pat.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				if len(m) > 1 {
					imp := m[1]
					// Only resolve relative imports.
					if !strings.HasPrefix(imp, ".") {
						continue
					}
					resolved := resolveRelativeImport(fileDir, imp, resolver)
					if resolved != "" {
						imports = append(imports, resolved)
					}
				}
			}
		}
	}
	return imports
}

func extractPyImports(scanner *bufio.Scanner, filePath string, resolver *importResolver) []string {
	var imports []string
	fileDir := filepath.Dir(filePath)
	projectRoot := resolver.projectRoot

	for scanner.Scan() {
		line := scanner.Text()
		m := pyImportPattern.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		importName := m[1]
		if importName == "" {
			importName = m[2]
		}

		// Skip standard library / third-party imports.
		if !strings.HasPrefix(importName, ".") {
			// Try to resolve as a local module.
			parts := strings.Split(importName, ".")
			candidate := filepath.Join(projectRoot, filepath.Join(parts...)) + ".py"
			if rel, err := filepath.Rel(projectRoot, candidate); err == nil {
				if resolver.exists(candidate) {
					imports = append(imports, rel)
				}
			}
			continue
		}

		// Relative import.
		dots := 0
		for _, c := range importName {
			if c == '.' {
				dots++
			} else {
				break
			}
		}
		name := importName[dots:]
		base := fileDir
		for i := 1; i < dots; i++ {
			base = filepath.Dir(base)
		}
		candidate := filepath.Join(base, strings.ReplaceAll(name, ".", string(filepath.Separator))) + ".py"
		if rel, err := filepath.Rel(projectRoot, candidate); err == nil {
			if resolver.exists(candidate) {
				imports = append(imports, rel)
			}
		}
	}
	return imports
}

func extractRsImports(scanner *bufio.Scanner, filePath string, resolver *importResolver) []string {
	var imports []string
	fileDir := filepath.Dir(filePath)
	projectRoot := resolver.projectRoot

	for scanner.Scan() {
		line := scanner.Text()
		m := rsModUsePattern.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		modName := strings.Split(m[1], "::")[0]
		modName = strings.TrimRight(modName, ";{")

		// Try mod.rs or modName.rs in the same directory.
		for _, candidate := range []string{
			filepath.Join(fileDir, modName+".rs"),
			filepath.Join(fileDir, modName, "mod.rs"),
		} {
			if rel, err := filepath.Rel(projectRoot, candidate); err == nil {
				if resolver.exists(candidate) {
					imports = append(imports, rel)
					break
				}
			}
		}
	}
	return imports
}

// --- Helpers ---

func detectGoModulePath(projectRoot string) string {
	goModFile := filepath.Join(projectRoot, "go.mod")
	f, err := os.Open(goModFile)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func findGoFilesInDir(dir, projectRoot string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			rel, err := filepath.Rel(projectRoot, filepath.Join(dir, e.Name()))
			if err == nil {
				files = append(files, rel)
			}
		}
	}
	return files
}

func resolveRelativeImport(fromDir, importPath string, resolver *importResolver) string {
	candidate := filepath.Join(fromDir, importPath)

	// Try exact match, then with extensions.
	for _, ext := range []string{"", ".ts", ".tsx", ".js", ".jsx", "/index.ts", "/index.tsx", "/index.js", "/index.jsx"} {
		full := candidate + ext
		if resolver.exists(full) {
			rel, err := filepath.Rel(resolver.projectRoot, full)
			if err == nil {
				return rel
			}
		}
	}
	return ""
}

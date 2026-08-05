// Purpose: Discovers project conventions and detects relevant patterns around edited files.
// Why: Keeps convention inference, caching, scanning, and presentation under one owner.
// Docs: docs/features/feature/convention-engine/index.md

package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxFilesToScan         = 500
	maxExamplesPerProbe    = 5
	maxConventionsToReport = 3
	maxFileSizeForScan     = 100 * 1024 // 100KB — skip generated/bundled files
	helperThreshold        = 2          // suggest extracting a helper at this many instances
	maxSummaryConventions  = 10         // top conventions to inject as context
)

// ConventionMatch holds examples of an existing codebase pattern.
type ConventionMatch struct {
	Pattern  string
	Examples []string // "relative/path.go:42: matched line content"
}

// staticProbes are non-call-site patterns that the discovery regex can't find.
// These complement discovered probes — they catch structural patterns like
// type declarations, data structures, and concurrency primitives.
var staticProbes = []string{
	"http.Client{",
	"map[string]func",
	"sync.Mutex",
	"sync.RWMutex",
	"new Map<",
	"new Set<",
	"chrome.storage.",
	"chrome.runtime.",
}

// typePattern detects struct declarations that should be checked for duplicates.
var typePattern = regexp.MustCompile(`type\s+(\w+)\s+struct`)

var skipDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true,
	"dist": true, "build": true, ".next": true,
	"__pycache__": true, ".cache": true, ".claude": true,
}

// DetectConventions finds patterns in newContent and searches the project for existing usage.
// Uses discovered probes (from automatic codebase analysis) plus static probes for
// non-call-site patterns. Returns nil if no conventions found or if newContent is empty.
func DetectConventions(filePath, projectRoot, newContent string) []ConventionMatch {
	if newContent == "" || projectRoot == "" {
		return nil
	}

	ext := filepath.Ext(filePath)
	exts := extensionFamily(ext)

	// Merge discovered probes with static probes.
	discovered := DiscoveredProbes(projectRoot, ext)
	allProbes := append(discovered, staticProbes...)

	// Collect probes that match the edit content.
	var probes []string
	for _, probe := range allProbes {
		if strings.Contains(newContent, probe) {
			probes = append(probes, probe)
		}
	}

	// Check for type declarations (duplicate detection).
	for _, m := range typePattern.FindAllStringSubmatch(newContent, -1) {
		if len(m) > 1 {
			probes = append(probes, "type "+m[1]+" struct")
		}
	}

	if len(probes) == 0 {
		return nil
	}

	// Search the project for each probe.
	var results []ConventionMatch
	for _, probe := range probes {
		examples := searchProject(projectRoot, probe, filePath, exts)
		if len(examples) > 0 {
			results = append(results, ConventionMatch{
				Pattern:  probe,
				Examples: examples,
			})
		}
		if len(results) >= maxConventionsToReport {
			break
		}
	}

	return results
}

// ConventionSummary returns a compact summary of the top discovered conventions
// for the given file's language. Injected on every edit so the LLM can judge
// convention drift even when the edit doesn't contain a matching pattern.
func ConventionSummary(projectRoot, ext string) string {
	conventions := DiscoverConventions(projectRoot, ext)
	if len(conventions) == 0 {
		return ""
	}

	limit := maxSummaryConventions
	if len(conventions) < limit {
		limit = len(conventions)
	}

	var b strings.Builder
	b.WriteString("=== PROJECT CONVENTIONS (auto-discovered) ===")
	b.WriteString("\nThis project consistently uses these patterns — align new code accordingly:")
	for _, c := range conventions[:limit] {
		fmt.Fprintf(&b, "\n  %s (%d files)", c.Pattern, c.FileCount)
	}
	b.WriteString("\n=== END PROJECT CONVENTIONS ===")
	return b.String()
}

// searchProject walks the project tree and finds lines containing the search term.
func searchProject(root, term, excludeFile string, exts []string) []string {
	absExclude, _ := filepath.Abs(excludeFile)
	var examples []string
	filesScanned := 0

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if decision, handled := projectDirectoryDecision(d); handled {
			return decision
		}

		absPath, _ := filepath.Abs(path)
		if absPath == absExclude {
			return nil
		}

		data, ok := readConventionSource(path, d, exts)
		if !ok {
			return nil
		}

		filesScanned++
		if filesScanned > maxFilesToScan {
			return filepath.SkipAll
		}

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(line, term) {
				relPath, _ := filepath.Rel(root, path)
				if relPath == "" {
					relPath = path
				}
				trimmed := strings.TrimSpace(line)
				if len([]rune(trimmed)) > 120 {
					trimmed = string([]rune(trimmed)[:117]) + "..."
				}
				examples = append(examples, fmt.Sprintf("  %s:%d: %s", relPath, i+1, trimmed))
				if len(examples) >= maxExamplesPerProbe {
					return filepath.SkipAll
				}
				break // one example per file
			}
		}

		return nil
	})

	return examples
}

// projectDirectoryDecision centralizes repository-walk pruning shared by hook
// scanners. The boolean distinguishes files from directories that should
// continue normally.
func projectDirectoryDecision(d os.DirEntry) (error, bool) {
	if !d.IsDir() {
		return nil, false
	}
	if skipDirs[d.Name()] || (strings.HasPrefix(d.Name(), ".") && d.Name() != ".") {
		return filepath.SkipDir, true
	}
	return nil, true
}

// readConventionSource applies the single canonical filter for source consumed
// by convention detection and discovery.
func readConventionSource(path string, d os.DirEntry, exts []string) ([]byte, bool) {
	if !matchesExtension(path, exts) || isGenerated(d.Name()) {
		return nil, false
	}
	info, err := d.Info()
	if err != nil || info.Size() > maxFileSizeForScan {
		return nil, false
	}
	data, err := os.ReadFile(path)
	return data, err == nil
}

func isGenerated(name string) bool {
	return strings.Contains(name, ".bundled.") ||
		strings.Contains(name, ".min.") ||
		strings.HasSuffix(name, ".map")
}

func matchesExtension(path string, exts []string) bool {
	ext := filepath.Ext(path)
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

func extensionFamily(ext string) []string {
	switch ext {
	case ".go":
		return []string{".go"}
	case ".ts", ".tsx":
		return []string{".ts", ".tsx", ".js", ".jsx"}
	case ".js", ".jsx":
		return []string{".js", ".jsx", ".ts", ".tsx"}
	case ".py":
		return []string{".py"}
	case ".rs":
		return []string{".rs"}
	default:
		return []string{ext}
	}
}

// FormatConventions formats convention matches for additionalContext output.
// If 2+ instances of a pattern exist, suggests extracting a shared helper.
func FormatConventions(matches []ConventionMatch) string {
	if len(matches) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("=== CODEBASE CONVENTIONS (match existing patterns) ===")
	for _, m := range matches {
		n := len(m.Examples)
		fmt.Fprintf(&b, "\n%s (%d existing usage%s):", m.Pattern, n, pluralS(n))
		for _, ex := range m.Examples {
			fmt.Fprintf(&b, "\n%s", ex)
		}
		if n >= helperThreshold {
			b.WriteString("\n  ^ SUGGESTION: Consider extracting a shared helper — this pattern already exists in ")
			b.WriteString(fmt.Sprintf("%d files. Reuse or align with existing code rather than introducing a new variant.", n))
		}
	}
	b.WriteString("\n=== END CONVENTIONS ===")
	return b.String()
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

const (
	discoveryMinFiles  = 3   // pattern must appear in this many distinct files
	discoveryMaxProbes = 20  // max conventions to return
	discoveryMaxFiles  = 500 // max files to scan during discovery
	discoveryCacheTTL  = 5 * time.Minute
)

// DiscoveredConvention is a call-site pattern found in multiple files.
type DiscoveredConvention struct {
	Pattern   string
	FileCount int
}

// discoveryCache stores discovered conventions per project root + language.
var discoveryCache = struct {
	mu      sync.RWMutex
	entries map[string]*discoveryCacheEntry
}{
	entries: make(map[string]*discoveryCacheEntry),
}

type discoveryCacheEntry struct {
	conventions []DiscoveredConvention
	timestamp   time.Time
}

// goCallSite matches pkg.ExportedFunc( — the dominant Go convention pattern.
// Lowercase receiver.UppercaseMethod( captures both package calls and method calls.
var goCallSite = regexp.MustCompile(`\b([a-z][a-zA-Z]*)\.([A-Z][a-zA-Z]*)\(`)

// tsCallSite matches obj.method( — TS/JS uses camelCase methods.
var tsCallSite = regexp.MustCompile(`\b([a-zA-Z][a-zA-Z]*)\.([a-zA-Z][a-zA-Z]*)\(`)

// goNoise are patterns so universal in Go they carry no convention signal.
// These appear in virtually every Go project — knowing about them adds no value.
var goNoise = map[string]bool{
	// testing
	"t.Fatalf(": true, "t.Fatal(": true, "t.Errorf(": true, "t.Error(": true,
	"t.Run(": true, "t.Helper(": true, "t.Parallel(": true, "t.TempDir(": true,
	"t.Cleanup(": true, "t.Setenv(": true, "t.Logf(": true, "t.Log(": true,
	"t.Skip(": true, "t.Skipf(": true, "t.Name(": true, "t.Failed(": true,
	"b.ResetTimer(": true, "b.ReportAllocs(": true, "b.RunParallel(": true,
	"f.Add(": true, "f.Fuzz(": true,
	// fmt — every Go program uses these
	"fmt.Sprintf(": true, "fmt.Fprintf(": true, "fmt.Printf(": true,
	"fmt.Println(": true, "fmt.Sprint(": true,
	// strings — universal
	"strings.Contains(": true, "strings.HasPrefix(": true, "strings.HasSuffix(": true,
	"strings.TrimSpace(": true, "strings.Split(": true, "strings.Join(": true,
	"strings.ToLower(": true, "strings.ToUpper(": true, "strings.NewReader(": true,
	"strings.Repeat(": true, "strings.Replace(": true, "strings.ReplaceAll(": true,
	"strings.Index(": true, "strings.Count(": true, "strings.Builder(": true,
	"strings.EqualFold(": true, "strings.Map(": true, "strings.Cut(": true,
	// filepath — universal
	"filepath.Join(": true, "filepath.Dir(": true, "filepath.Base(": true,
	"filepath.Ext(": true, "filepath.Rel(": true, "filepath.Abs(": true,
	// os basics
	"os.Stat(": true, "os.IsNotExist(": true, "os.Getenv(": true,
	"os.MkdirAll(": true, "os.Remove(": true,
	// errors
	"err.Error(": true, "errors.Is(": true, "errors.As(": true,
	// sync primitives — method calls on instances, not pattern choices
	"mu.Lock(": true, "mu.Unlock(": true, "mu.RLock(": true, "mu.RUnlock(": true,
	"wg.Add(": true, "wg.Done(": true, "wg.Wait(": true,
	// time basics
	"time.Now(": true, "time.Since(": true, "time.Sleep(": true,
	// context
	"ctx.Done(": true, "ctx.Err(": true, "ctx.Value(": true,
	// io
	"io.ReadAll(": true, "io.Copy(": true,
	// bytes
	"bytes.NewBuffer(": true, "bytes.NewReader(": true,
	// sort
	"sort.Slice(": true, "sort.Sort(": true, "sort.Strings(": true,
}

// tsNoise are patterns so universal in TS/JS they carry no convention signal.
var tsNoise = map[string]bool{
	// builtins
	"Date.now(": true, "Math.min(": true, "Math.max(": true,
	"Math.round(": true, "Math.floor(": true, "Math.ceil(": true,
	"Math.abs(": true, "Math.random(": true,
	"Array.from(": true, "Array.isArray(": true,
	"Object.keys(": true, "Object.values(": true, "Object.entries(": true,
	"Object.assign(": true, "Object.freeze(": true,
	"Number.isFinite(": true, "Number.parseInt(": true,
	"JSON.stringify(": true, "JSON.parse(": true,
	"Promise.all(": true, "Promise.race(": true, "Promise.resolve(": true,
	"String.fromCharCode(": true,
	// console — borderline, but universal
	"console.log(": true, "console.error(": true, "console.warn(": true,
	// DOM basics too universal to be conventions
	"document.createElement(": true, "document.createTextNode(": true,
	// string/array methods on instances
	"tagName.toLowerCase(": true,
}

// DiscoverConventions walks the project and returns call-site patterns
// that repeat across discoveryMinFiles+ files, ranked by frequency.
// Results are cached per project root + file extension.
func DiscoverConventions(projectRoot, ext string) []DiscoveredConvention {
	if projectRoot == "" {
		return nil
	}

	key := projectRoot + "\x00" + ext
	discoveryCache.mu.RLock()
	if entry, ok := discoveryCache.entries[key]; ok {
		if time.Since(entry.timestamp) < discoveryCacheTTL {
			discoveryCache.mu.RUnlock()
			return entry.conventions
		}
	}
	discoveryCache.mu.RUnlock()

	exts := extensionFamily(ext)
	noise := noiseSetForExt(ext)
	callSite := callSiteForExt(ext)

	// Map: pattern -> set of files it appears in.
	patternFiles := make(map[string]map[string]bool)
	filesScanned := 0

	_ = filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if decision, handled := projectDirectoryDecision(d); handled {
			return decision
		}

		data, ok := readConventionSource(path, d, exts)
		if !ok {
			return nil
		}

		filesScanned++
		if filesScanned > discoveryMaxFiles {
			return filepath.SkipAll
		}

		relPath, _ := filepath.Rel(projectRoot, path)
		content := string(data)
		seen := make(map[string]bool)

		for _, m := range callSite.FindAllString(content, -1) {
			if seen[m] || noise[m] {
				continue
			}
			seen[m] = true
			if patternFiles[m] == nil {
				patternFiles[m] = make(map[string]bool)
			}
			patternFiles[m][relPath] = true
		}

		return nil
	})

	// Filter: keep patterns in 3+ files, sort by frequency descending.
	var conventions []DiscoveredConvention
	for pattern, files := range patternFiles {
		if len(files) >= discoveryMinFiles {
			conventions = append(conventions, DiscoveredConvention{
				Pattern:   pattern,
				FileCount: len(files),
			})
		}
	}

	sort.Slice(conventions, func(i, j int) bool {
		if conventions[i].FileCount != conventions[j].FileCount {
			return conventions[i].FileCount > conventions[j].FileCount
		}
		return conventions[i].Pattern < conventions[j].Pattern
	})

	if len(conventions) > discoveryMaxProbes {
		conventions = conventions[:discoveryMaxProbes]
	}

	// Cache.
	discoveryCache.mu.Lock()
	discoveryCache.entries[key] = &discoveryCacheEntry{
		conventions: conventions,
		timestamp:   time.Now(),
	}
	discoveryCache.mu.Unlock()

	return conventions
}

// DiscoveredProbes returns just the pattern strings from discovery, suitable
// for passing to the existing convention detection + search flow.
func DiscoveredProbes(projectRoot, ext string) []string {
	conventions := DiscoverConventions(projectRoot, ext)
	probes := make([]string, len(conventions))
	for i, c := range conventions {
		probes[i] = c.Pattern
	}
	return probes
}

func callSiteForExt(ext string) *regexp.Regexp {
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx":
		return tsCallSite
	default:
		return goCallSite
	}
}

func noiseSetForExt(ext string) map[string]bool {
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx":
		return tsNoise
	default:
		return goNoise
	}
}

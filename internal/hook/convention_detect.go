// convention_detect.go — Detects patterns in edited code and searches the codebase
// for existing usage, so the AI sees how the project already handles that pattern.
// If 2+ instances exist, suggests extracting a shared helper.
//
// Two detection modes:
// 1. Direct match — edit contains a discovered/static probe → search for examples
// 2. Convention summary — top discovered patterns injected on every edit so the
//    LLM can judge drift even when the edit doesn't contain the pattern

package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

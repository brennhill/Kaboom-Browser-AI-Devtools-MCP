// convention_discover.go — Automatic convention discovery.
// Walks the codebase, extracts call-site patterns that repeat across 3+ files,
// and returns them ranked by frequency. No hardcoded probe list needed.
//
// The discovery engine finds what the codebase DOES. The LLM judges what
// new code SHOULD follow. The hook surfaces conventions as context — the LLM
// connects the dots when an edit drifts from established patterns.

package hook

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

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

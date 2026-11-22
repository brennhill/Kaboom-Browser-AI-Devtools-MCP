// contexts.go — Bounded-context dependency rules: a named context never imports the contexts upstream of it.
//
// Each entry names a bounded context from GLOSSARY.md and
// docs/core/architecture/server-architecture.md ("Bounded contexts"). The
// rule is directional: a context may depend on the contexts DOWNSTREAM of it
// (its own stores, leaf utilities, the browser-command owner's public API),
// but importing a package listed in forbidden means the context reached
// around its handoff contract into another context's internals or into a
// transport/adapter layer that should depend on it instead.
//
// The browser-command module, application use cases and the observe/interact
// adapters keep their dedicated rules in main.go and command_owner.go; this
// table covers the remaining daemon-side contexts.
package main

import (
	"fmt"
	"strings"
)

type boundedContext struct {
	// name is the GLOSSARY.md context name reported in violations.
	name string
	// packages are repo-relative package roots (the package and its subpackages).
	packages []string
	// excluded are repo-relative package roots inside packages that are transport
	// adapters (MCP handlers) rather than the context's domain code.
	excluded []string
	// forbidden are import roots (repo-relative internal packages or stdlib
	// roots); the import and its subpackages are rejected.
	forbidden []string
	// forbiddenExact are repo-relative packages rejected only as an exact
	// match (e.g. the whole-Capture composition object, not its stores).
	forbiddenExact []string
	why            string
}

func boundedContexts() []boundedContext {
	return []boundedContext{
		{
			name: "Observation Capture",
			packages: []string{
				"internal/observation/store/telemetry", "internal/observation/store/logs", "internal/observation/store/perf",
				"internal/observation/store/actions", "internal/observation/store/bodies", "internal/observation/store/waterfall",
				"internal/observation/store/ring", "internal/observation/store/wsconn", "internal/observation/pressure",
			},
			forbidden: []string{
				"net/http", "internal/tools/mcp", "internal/tools", "internal/page/app", "internal/analysis/app",
				"internal/browsercommand", "internal/browsercommand/query",
				"internal/sync/runtime", "internal/ingestion/http", "internal/recording",
			},
			forbiddenExact: []string{"internal/composition/capture"},
			why:            "capture stores retain observations; ingestion and readers depend on them, never the reverse",
		},
		{
			name:     "Event Recording",
			packages: []string{"internal/recording"},
			excluded: []string{"internal/recording/replay", "internal/recording/sequence"},
			forbidden: []string{
				"net/http", "internal/tools/mcp", "internal/page/app", "internal/analysis/app",
				"internal/browsercommand", "internal/browsercommand/query",
				"internal/sync/runtime", "internal/ingestion/http",
			},
			forbiddenExact: []string{"internal/composition/capture"},
			why:            "event recording is a daemon-local lifecycle; MCP and HTTP adapters drive it",
		},
		{
			name:     "Connection & Sync",
			packages: []string{"internal/sync/runtime"},
			forbidden: []string{
				"internal/tools/mcp", "internal/tools", "internal/page/app", "internal/analysis/app", "internal/recording",
				"internal/ingestion/http",
			},
			forbiddenExact: []string{"internal/composition/capture"},
			why:            "sync owns connection identity and hands command work to browsercommand.Owner only",
		},
		{
			name:     "Extension Ingestion",
			packages: []string{"internal/ingestion/http"},
			forbidden: []string{
				"internal/tools/mcp", "internal/tools", "internal/page/app", "internal/analysis/app", "internal/sync/runtime",
			},
			forbiddenExact: []string{"internal/composition/capture"},
			why:            "HTTP ingestion writes to its owners' ports; it is not a tool or sync adapter",
		},
	}
}

// contextOf returns the bounded context owning a package import path, if any.
func contextOf(pkg string) (boundedContext, bool) {
	rel, ok := strings.CutPrefix(pkg, modulePrefix)
	if !ok {
		return boundedContext{}, false
	}
	for _, c := range boundedContexts() {
		if c.excludes(rel) {
			continue
		}
		for _, root := range c.packages {
			if isPkgOrSub(rel, root) {
				return c, true
			}
		}
	}
	return boundedContext{}, false
}

// excludes reports whether rel is an adapter package carved out of the context.
func (c boundedContext) excludes(rel string) bool {
	for _, root := range c.excluded {
		if isPkgOrSub(rel, root) {
			return true
		}
	}
	return false
}

// forbids reports whether the context rejects the import path imp.
func (c boundedContext) forbids(imp string) bool {
	rel := strings.TrimPrefix(imp, modulePrefix)
	internal := rel != imp
	for _, root := range c.forbidden {
		isStdlib := !strings.HasPrefix(root, "internal/") && !strings.HasPrefix(root, "cmd/")
		if isStdlib == internal {
			continue
		}
		if isPkgOrSub(rel, root) {
			return true
		}
	}
	if internal {
		for _, exact := range c.forbiddenExact {
			if rel == exact {
				return true
			}
		}
	}
	return false
}

// contextViolations reports production imports that cross a bounded context
// against its declared direction.
func contextViolations(scan *treeScan) []string {
	var violations []string
	for file, imports := range scan.imports {
		c, ok := contextOf(scan.pkgOf[file])
		if !ok {
			continue
		}
		for imp := range imports {
			if c.forbids(imp) {
				violations = append(violations, fmt.Sprintf("context: %s (%s) imports %s; %s", file, c.name, imp, c.why))
			}
		}
	}
	return violations
}

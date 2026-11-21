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
				"internal/capture/telemetrystore", "internal/capture/logstore", "internal/capture/perfstore",
				"internal/capture/actionstore", "internal/capture/bodystore", "internal/capture/waterfallstore",
				"internal/capture/ringstore", "internal/capture/wsconn", "internal/capture/pressure",
			},
			forbidden: []string{
				"net/http", "internal/mcp", "internal/tools", "internal/app",
				"internal/browsercommand", "internal/queries",
				"internal/capture/syncruntime", "internal/capture/httpingest", "internal/recording",
			},
			forbiddenExact: []string{"internal/capture"},
			why:            "capture stores retain observations; ingestion and readers depend on them, never the reverse",
		},
		{
			name:     "Event Recording",
			packages: []string{"internal/recording"},
			forbidden: []string{
				"net/http", "internal/mcp", "internal/app",
				"internal/browsercommand", "internal/queries",
				"internal/capture/syncruntime", "internal/capture/httpingest",
			},
			forbiddenExact: []string{"internal/capture"},
			why:            "event recording is a daemon-local lifecycle; MCP and HTTP adapters drive it",
		},
		{
			name:     "Connection & Sync",
			packages: []string{"internal/capture/syncruntime"},
			forbidden: []string{
				"internal/mcp", "internal/tools", "internal/app", "internal/recording",
				"internal/capture/httpingest",
			},
			forbiddenExact: []string{"internal/capture"},
			why:            "sync owns connection identity and hands command work to browsercommand.Owner only",
		},
		{
			name:     "Extension Ingestion",
			packages: []string{"internal/capture/httpingest"},
			forbidden: []string{
				"internal/mcp", "internal/tools", "internal/app", "internal/capture/syncruntime",
			},
			forbiddenExact: []string{"internal/capture"},
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
		for _, root := range c.packages {
			if isPkgOrSub(rel, root) {
				return c, true
			}
		}
	}
	return boundedContext{}, false
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

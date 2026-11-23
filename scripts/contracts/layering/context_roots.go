// context_roots.go — Gate: every internal package lives under a declared bounded-context root folder.
//
// The physical layout is internal/<context>/... (kaboom-us7o). A new top-level
// folder under internal/ must be declared here, with the bounded context that
// owns it, so code cannot accumulate outside a context.
package main

import (
	"fmt"
	"sort"
	"strings"
)

// contextRoots maps each top-level folder under internal/ to its owning context.
func contextRoots() map[string]string {
	return map[string]string{
		"analysis":       "Analysis",
		"annotation":     "Annotation",
		"browsercommand": "Browser Command",
		"composition":    "Composition",
		"hooks":          "Agent Hooks",
		"ingestion":      "Extension Ingestion",
		"lifecycle":      "Process Lifecycle",
		"observation":    "Observation Capture",
		"page":           "Page",
		"platform":       "Platform (shared kernel)",
		"push":           "Push",
		"recording":      "Event Recording",
		"screenrec":      "Screen Recording",
		"sync":           "Connection & Sync",
		"telemetry":      "Product Telemetry",
		"terminal":       "Terminal",
		"testgen":        "Test Generation",
		"tools":          "Tool Surface",
		"upload":         "Upload",
	}
}

// contextRootViolations reports internal packages whose top-level folder is not a declared context root.
func contextRootViolations(scan *treeScan) []string {
	roots := contextRoots()
	seen := map[string]bool{}
	var violations []string
	for _, pkg := range scan.pkgOf {
		rel, ok := strings.CutPrefix(pkg, modulePrefix+"internal/")
		if !ok || seen[pkg] {
			continue
		}
		seen[pkg] = true
		top, _, _ := strings.Cut(rel, "/")
		if _, declared := roots[top]; !declared {
			violations = append(violations, fmt.Sprintf("context-root: internal/%s is under undeclared root internal/%s; move it into a context or declare the root in scripts/contracts/layering/context_roots.go", rel, top))
		}
	}
	sort.Strings(violations)
	return violations
}

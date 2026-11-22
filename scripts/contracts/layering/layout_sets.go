// layout_sets.go — Package sets the layering rules share after the bounded-context relocation.
package main

import "strings"

// toolsLayer reports whether the module-relative path is one of the five MCP tool
// domain packages (observe, analyze, configure, generate, interact). The wider
// internal/tools context root also holds protocol and contract packages, which are
// not the "tool layer" the schema, protocol and use-case rules keep apart.
func toolsLayer(internal string) bool {
	for _, tool := range []string{"observe", "analyze", "configure", "generate", "interact"} {
		if isPkgOrSub(internal, "internal/tools/"+tool) {
			return true
		}
	}
	return false
}

// captureTree lists the packages that formerly lived under internal/capture. They
// remain the capture tree for the capture-root rule and the command-module
// transport rule even though they now sit in their bounded contexts.
var captureTree = []string{
	"internal/composition/capture", "internal/capture/settingscache", "internal/sync/runtime",
	"internal/ingestion/http", "internal/observation/store", "internal/observation/pressure",
	"internal/observation/clientstore", "internal/observation/featureusage",
	"internal/observation/healthreader", "internal/observation/resetter",
	"internal/observation/testsupport/pipeline", "internal/observation/testsupport/websocket",
	"internal/recording/testsupport",
}

// inCaptureTree reports whether a full import/package path is in the former capture tree.
func inCaptureTree(path string) bool {
	rel := strings.TrimPrefix(path, modulePrefix)
	for _, p := range captureTree {
		if isPkgOrSub(rel, p) {
			return true
		}
	}
	return false
}

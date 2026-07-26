// modespecs_interact.go — interact tool per-mode parameter specs.
// Docs: docs/features/describe_capabilities.md
package capabilities

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema/interact"

// interactModeSpecs derives directly from the canonical interact action registry
// in internal/schema/interact/actions.go to keep schema + capabilities in sync.
var interactModeSpecs = buildInteractModeSpecs()

func buildInteractModeSpecs() map[string]modeParamSpec {
	specs := interact.ActionSpecs()
	out := make(map[string]modeParamSpec, len(specs))
	for _, spec := range specs {
		if spec.IsAlias {
			continue
		}
		out[spec.Name] = modeParamSpec{
			Hint:     spec.Hint,
			Required: append([]string(nil), spec.Required...),
			Optional: append([]string(nil), spec.Optional...),
		}
	}
	return out
}

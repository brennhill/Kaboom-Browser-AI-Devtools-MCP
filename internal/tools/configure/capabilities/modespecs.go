// modespecs.go — Per-mode parameter specs for all tools.
// Docs: docs/features/generation/describe_capabilities.md
//
// One file per MCP tool below. That is not an accident of a past file split: the
// repo has exactly five tools (observe, generate, configure, interact, analyze) and
// the set is fixed, so a file per tool is a stable one-to-one mapping rather than an
// arbitrary slice of a big table.
package capabilities

// modeParamSpec is the shape of every entry in the per-tool spec tables below.
type modeParamSpec struct {
	// Hint says what the mode DOES.
	Hint string
	// Returns says what the RESPONSE CONTAINS: the fields a caller gets back,
	// and anything a reader would reasonably expect but will not find.
	//
	// Hint alone is not a contract. "List captured browser session recordings"
	// and "List saved browser recording videos" were indistinguishable, and
	// neither said that a recordings entry carried every captured action — so
	// the listing shipped its whole corpus to answer a question about names.
	Returns  string
	Required []string
	Optional []string
}

// toolModeSpecs maps tool name → mode name → { Hint, Required, Optional }.
// Each mode lists only the params relevant to that mode, preventing
// the full param list from being dumped into every mode's output.
// Hint is a one-line description surfaced in summary mode for discovery.
var toolModeSpecs = map[string]map[string]modeParamSpec{
	"configure": configureModeSpecs,
	"observe":   observeModeSpecs,
	"interact":  interactModeSpecs,
	"analyze":   analyzeModeSpecs,
	"generate":  generateModeSpecs,
}

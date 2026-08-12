// Purpose: Parses and validates computed styles query arguments for the analyze tool.
// Docs: docs/features/feature/analyze-tool/index.md

package analyze

import (
	"encoding/json"
	"errors"
)

// ComputedStylesArgs holds parsed arguments for computed styles queries.
type ComputedStylesArgs struct {
	Selector   string   `json:"selector"`
	Properties []string `json:"properties,omitempty"`
	Frame      string   `json:"frame,omitempty"`
	TabID      int      `json:"tab_id,omitempty"`
	// MaxElements raises or lowers the probe's element cap. The page clamps it
	// to a documented ceiling and reports truncation explicitly, so a caller
	// never has to guess whether a result covers every match.
	MaxElements int `json:"max_elements,omitempty"`
	// IncludeCustomProperties adds the :root token table and each element's
	// in-scope --* values. Off by default because only design-token analysis
	// needs them and they are not free to enumerate.
	IncludeCustomProperties bool `json:"include_custom_properties,omitempty"`
}

// ParseComputedStylesArgs validates and parses computed styles arguments.
func ParseComputedStylesArgs(args json.RawMessage) (*ComputedStylesArgs, error) {
	params, err := parseAnalyzeArgs[ComputedStylesArgs](args)
	if err != nil {
		return nil, err
	}
	if params.Selector == "" {
		return nil, errors.New("required parameter 'selector' is missing")
	}
	return params, nil
}

// Purpose: Repairs individual broken CSS selectors using heuristic rewrite strategies.
// Why: Separates single-selector repair logic from batch processing and path validation.
package heal

import (
	"fmt"
	"strings"
)

// RepairSelectors attempts to repair a list of broken selectors.
func RepairSelectors(req TestHealRequest) (*HealResult, error) {
	result := &HealResult{
		Healed:   make([]HealedSelector, 0),
		Unhealed: make([]string, 0),
		Summary: HealSummary{
			TotalBroken: len(req.BrokenSelectors),
		},
	}

	for _, selector := range req.BrokenSelectors {
		repairOneSelector(selector, req.AutoApply, result)
	}

	result.Summary.Unhealed = len(result.Unhealed)
	return result, nil
}

func repairOneSelector(selector string, autoApply bool, result *HealResult) {
	if err := ValidateSelector(selector); err != nil {
		result.Unhealed = append(result.Unhealed, selector)
		return
	}

	healed, err := HealSelector(selector)
	if err != nil || healed == nil {
		result.Unhealed = append(result.Unhealed, selector)
		return
	}

	result.Healed = append(result.Healed, *healed)
	ClassifyHealedSelector(healed, autoApply, result)
}

// ClassifyHealedSelector categorizes a healed selector by confidence.
func ClassifyHealedSelector(healed *HealedSelector, autoApply bool, result *HealResult) {
	if autoApply && healed.Confidence >= 0.9 {
		result.Summary.HealedAuto++
	} else if healed.Confidence >= 0.5 {
		result.Summary.HealedManual++
	}
}

// HealSelector attempts to find a replacement for a broken selector.
func HealSelector(oldSelector string) (*HealedSelector, error) {
	var newSelector string
	var confidence float64
	var strategy string

	if strings.HasPrefix(oldSelector, "#") {
		idValue := strings.TrimPrefix(oldSelector, "#")
		newSelector = fmt.Sprintf("[data-testid='%s']", idValue)
		confidence = 0.6
		strategy = "testid_match"
	} else if strings.HasPrefix(oldSelector, ".") {
		newSelector = oldSelector
		confidence = 0.3
		strategy = "structural_match"
	} else {
		return nil, fmt.Errorf("cannot heal selector: %s", oldSelector)
	}

	return &HealedSelector{
		OldSelector: oldSelector,
		NewSelector: newSelector,
		Confidence:  confidence,
		Strategy:    strategy,
		LineNumber:  0,
	}, nil
}

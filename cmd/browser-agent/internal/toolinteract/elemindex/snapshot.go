// snapshot.go — Building one index snapshot from what a discovery action returned.
//
// Two actions discover elements and both hand out handles: list_interactive scans the DOM
// and returns selectors, find reads the accessibility tree and returns ax_ refs. They
// describe the same page, so they build the same kind of snapshot and are stamped with the
// same generation. Keeping both constructors here is what stops the two handle spaces from
// drifting apart again.
//
// Both take the decoded JSON payload as []any: values that came through encoding/json into
// map[string]any, where every number is a float64.
package elemindex

// TargetsFromElements converts a list_interactive element list into index-keyed targets.
//
// An element with no selector is skipped: its index would name something no action can
// address, and handing back an unusable handle is worse than one fewer result.
func TargetsFromElements(elements []any) map[int]Target {
	targets := make(map[int]Target, len(elements))
	for _, element := range elements {
		fields, ok := element.(map[string]any)
		if !ok {
			continue
		}
		selector, _ := fields["selector"].(string)
		if selector == "" {
			continue
		}
		index, _ := fields["index"].(float64)
		role, _ := fields["element_type"].(string)
		label, _ := fields["label"].(string)
		targets[int(index)] = Target{Selector: selector, Role: role, Name: label}
	}
	return targets
}

// TargetsFromCandidates converts find's ranked candidates into index-keyed targets.
//
// Rank order is the index: the response lists candidates best-first, so an agent that reads
// "index 0" must get the candidate it read at position 0.
func TargetsFromCandidates(candidates []any) map[int]Target {
	targets := make(map[int]Target, len(candidates))
	for position, candidate := range candidates {
		fields, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		backendID, wellFormed := parseRef(stringField(fields, "ref"))
		if !wellFormed {
			continue
		}
		target := Target{
			AXBackendID: backendID,
			Role:        stringField(fields, "role"),
			Name:        stringField(fields, "name"),
		}
		if centerX, centerY, resolved := centerOf(fields); resolved {
			target.CenterX, target.CenterY, target.HasCenter = centerX, centerY, true
		}
		targets[position] = target
	}
	return targets
}

// centerOf is the viewport point to address for a candidate, from the box find resolved.
//
// An accessibility candidate has no selector, so this point is the only way to act on it. A
// candidate whose box could not be read reports no center rather than 0,0 — a click at the
// top-left corner of the page is not a degraded version of the right click.
func centerOf(fields map[string]any) (float64, float64, bool) {
	x, hasX := fields["x"].(float64)
	y, hasY := fields["y"].(float64)
	if !hasX || !hasY {
		return 0, 0, false
	}
	width, _ := fields["width"].(float64)
	height, _ := fields["height"].(float64)
	return x + width/2, y + height/2, true
}

func stringField(fields map[string]any, name string) string {
	value, _ := fields[name].(string)
	return value
}

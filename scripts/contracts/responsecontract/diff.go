// diff.go — Names the mode and the field that moved when a response drifts.
//
// PURPOSE: "the response changed" is not actionable. Every message here names
// the tool/mode and the exact field path, so a failing gate tells the author
// which handler they edited and which key they moved.
//
// Docs: docs/features/feature/quality-gates/index.md
package responsecontract

import (
	"fmt"
	"sort"
)

// Drift is one difference between a declared shape and a shipped response.
type Drift struct {
	Mode  string `json:"mode"`
	Field string `json:"field"`
	// Detail reads as a full sentence naming the mode and the field.
	Detail string `json:"detail"`
}

// index maps field path to declared type for O(1) comparison.
func index(fields []Field) map[string]string {
	byPath := make(map[string]string, len(fields))
	for _, field := range fields {
		byPath[field.Path] = field.Type
	}
	return byPath
}

// Diff reports every way the shipped response no longer matches its declared
// shape. An empty result means the response still honours its contract.
func Diff(mode string, declared, shipped Shape) []Drift {
	drifts := make([]Drift, 0)
	if declared.Kind != shipped.Kind {
		drifts = append(drifts, Drift{
			Mode:  mode,
			Field: "",
			Detail: fmt.Sprintf(
				"%s: response kind changed — declared %q, the response is %q. A daemon-local payload and an async lifecycle envelope are different contracts.",
				mode, declared.Kind, shipped.Kind),
		})
	}
	declaredByPath, shippedByPath := index(declared.Fields), index(shipped.Fields)
	drifts = append(drifts, missingAndRetyped(mode, declared.Fields, shippedByPath)...)
	drifts = append(drifts, undeclared(mode, shipped.Fields, declaredByPath)...)
	sort.Slice(drifts, func(a, b int) bool { return drifts[a].Detail < drifts[b].Detail })
	return drifts
}

// typesAgree reports whether a shipped type still honours the declared one.
//
// null agrees with everything, in both directions. JSON null means "no value",
// not "of type null": a nullable field whose fixture sample happened to be null
// carries no type knowledge, and failing the gate when it later carries a
// string would report the fixture's data as contract drift. The PATH is still
// gated, which is where a renamed or dropped field shows up.
func typesAgree(declared, shipped string) bool {
	return declared == shipped || declared == "null" || shipped == "null"
}

// missingAndRetyped reports declared fields the response dropped or retyped.
func missingAndRetyped(mode string, declared []Field, shipped map[string]string) []Drift {
	drifts := make([]Drift, 0)
	for _, field := range declared {
		shippedType, present := shipped[field.Path]
		if !present {
			drifts = append(drifts, Drift{Mode: mode, Field: field.Path, Detail: fmt.Sprintf(
				"%s: declared field %q (%s) is GONE from the response. A caller reading it now gets nothing.",
				mode, field.Path, field.Type)})
			continue
		}
		if !typesAgree(field.Type, shippedType) {
			drifts = append(drifts, Drift{Mode: mode, Field: field.Path, Detail: fmt.Sprintf(
				"%s: field %q CHANGED TYPE — declared %s, the response carries %s.",
				mode, field.Path, field.Type, shippedType)})
		}
	}
	return drifts
}

// undeclared reports response fields that no declaration covers.
func undeclared(mode string, shipped []Field, declared map[string]string) []Drift {
	drifts := make([]Drift, 0)
	for _, field := range shipped {
		if _, present := declared[field.Path]; present {
			continue
		}
		drifts = append(drifts, Drift{Mode: mode, Field: field.Path, Detail: fmt.Sprintf(
			"%s: the response carries UNDECLARED field %q (%s). Re-freeze the contract so the addition is reviewed.",
			mode, field.Path, field.Type)})
	}
	return drifts
}

// Details flattens drifts into their printable sentences.
func Details(drifts []Drift) []string {
	lines := make([]string, 0, len(drifts))
	for _, drift := range drifts {
		lines = append(lines, drift.Detail)
	}
	return lines
}

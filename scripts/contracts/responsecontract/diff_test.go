// diff_test.go — Proves the gate DISCRIMINATES: it goes red on a real drift and
// stays green otherwise, and every message names the mode and the field.
//
// A gate that never fails and a gate that always fails are equally useless. Each
// test here mutates one declared shape and asserts both that the drift is
// reported and that the report is specific enough to act on.
package responsecontract

import (
	"strings"
	"testing"
)

// declared is a shape modelled on observe/errors as it actually ships.
func declared() Shape {
	return Shape{Kind: kindDirect, Fields: []Field{
		{Path: "count", Type: "number"},
		{Path: "errors", Type: "array"},
		{Path: "errors[]", Type: "object"},
		{Path: "errors[].message", Type: "string"},
		{Path: "metadata", Type: "object"},
		{Path: "metadata.is_stale", Type: "boolean"},
		{Path: "scope", Type: "string"},
	}}
}

// mutate copies the declared shape and applies one change. Every mutation used
// below is ordinary Go that compiles, so the control cannot rot into a comment.
func mutate(change func(fields []Field) []Field) Shape {
	base := declared()
	return Shape{Kind: base.Kind, Fields: change(append([]Field(nil), base.Fields...))}
}

// drop removes the field at path. Removing by path rather than by index keeps
// the control readable and survives a reordering of the declared list.
func drop(path string) func([]Field) []Field {
	return func(fields []Field) []Field {
		kept := make([]Field, 0, len(fields))
		for _, field := range fields {
			if field.Path != path {
				kept = append(kept, field)
			}
		}
		return kept
	}
}

// rename replaces the field at path with a new path, keeping its type.
func rename(from, to string) func([]Field) []Field {
	return func(fields []Field) []Field {
		for index, field := range fields {
			if field.Path == from {
				fields[index] = Field{Path: to, Type: field.Type}
			}
		}
		return fields
	}
}

// retype changes the declared type at path without moving it.
func retype(path, newType string) func([]Field) []Field {
	return func(fields []Field) []Field {
		for index, field := range fields {
			if field.Path == path {
				fields[index] = Field{Path: path, Type: newType}
			}
		}
		return fields
	}
}

func TestAnUnchangedResponseProducesNoDrift(t *testing.T) {
	t.Parallel()
	// The other half of discrimination: if this failed, every drift below would
	// be noise rather than signal.
	if drifts := Diff("observe/errors", declared(), declared()); len(drifts) != 0 {
		t.Fatalf("an identical shape reported drift: %v", Details(drifts))
	}
}

func TestADroppedFieldIsReportedByName(t *testing.T) {
	t.Parallel()
	shipped := mutate(drop("metadata.is_stale"))

	drifts := Diff("observe/errors", declared(), shipped)
	assertNames(t, drifts, "observe/errors", "metadata.is_stale", "GONE")
}

func TestARenamedFieldIsReportedAsBothADropAndAnAddition(t *testing.T) {
	t.Parallel()
	// A rename is the drift the old regex table could not see: cat-33 matched
	// "result" and a handler could rename every key under it and still pass.
	shipped := mutate(rename("count", "total"))

	drifts := Diff("observe/errors", declared(), shipped)
	assertNames(t, drifts, "observe/errors", "count", "GONE")
	assertNames(t, drifts, "observe/errors", "total", "UNDECLARED")
}

func TestAChangedTypeIsReportedWithBothTypes(t *testing.T) {
	t.Parallel()
	shipped := mutate(retype("count", "string"))

	drifts := Diff("observe/errors", declared(), shipped)
	assertNames(t, drifts, "observe/errors", "count", "CHANGED TYPE")
	if !strings.Contains(Details(drifts)[0], "declared number") || !strings.Contains(Details(drifts)[0], "carries string") {
		t.Fatalf("the message does not name both types: %q", Details(drifts)[0])
	}
}

func TestAFieldRenamedInsideAnArrayElementIsCaught(t *testing.T) {
	t.Parallel()
	// The keys inside a list are where a payload quietly changes: nothing above
	// this contract ever asserted them.
	shipped := mutate(rename("errors[].message", "errors[].text"))

	drifts := Diff("observe/errors", declared(), shipped)
	assertNames(t, drifts, "observe/errors", "errors[].message", "GONE")
	assertNames(t, drifts, "observe/errors", "errors[].text", "UNDECLARED")
}

func TestADirectPayloadTurningIntoAnEnvelopeIsCaught(t *testing.T) {
	t.Parallel()
	// The dual shape the issue calls folklore: a daemon-local mode that starts
	// answering with a queued envelope has changed its contract completely,
	// and every field moves under .result.
	shipped := Shape{Kind: kindEnvelope, Fields: declared().Fields}

	drifts := Diff("observe/errors", declared(), shipped)
	if len(drifts) == 0 || !strings.Contains(drifts[0].Detail, "response kind changed") {
		t.Fatalf("a direct payload becoming an envelope was not reported: %v", Details(drifts))
	}
	if !strings.Contains(drifts[0].Detail, "observe/errors") {
		t.Fatalf("the message does not name the mode: %q", drifts[0].Detail)
	}
}

func TestANullableFieldSampledAsNullDoesNotFakeATypeDrift(t *testing.T) {
	t.Parallel()
	// null is "no value", not a type. Failing when a nullable field later
	// carries a string would report the fixture's data as contract drift and
	// train people to re-freeze without reading the diff.
	nullable := Shape{Kind: kindDirect, Fields: []Field{{Path: "errors[].line", Type: "null"}}}
	typed := Shape{Kind: kindDirect, Fields: []Field{{Path: "errors[].line", Type: "number"}}}

	if drifts := Diff("observe/errors", nullable, typed); len(drifts) != 0 {
		t.Fatalf("a nullable field reported drift: %v", Details(drifts))
	}
	// The PATH is still gated, which is where a rename shows up.
	gone := Shape{Kind: kindDirect, Fields: []Field{{Path: "errors[].row", Type: "number"}}}
	if drifts := Diff("observe/errors", nullable, gone); len(drifts) != 2 {
		t.Fatalf("a renamed nullable field was not caught: %v", Details(drifts))
	}
}

// assertNames requires a drift naming the mode, the field, and the kind of move.
func assertNames(t *testing.T, drifts []Drift, mode, field, verb string) {
	t.Helper()
	for _, drift := range drifts {
		if drift.Mode == mode && drift.Field == field && strings.Contains(drift.Detail, verb) {
			return
		}
	}
	t.Fatalf("no drift named %s field %q as %s; got:\n  %s", mode, field, verb, strings.Join(Details(drifts), "\n  "))
}

// main_test.go — Pins the wire-decode contract scanner.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeGo(t *testing.T, root, rel, source string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsUnmarshalIntoAWireType(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/probe/handler.go", `package probe

import (
	"encoding/json"

	"example.com/styleprobe"
)

func run(raw []byte) {
	var probe styleprobe.WireStyleProbeResult
	_ = json.Unmarshal(raw, &probe)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 {
		t.Fatalf("sites = %+v, want one", sites)
	}
	if sites[0].Type != "WireStyleProbeResult" {
		t.Errorf("Type = %q, want WireStyleProbeResult", sites[0].Type)
	}
	if sites[0].File != "internal/probe/handler.go" {
		t.Errorf("File = %q, want a repo-relative path", sites[0].File)
	}
}

func TestScanFindsDecoderDecodeIntoAWireType(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/perftrace/http.go", `package perftrace

import (
	"encoding/json"
	"io"
)

func run(r io.Reader) {
	var req WirePerformanceTraceStartRequest
	_ = json.NewDecoder(r).Decode(&req)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Type != "WirePerformanceTraceStartRequest" {
		t.Fatalf("sites = %+v, want the decoder site", sites)
	}
}

// A decoder held in a variable is the shape decodePOST uses, and it is the one
// that shipped the defect, so the scanner must follow it.
func TestScanFollowsADecoderHeldInAVariable(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/perftrace/http.go", `package perftrace

import (
	"encoding/json"
	"io"
)

func run(r io.Reader) {
	var req WireTraceChunk
	decoder := json.NewDecoder(r)
	_ = decoder.Decode(&req)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Type != "WireTraceChunk" {
		t.Fatalf("sites = %+v, want the variable-held decoder site", sites)
	}
}

// The perftrace endpoints hid their decode behind a helper taking `dst any`,
// so the wire type was invisible at the decode itself. Following the helper is
// what makes the contract cover the sites that actually shipped the defect.
func TestScanFollowsAGenericDecodeHelperCalledWithAWireType(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/perftrace/http.go", `package perftrace

import (
	"encoding/json"
	"io"
)

func decodePOST(r io.Reader, dst any) bool {
	return json.NewDecoder(r).Decode(dst) == nil
}

func run(r io.Reader) {
	var req WireTraceStart
	_ = decodePOST(r, &req)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Type != "WireTraceStart" {
		t.Fatalf("sites = %+v, want the helper call flagged with the wire type", sites)
	}
	if sites[0].File != "internal/perftrace/http.go" {
		t.Errorf("File = %q, want the call site's file", sites[0].File)
	}
}

// mcp.ParseArgs is a generic decode helper used by every tool. It only becomes
// a wire-decode site when a wire type is handed to it, so a helper called with
// an ordinary params struct must stay silent or the contract is unusable.
func TestScanIgnoresAGenericHelperCalledWithANonWireType(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/mcp/args.go", `package mcp

import "encoding/json"

func ParseArgs(raw []byte, dest any) error {
	return json.Unmarshal(raw, dest)
}
`)
	writeGo(t, root, "cmd/tool/handler.go", `package tool

type auditParams struct{}

func run(raw []byte) {
	var params auditParams
	_ = mcp.ParseArgs(raw, &params)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Fatalf("sites = %+v, want none", sites)
	}
}

// A helper that merely receives a wire value without decoding into it is not a
// decode site, and treating it as one would make the contract noise.
func TestScanIgnoresANonDecodingHelperPassedAWireType(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/thing/thing.go", `package thing

func record(value any) {}

func run() {
	var probe WireProbe
	record(&probe)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Fatalf("sites = %+v, want a non-decoding helper ignored", sites)
	}
}

// The convention is a Wire prefix followed by a capital, not any identifier
// beginning with those four letters. Matching the prefix alone would drag
// unrelated types into a contract written for peer payloads.
func TestScanIgnoresTypesMerelyStartingWithWire(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/thing/thing.go", `package thing

import "encoding/json"

func run(raw []byte) {
	var frame Wireframe
	_ = json.Unmarshal(raw, &frame)
	var w Wire
	_ = json.Unmarshal(raw, &w)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Fatalf("sites = %+v, want Wireframe and Wire ignored", sites)
	}
}

func TestScanIgnoresDecodesIntoNonWireTypes(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/thing/thing.go", `package thing

import "encoding/json"

type payload struct{}

func run(raw []byte) {
	var p payload
	_ = json.Unmarshal(raw, &p)
	var generic map[string]any
	_ = json.Unmarshal(raw, &generic)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Fatalf("sites = %+v, want none", sites)
	}
}

// Wire types appear in slices too, and a slice of them decoded loosely has the
// same failure mode as a single one.
func TestScanFindsSliceOfWireTypes(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/thing/thing.go", `package thing

import "encoding/json"

func run(raw []byte) {
	var logs []types.WireLog
	_ = json.Unmarshal(raw, &logs)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Type != "WireLog" {
		t.Fatalf("sites = %+v, want the slice element type", sites)
	}
}

// Tests construct payloads deliberately, including malformed ones, so gating
// them would only push authors into indirection.
func TestScanIgnoresTestFiles(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/probe/handler_test.go", `package probe

import "encoding/json"

func run(raw []byte) {
	var probe WireThing
	_ = json.Unmarshal(raw, &probe)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Fatalf("sites = %+v, want test files ignored", sites)
	}
}

// The decoder itself must decode; exempting it by name keeps the contract from
// forbidding its own implementation.
func TestScanIgnoresTheWirecodecPackage(t *testing.T) {
	root := t.TempDir()
	writeGo(t, root, "internal/wirecodec/decode.go", `package wirecodec

import "encoding/json"

func run(raw []byte) {
	var probe WireThing
	_ = json.Unmarshal(raw, &probe)
}
`)
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Fatalf("sites = %+v, want the decoder package ignored", sites)
	}
}

func TestEvaluateReportsAnUnexemptedSite(t *testing.T) {
	sites := []site{{File: "internal/probe/handler.go", Line: 9, Type: "WireProbe"}}
	violations := evaluate(sites, nil)
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one", violations)
	}
	if !contains(violations[0], "wirecodec") {
		t.Errorf("violation = %q, want it to name the remedy", violations[0])
	}
}

func TestEvaluateAcceptsAnExemptedSite(t *testing.T) {
	sites := []site{{File: "internal/qafixture/wire_fixture.go", Type: "WireQAFixture"}}
	exemptions := []exemption{{
		File:   "internal/qafixture/wire_fixture.go",
		Type:   "WireQAFixture",
		Reason: "author-facing fixture documents reject unknown fields outright, which is stricter",
	}}
	if violations := evaluate(sites, exemptions); len(violations) != 0 {
		t.Fatalf("violations = %v, want none", violations)
	}
}

// An exemption that no longer matches a real site is the mechanism going stale.
// Reporting it is what stops the allow-list outliving the code it excused.
func TestEvaluateReportsAStaleExemption(t *testing.T) {
	exemptions := []exemption{{File: "internal/gone/old.go", Type: "WireGone", Reason: "historic"}}
	violations := evaluate(nil, exemptions)
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want the stale exemption reported", violations)
	}
	if !contains(violations[0], "internal/gone/old.go") {
		t.Errorf("violation = %q, want it to name the stale entry", violations[0])
	}
}

// A reasonless exemption is an allow-list entry nobody can review.
func TestEvaluateRejectsAnExemptionWithoutAReason(t *testing.T) {
	sites := []site{{File: "internal/probe/handler.go", Type: "WireProbe"}}
	exemptions := []exemption{{File: "internal/probe/handler.go", Type: "WireProbe"}}
	violations := evaluate(sites, exemptions)
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want the reasonless exemption rejected", violations)
	}
	if !contains(violations[0], "reason") {
		t.Errorf("violation = %q, want it to name the missing reason", violations[0])
	}
}

// An exemption is scoped to one type in one file; the same type elsewhere is a
// separate decision.
func TestEvaluateDoesNotLetAnExemptionCoverAnotherFile(t *testing.T) {
	sites := []site{
		{File: "internal/a/a.go", Type: "WireThing"},
		{File: "internal/b/b.go", Type: "WireThing"},
	}
	exemptions := []exemption{{File: "internal/a/a.go", Type: "WireThing", Reason: "documented"}}
	violations := evaluate(sites, exemptions)
	if len(violations) != 1 || !contains(violations[0], "internal/b/b.go") {
		t.Fatalf("violations = %v, want only the unexempted file reported", violations)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

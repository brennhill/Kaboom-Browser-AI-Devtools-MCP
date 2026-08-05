// flags_test.go — Unit tests for the parser's private flag-decoding primitives.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package parser

import "testing"

func TestParseFlagsBySpec_KindsAndErrors(t *testing.T) {
	t.Parallel()

	specs := map[string]cliFlagSpec{
		"--str":  {MCPKey: "str", Kind: FlagString},
		"--int":  {MCPKey: "int", Kind: FlagInt},
		"--json": {MCPKey: "json", Kind: FlagJSON},
		"--list": {MCPKey: "list", Kind: FlagStringList},
		"--jos":  {MCPKey: "jos", Kind: FlagJSONOrString},
		"--ios":  {MCPKey: "ios", Kind: FlagIntOrString},
		"--bool": {MCPKey: "bool", Kind: FlagBool},
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"unknown flag", []string{"--nope"}, true},
		{"missing string value", []string{"--str"}, true},
		{"string value is another flag", []string{"--str", "--bool"}, true},
		{"invalid int", []string{"--int", "abc"}, true},
		{"missing int value", []string{"--int"}, true},
		{"invalid json", []string{"--json", "{bad"}, true},
		{"missing json value", []string{"--json"}, true},
		{"missing list value", []string{"--list"}, true},
		{"missing jos value", []string{"--jos"}, true},
		{"missing ios value", []string{"--ios"}, true},
		{"valid string", []string{"--str", "hello"}, false},
		{"valid int", []string{"--int", "42"}, false},
		{"valid json object", []string{"--json", `{"a":1}`}, false},
		{"valid list", []string{"--list", "a,b,c"}, false},
		{"jos plain string", []string{"--jos", "plain"}, false},
		{"jos json array", []string{"--jos", `[1,2]`}, false},
		{"ios integer", []string{"--ios", "42"}, false},
		{"ios string", []string{"--ios", "top"}, false},
		{"bool flag", []string{"--bool"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseFlagsBySpec(tc.args, specs)
			if tc.wantErr && err == nil {
				t.Fatalf("parseFlagsBySpec(%v) error = nil, want error", tc.args)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("parseFlagsBySpec(%v) error = %v, want nil", tc.args, err)
			}
		})
	}
}

func TestParseFlagsBySpec_UnsupportedKind(t *testing.T) {
	t.Parallel()

	specs := map[string]cliFlagSpec{"--x": {MCPKey: "x", Kind: cliFlagKind(99)}}
	if _, err := parseFlagsBySpec([]string{"--x"}, specs); err == nil {
		t.Fatal("parseFlagsBySpec() error = nil, want error for unsupported kind")
	}
}

func TestParseFlagsBySpec_ValuesMapCorrectly(t *testing.T) {
	t.Parallel()

	specs := map[string]cliFlagSpec{
		"--ios": {MCPKey: "frame", Kind: FlagIntOrString},
	}
	out, err := parseFlagsBySpec([]string{"--ios", "5"}, specs)
	if err != nil {
		t.Fatalf("parseFlagsBySpec() error = %v", err)
	}
	if out["frame"] != 5 {
		t.Fatalf("frame = %v (%T), want int 5", out["frame"], out["frame"])
	}

	out, err = parseFlagsBySpec([]string{"--ios", "main"}, specs)
	if err != nil {
		t.Fatalf("parseFlagsBySpec() error = %v", err)
	}
	if out["frame"] != "main" {
		t.Fatalf("frame = %v, want string main", out["frame"])
	}
}

func TestParseCSVList_AllEmptyYieldsEmptySlice(t *testing.T) {
	t.Parallel()

	got := parseCSVList("  ,  , ")
	if len(got) != 0 {
		t.Fatalf("parseCSVList() = %v, want empty slice", got)
	}
}

// cat33_test.go — Keeps the UAT sweep's content expectations from disagreeing
// with the declared response contract.
//
// PURPOSE: the 32 regexes in cat-33-expectations.sh were, until this contract
// existed, the ONLY written statement of what any MCP response contains. They
// were captured empirically because there was nowhere to read the answer from.
// Now there is. Where a mode has both, the field the regex names must exist in
// the declared shape — so an expectation cannot outlive the field it asserts,
// and a re-freeze that drops a field fails the sweep's expectation too.
//
// Docs: docs/features/feature/quality-gates/index.md
package responsecontract

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const expectationsFile = "scripts/tests/browser/cat-33-expectations.sh"

// expectationPattern matches one `tool/mode) echo '...' ;;` case arm.
var expectationPattern = regexp.MustCompile(`(?m)^\s+([a-z_]+/[a-z0-9_]+)\)\s+echo\s+'([^']+)'`)

// quotedField matches the "field" tokens inside an expectation's regex.
var quotedField = regexp.MustCompile(`"([a-z0-9_]+)"`)

// expectations reads the sweep's content expectations.
func expectations(t *testing.T) map[string]string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := RepoRoot(working)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, expectationsFile))
	if err != nil {
		t.Fatal(err)
	}
	table := map[string]string{}
	for _, match := range expectationPattern.FindAllStringSubmatch(string(raw), -1) {
		table[match[1]] = match[2]
	}
	if len(table) == 0 {
		t.Fatalf("no expectations parsed from %s; the parser is broken and every check below would pass vacuously", expectationsFile)
	}
	return table
}

// covers reports whether the shape declares path, or anything nested under it.
func covers(shape Shape, name string) bool {
	for _, field := range shape.Fields {
		if field.Path == name ||
			strings.HasPrefix(field.Path, name+".") ||
			strings.HasPrefix(field.Path, name+"[") {
			return true
		}
	}
	return false
}

func TestSweepExpectationsNameFieldsTheContractDeclares(t *testing.T) {
	t.Parallel()
	_, contract := fixture(t)
	table := expectations(t)

	var disagreements, compared []string
	for mode, expectation := range table {
		shape, declared := contract.Modes[mode]
		if !declared {
			continue
		}
		names := quotedField.FindAllStringSubmatch(expectation, -1)
		if len(names) == 0 {
			continue
		}
		compared = append(compared, mode)
		if anyCovered(shape, names) {
			continue
		}
		disagreements = append(disagreements, mode+": the sweep expects "+expectation+
			" but the declared response shape has no such field. One of the two is wrong, and until now nothing could tell you which.")
	}
	sort.Strings(disagreements)

	if len(compared) == 0 {
		t.Fatal("control: no mode had both an expectation and a declared shape, so this check proved nothing")
	}
	if len(disagreements) > 0 {
		t.Fatalf("%d of %d compared mode(s) have a sweep expectation the contract contradicts:\n  %s",
			len(disagreements), len(compared), strings.Join(disagreements, "\n  "))
	}
	t.Logf("%d mode(s) have both a sweep expectation and a declared shape, and they agree", len(compared))
}

// anyCovered reports whether the shape declares any of the named fields. The
// expectations are alternations ("forms"|"count"), so one match is agreement.
func anyCovered(shape Shape, names [][]string) bool {
	for _, name := range names {
		if covers(shape, name[1]) {
			return true
		}
	}
	return false
}

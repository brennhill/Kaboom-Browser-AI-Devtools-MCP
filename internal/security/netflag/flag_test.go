// flag_test.go — Defines the canonical network-security finding contract.
package netflag

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFlagOwnsSnakeCaseNetworkFindingContract(t *testing.T) {
	encoded, err := json.Marshal(Flag{
		Type: "mixed_content", Severity: "high", Origin: "https://example.test",
		Resource: "http://example.test/script.js", PageURL: "https://example.test", Timestamp: time.Unix(1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"type"`, `"severity"`, `"origin"`, `"resource"`, `"page_url"`, `"timestamp"`} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("encoded flag %s missing %s", encoded, field)
		}
	}
}

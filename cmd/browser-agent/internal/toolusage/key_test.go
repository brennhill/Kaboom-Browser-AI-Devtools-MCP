// key_test.go — Tests privacy-safe MCP product-usage key derivation.

package toolusage

import (
	"encoding/json"
	"testing"
)

func TestKeyDerivesActionWithoutRetainingArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args json.RawMessage
		want string
	}{
		{"ordinary action", json.RawMessage(`{"what":"errors","selector":"#private"}`), "errors"},
		{"missing action", json.RawMessage(`{"selector":"#private"}`), ""},
		{"empty object", json.RawMessage(`{}`), ""},
		{"malformed JSON", json.RawMessage(`{not-json`), ""},
		{"absent arguments", nil, ""},
		{"command result prefix", json.RawMessage(`{"what":"command_result","correlation_id":"nav_1708300000_123"}`), "command_result:nav"},
		{"command result without ID", json.RawMessage(`{"what":"command_result"}`), "command_result"},
		{"opaque command ID", json.RawMessage(`{"what":"command_result","correlation_id":"plainid"}`), "command_result:plainid"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Key(test.args); got != test.want {
				t.Errorf("Key(%s) = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

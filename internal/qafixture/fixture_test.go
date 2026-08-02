// fixture_test.go — Tests the declarative QA fixture contract and validation.

package qafixture

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestParseValidFixture(t *testing.T) {
	raw := json.RawMessage(`{
		"version":1,
		"target":{"url":"https://example.test/account"},
		"viewport":{"width":1280,"height":720},
		"locale":"en-US",
		"permissions":["geolocation","notifications"],
		"network":{"profile":"fast_3g"},
		"cookies":[{"name":"session","value":"private","path":"/"}],
		"local_storage":{"feature":"enabled"},
		"session_storage":{"journey":"checkout"},
		"feature_flags":{"new_checkout":true},
		"seed_data":{"cart":{"items":2}},
		"user_state":"returning",
		"auth_role":"member",
		"setup_timeout_ms":5000
	}`)
	fixture, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if fixture.Version != CurrentVersion || fixture.Viewport.Width != 1280 || fixture.Network.Profile != "fast_3g" {
		t.Fatalf("fixture decoded incorrectly: %+v", fixture)
	}
}

func TestParseRejectsUnsafeOrUnsupportedFixtures(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"malformed", `{`, "invalid fixture JSON"},
		{"unknown field", `{"version":1,"private_token":"do-not-repeat"}`, "unknown field"},
		{"unsupported version", `{"version":2}`, "unsupported fixture version"},
		{"unsafe target", `{"version":1,"target":{"url":"javascript:alert(1)"}}`, "target.url"},
		{"invalid viewport", `{"version":1,"viewport":{"width":10,"height":720}}`, "viewport.width"},
		{"unknown permission", `{"version":1,"permissions":["filesystem"]}`, "permissions"},
		{"duplicate permission", `{"version":1,"permissions":["camera","camera"]}`, "permissions contains a duplicate"},
		{"unknown network profile", `{"version":1,"network":{"profile":"satellite"}}`, "network.profile"},
		{"invalid cookie name", `{"version":1,"cookies":[{"name":"bad;name","value":"secret"}]}`, "cookie name"},
		{"invalid cookie same site", `{"version":1,"cookies":[{"name":"session","value":"secret","same_site":"sometimes"}]}`, "same_site"},
		{"invalid user state", `{"version":1,"user_state":"sometimes"}`, "user_state"},
		{"timeout too large", `{"version":1,"setup_timeout_ms":60000}`, "setup_timeout_ms"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(json.RawMessage(tt.raw))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Parse error = %v, want containing %q", err, tt.want)
			}
			if strings.Contains(err.Error(), "do-not-repeat") {
				t.Fatalf("error leaked fixture value: %v", err)
			}
		})
	}
}

func TestParseRejectsTooManyStateEntries(t *testing.T) {
	entries := make(map[string]string, maxStateEntries+1)
	for index := 0; index <= maxStateEntries; index++ {
		entries[fmt.Sprintf("key_%d", index)] = "value"
	}
	raw, err := json.Marshal(map[string]any{"version": 1, "local_storage": entries})
	if err != nil {
		t.Fatal(err)
	}
	_, parseErr := Parse(raw)
	if parseErr == nil || !strings.Contains(parseErr.Error(), "local_storage exceeds") {
		t.Fatalf("Parse error = %v, want local_storage cardinality error", parseErr)
	}
}

func TestParseAppliesBoundedDefaultTimeout(t *testing.T) {
	fixture, err := Parse(json.RawMessage(`{"version":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if fixture.SetupTimeoutMs != DefaultSetupTimeoutMs {
		t.Fatalf("setup_timeout_ms = %d, want %d", fixture.SetupTimeoutMs, DefaultSetupTimeoutMs)
	}
}

func TestParseRejectsOversizedStateWithoutLeakingIt(t *testing.T) {
	secret := strings.Repeat("s", MaxStateBytes+1)
	raw, err := json.Marshal(map[string]any{
		"version":       1,
		"local_storage": map[string]string{"secret": secret},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, parseErr := Parse(raw)
	if parseErr == nil || !strings.Contains(parseErr.Error(), "state payload exceeds") {
		t.Fatalf("Parse error = %v", parseErr)
	}
	if strings.Contains(parseErr.Error(), secret) {
		t.Fatal("error leaked oversized state value")
	}
}

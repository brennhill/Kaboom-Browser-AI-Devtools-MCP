// Purpose: Unit tests for redaction logic.
// Docs: docs/features/feature/redaction-patterns/index.md

package redaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestNewRedactionEngine_CustomPatterns(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "redaction.json")
	config := `{
		"patterns": [
			{"name":"custom-secret","pattern":"secret_[0-9]+"},
			{"name":"explicit-replacement","pattern":"token=[a-z]+","replacement":"token=[MASKED]"},
			{"name":"invalid-regex","pattern":"[unclosed"}
		]
	}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	engine := NewRedactionEngine(configPath)
	got := engine.Redact("secret_42 token=abc")
	if !strings.Contains(got, "[REDACTED:custom-secret]") {
		t.Fatalf("expected default replacement for custom-secret, got %q", got)
	}
	if !strings.Contains(got, "token=[MASKED]") {
		t.Fatalf("expected explicit replacement, got %q", got)
	}
}

func TestRedact_CreditCardLuhnValidation(t *testing.T) {
	t.Parallel()

	engine := NewRedactionEngine("")

	valid := engine.Redact("card 4111 1111 1111 1111")
	if !strings.Contains(valid, "[REDACTED:credit-card]") {
		t.Fatalf("expected valid card to be redacted, got %q", valid)
	}

	invalid := engine.Redact("card 4111 1111 1111 1112")
	if strings.Contains(invalid, "[REDACTED:credit-card]") {
		t.Fatalf("expected invalid card to remain, got %q", invalid)
	}
}

func TestRedactMapValues_RedactsSensitiveKeyNames(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	input := map[string]any{
		"username":    "alice",
		"password":    "s3cret",
		"token":       "abc123",
		"secret":      "mysecret",
		"ssn":         "123-45-6789",
		"credit_card": "4111111111111111",
		"cvv":         "123",
		"auth":        "bearer xyz",
		"normal_key":  "normal_value",
	}
	result := engine.RedactMapValues(input)

	// Sensitive keys should be redacted
	for _, key := range []string{"password", "token", "secret", "ssn", "credit_card", "cvv", "auth"} {
		v, ok := result[key].(string)
		if !ok {
			t.Fatalf("expected string value for key %q, got %T", key, result[key])
		}
		if !strings.Contains(v, "[REDACTED") {
			t.Errorf("key %q should be redacted, got %q", key, v)
		}
	}

	// Non-sensitive keys should pass through
	if result["username"] != "alice" {
		t.Errorf("username should be preserved, got %q", result["username"])
	}
	if result["normal_key"] != "normal_value" {
		t.Errorf("normal_key should be preserved, got %q", result["normal_key"])
	}
}

func TestRedactMapValues_RedactsSensitiveKeyNamePatterns(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	input := map[string]any{
		"user_password": "plain-password",
		"authToken":     "short-token",
		"apiKeyInput":   "dev-key",
		"display_name":  "alice",
	}
	result := engine.RedactMapValues(input)

	for _, key := range []string{"user_password", "authToken", "apiKeyInput"} {
		v, ok := result[key].(string)
		if !ok {
			t.Fatalf("expected string value for key %q, got %T", key, result[key])
		}
		if !strings.Contains(v, "[REDACTED") {
			t.Errorf("key %q should be redacted by key-name pattern, got %q", key, v)
		}
	}

	if result["display_name"] != "alice" {
		t.Errorf("display_name should be preserved, got %q", result["display_name"])
	}
}

func TestRedactMapValues_RedactsPatternMatches(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	input := map[string]any{
		"header":    "Bearer abc123def456",
		"note":      "Key is AKIAIOSFODNN7EXAMPLE",
		"safe_data": "nothing special here",
	}
	result := engine.RedactMapValues(input)

	if v := result["header"].(string); !strings.Contains(v, "[REDACTED:bearer-token]") {
		t.Errorf("bearer token pattern should be redacted, got %q", v)
	}
	if v := result["note"].(string); !strings.Contains(v, "[REDACTED:aws-key]") {
		t.Errorf("AWS key pattern should be redacted, got %q", v)
	}
	if result["safe_data"] != "nothing special here" {
		t.Errorf("safe_data should be preserved, got %q", result["safe_data"])
	}
}

func TestRedactMapValues_RedactsTokenLikeValues(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	input := map[string]any{
		"notes":     "sk-" + "1234567890abcdef1234567890abcdef",
		"slack_key": "xoxb-" + "123456789012-123456789012-abcdefghijklmnopqrstuvwx",
		"safe_data": "hello world",
	}
	result := engine.RedactMapValues(input)

	if v := result["notes"].(string); !strings.Contains(v, "[REDACTED") {
		t.Errorf("expected OpenAI-style token-like value redacted, got %q", v)
	}
	if v := result["slack_key"].(string); !strings.Contains(v, "[REDACTED") {
		t.Errorf("expected Slack-style token-like value redacted, got %q", v)
	}
	if result["safe_data"] != "hello world" {
		t.Errorf("safe_data should be preserved, got %q", result["safe_data"])
	}
}

func TestRedactMapValues_RecursesNestedMaps(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	input := map[string]any{
		"user": map[string]any{
			"name":     "alice",
			"password": "s3cret",
			"prefs": map[string]any{
				"theme":  "dark",
				"secret": "hidden",
			},
		},
		"top_level": "safe",
	}
	result := engine.RedactMapValues(input)

	user, ok := result["user"].(map[string]any)
	if !ok {
		t.Fatal("expected nested map for 'user'")
	}
	if user["name"] != "alice" {
		t.Errorf("nested name should be preserved, got %q", user["name"])
	}
	if v := user["password"].(string); !strings.Contains(v, "[REDACTED") {
		t.Errorf("nested password should be redacted, got %q", v)
	}

	prefs, ok := user["prefs"].(map[string]any)
	if !ok {
		t.Fatal("expected nested map for 'prefs'")
	}
	if prefs["theme"] != "dark" {
		t.Errorf("deeply nested theme should be preserved, got %q", prefs["theme"])
	}
	if v := prefs["secret"].(string); !strings.Contains(v, "[REDACTED") {
		t.Errorf("deeply nested secret should be redacted, got %q", v)
	}
}

func TestRedactMapValues_RecursesSlicesForLegacyPayloads(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	input := map[string]any{
		"form_values": []any{
			map[string]any{"authToken": "legacy-secret"},
			"sk-" + "1234567890abcdef1234567890abcdef",
			map[string]any{
				"nested": []any{
					map[string]any{"user_password": "another-secret"},
				},
			},
		},
	}

	result := engine.RedactMapValues(input)
	formValues, ok := result["form_values"].([]any)
	if !ok {
		t.Fatalf("form_values type = %T, want []any", result["form_values"])
	}

	first, ok := formValues[0].(map[string]any)
	if !ok {
		t.Fatalf("form_values[0] type = %T, want map[string]any", formValues[0])
	}
	if v := first["authToken"].(string); !strings.Contains(v, "[REDACTED") {
		t.Errorf("legacy authToken should be redacted, got %q", v)
	}

	if v, ok := formValues[1].(string); !ok || !strings.Contains(v, "[REDACTED") {
		t.Errorf("form_values[1] should be redacted string, got %T %q", formValues[1], v)
	}

	nestedMap, ok := formValues[2].(map[string]any)
	if !ok {
		t.Fatalf("form_values[2] type = %T, want map[string]any", formValues[2])
	}
	nestedSlice, ok := nestedMap["nested"].([]any)
	if !ok {
		t.Fatalf("nested type = %T, want []any", nestedMap["nested"])
	}
	deepMap, ok := nestedSlice[0].(map[string]any)
	if !ok {
		t.Fatalf("nested[0] type = %T, want map[string]any", nestedSlice[0])
	}
	if v := deepMap["user_password"].(string); !strings.Contains(v, "[REDACTED") {
		t.Errorf("deep nested user_password should be redacted, got %q", v)
	}
}

func TestRedactMapValues_PreservesNonSensitiveData(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	input := map[string]any{
		"url":        "https://example.com",
		"title":      "My Page",
		"tab_id":     float64(42),
		"is_active":  true,
		"saved_at":   "2024-01-01T00:00:00Z",
		"nil_value":  nil,
		"int_value":  float64(100),
		"bool_value": false,
	}
	result := engine.RedactMapValues(input)

	if result["url"] != "https://example.com" {
		t.Errorf("url should be preserved, got %v", result["url"])
	}
	if result["title"] != "My Page" {
		t.Errorf("title should be preserved, got %v", result["title"])
	}
	if result["tab_id"] != float64(42) {
		t.Errorf("tab_id should be preserved, got %v", result["tab_id"])
	}
	if result["is_active"] != true {
		t.Errorf("is_active should be preserved, got %v", result["is_active"])
	}
	if result["nil_value"] != nil {
		t.Errorf("nil_value should be preserved, got %v", result["nil_value"])
	}
}

func TestRedactJSON_StructuredBlocksOnly(t *testing.T) {
	t.Parallel()

	engine := NewRedactionEngine("")
	input := json.RawMessage(`{
		"content": [
			{"type":"text","text":"Authorization: Bearer abc123"},
			{"type":"image","text":"Authorization: Bearer keep-me"}
		],
		"isError": false
	}`)

	out := engine.RedactJSON(input)
	var parsed mcp.MCPToolResult
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(parsed.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(parsed.Content))
	}
	if !strings.Contains(parsed.Content[0].Text, "[REDACTED:bearer-token]") {
		t.Fatalf("expected text block redacted, got %q", parsed.Content[0].Text)
	}
	if parsed.Content[1].Text != "Authorization: Bearer keep-me" {
		t.Fatalf("non-text block should be untouched, got %q", parsed.Content[1].Text)
	}
}

func TestRedactJSON_PreservesMetadataField(t *testing.T) {
	t.Parallel()

	engine := NewRedactionEngine("")
	// Input has a metadata field that must survive the RedactJSON round-trip
	input := json.RawMessage(`{
		"content": [{"type":"text","text":"no secrets here"}],
		"isError": false,
		"metadata": {"telemetry_changed": true, "version": "0.7.3"}
	}`)

	out := engine.RedactJSON(input)

	// Parse the output and verify metadata survived
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
	metaRaw, ok := raw["metadata"]
	if !ok {
		t.Fatal("RedactJSON dropped the 'metadata' field during round-trip — B1 bug")
	}
	var meta map[string]any
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatalf("metadata should be valid JSON: %v", err)
	}
	if meta["telemetry_changed"] != true {
		t.Errorf("metadata.telemetry_changed should be true, got %v", meta["telemetry_changed"])
	}
}

func TestRedactJSON_PreservesEmptyTextKey(t *testing.T) {
	t.Parallel()

	engine := NewRedactionEngine("")
	// A content block with text:"" must keep the "text" key in output
	input := json.RawMessage(`{
		"content": [{"type":"text","text":""}],
		"isError": false
	}`)

	out := engine.RedactJSON(input)

	// Parse and check the text key is still present
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
	var content []map[string]any
	if err := json.Unmarshal(raw["content"], &content); err != nil {
		t.Fatalf("content should be array: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected 1 content block")
	}
	if _, hasText := content[0]["text"]; !hasText {
		t.Fatal("RedactJSON dropped empty 'text' key due to omitempty — B2 bug")
	}
}

func TestRedactJSON_FallbackForMalformedInput(t *testing.T) {
	t.Parallel()

	engine := NewRedactionEngine("")
	input := json.RawMessage(`{"content":[{"type":"text","text":"AKIAIOSFODNN7EXAMPLE"}]`)
	out := engine.RedactJSON(input)
	if !strings.Contains(string(out), "[REDACTED:aws-key]") {
		t.Fatalf("expected fallback raw-string redaction, got %q", string(out))
	}
}

func TestLuhnValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		valid bool
	}{
		{value: "4111111111111111", valid: true},
		{value: "4111-1111-1111-1111", valid: true},
		{value: "4111 1111 1111 1111", valid: true},
		{value: "4111111111111112", valid: false},
		{value: "123456", valid: false},
	}

	for _, tt := range tests {
		if got := luhnValid(tt.value); got != tt.valid {
			t.Fatalf("luhnValid(%q) = %v, want %v", tt.value, got, tt.valid)
		}
	}
}

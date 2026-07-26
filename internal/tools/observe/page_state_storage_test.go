// storage_test.go — Tests for storage filtering and summarization.
package observe

import (
	"encoding/json"
	"testing"
)

func assertSummaryShape(t *testing.T, result map[string]any, wantKeyCount, wantSampleKeys int) []string {
	t.Helper()
	if result["key_count"] != wantKeyCount {
		t.Errorf("key_count = %v, want %d", result["key_count"], wantKeyCount)
	}
	sampleKeys, ok := result["sample_keys"].([]string)
	if !ok {
		t.Fatal("sample_keys not a []string")
	}
	if len(sampleKeys) != wantSampleKeys {
		t.Errorf("sample_keys len = %d, want %d", len(sampleKeys), wantSampleKeys)
	}
	return sampleKeys
}

func assertSummaryTotalBytes(t *testing.T, result map[string]any, want int) int {
	t.Helper()
	totalBytes := summaryTotalBytes(t, result)
	if totalBytes != want {
		t.Errorf("total_bytes = %d, want %d", totalBytes, want)
	}
	return totalBytes
}

func summaryTotalBytes(t *testing.T, result map[string]any) int {
	t.Helper()
	totalBytes, ok := result["total_bytes"].(int)
	if !ok {
		t.Fatal("total_bytes not an int")
	}
	return totalBytes
}

// ---------------------------------------------------------------------------
// summarizeStorageMap
// ---------------------------------------------------------------------------

func TestSummarizeStorageMap_Empty(t *testing.T) {
	t.Parallel()
	result := summarizeStorageMap(map[string]any{})

	assertSummaryShape(t, result, 0, 0)
	assertSummaryTotalBytes(t, result, 0)
}

func TestSummarizeStorageMap_Normal(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"theme":    "dark",
		"language": "en",
	}
	result := summarizeStorageMap(data)

	assertSummaryShape(t, result, 2, 2)

	// total_bytes = len("theme") + len("dark") + len("language") + len("en")
	//             = 5 + 4 + 8 + 2 = 19
	assertSummaryTotalBytes(t, result, 19)
}

func TestSummarizeStorageMap_NonStringValue(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"count": 42,
	}
	result := summarizeStorageMap(data)

	assertSummaryShape(t, result, 1, 1)

	// total_bytes = len("count") + len(json.Marshal(42)) = 5 + 2 = 7
	assertSummaryTotalBytes(t, result, 7)
}

func TestSummarizeStorageMap_SampleKeysCappedAt5(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"a": "1", "b": "2", "c": "3",
		"d": "4", "e": "5", "f": "6",
		"g": "7",
	}
	result := summarizeStorageMap(data)

	assertSummaryShape(t, result, 7, 5)
}

// ---------------------------------------------------------------------------
// summarizeCookies
// ---------------------------------------------------------------------------

func TestSummarizeCookies_Empty(t *testing.T) {
	t.Parallel()
	result := summarizeCookies([]any{})

	assertSummaryShape(t, result, 0, 0)
	assertSummaryTotalBytes(t, result, 0)
}

func TestSummarizeCookies_Normal(t *testing.T) {
	t.Parallel()
	cookies := []any{
		map[string]any{"name": "session_id", "value": "abc123"},
		map[string]any{"name": "theme", "value": "dark"},
	}
	result := summarizeCookies(cookies)

	sampleKeys := assertSummaryShape(t, result, 2, 2)

	// Verify both cookie names appear in sample_keys
	nameSet := make(map[string]bool)
	for _, k := range sampleKeys {
		nameSet[k] = true
	}
	if !nameSet["session_id"] || !nameSet["theme"] {
		t.Errorf("sample_keys = %v, want [session_id, theme]", sampleKeys)
	}

	totalBytes := summaryTotalBytes(t, result)
	if totalBytes <= 0 {
		t.Errorf("total_bytes = %d, want > 0", totalBytes)
	}
}

func TestSummarizeCookies_MissingNameField(t *testing.T) {
	t.Parallel()
	cookies := []any{
		map[string]any{"value": "no-name-here"},
		map[string]any{"name": "valid", "value": "ok"},
	}
	result := summarizeCookies(cookies)

	// key_count is len(cookies) which is 2
	// Only the cookie with a name field should appear in sample_keys
	sampleKeys := assertSummaryShape(t, result, 2, 1)
	if len(sampleKeys) != 1 {
		t.Errorf("sample_keys len = %d, want 1", len(sampleKeys))
	}
	if len(sampleKeys) > 0 && sampleKeys[0] != "valid" {
		t.Errorf("sample_keys[0] = %q, want 'valid'", sampleKeys[0])
	}
}

func TestSummarizeCookies_SampleCappedAt5(t *testing.T) {
	t.Parallel()
	cookies := make([]any, 8)
	for i := range cookies {
		cookies[i] = map[string]any{"name": string(rune('a' + i)), "value": "v"}
	}
	result := summarizeCookies(cookies)

	assertSummaryShape(t, result, 8, 5)
}

func TestSummarizeCookies_NonMapEntry(t *testing.T) {
	t.Parallel()
	// A non-map entry should be counted in key_count but not contribute a name
	cookies := []any{
		"not-a-map",
		map[string]any{"name": "real", "value": "ok"},
	}
	result := summarizeCookies(cookies)

	sampleKeys := assertSummaryShape(t, result, 2, 1)
	if len(sampleKeys) != 1 {
		t.Errorf("sample_keys len = %d, want 1 (only map entries contribute names)", len(sampleKeys))
	}
}

// ---------------------------------------------------------------------------
// parseStorageParams
// ---------------------------------------------------------------------------

func TestParseStorageParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        json.RawMessage
		wantSummary bool
		wantType    string
		wantKey     string
	}{
		{
			name:        "nil args",
			args:        nil,
			wantSummary: true,
			wantType:    "",
			wantKey:     "",
		},
		{
			name:        "empty args",
			args:        json.RawMessage(``),
			wantSummary: true,
			wantType:    "",
			wantKey:     "",
		},
		{
			name:        "empty object",
			args:        json.RawMessage(`{}`),
			wantSummary: true,
			wantType:    "",
			wantKey:     "",
		},
		{
			name:        "summary false only",
			args:        json.RawMessage(`{"summary":false}`),
			wantSummary: false,
			wantType:    "",
			wantKey:     "",
		},
		{
			name:        "storage_type local",
			args:        json.RawMessage(`{"storage_type":"local"}`),
			wantSummary: true,
			wantType:    "local",
			wantKey:     "",
		},
		{
			name:        "storage_type session",
			args:        json.RawMessage(`{"storage_type":"session"}`),
			wantSummary: true,
			wantType:    "session",
			wantKey:     "",
		},
		{
			name:        "storage_type cookies",
			args:        json.RawMessage(`{"storage_type":"cookies"}`),
			wantSummary: true,
			wantType:    "cookies",
			wantKey:     "",
		},
		{
			name:        "key only",
			args:        json.RawMessage(`{"key":"theme"}`),
			wantSummary: true,
			wantType:    "",
			wantKey:     "theme",
		},
		{
			name:        "all fields set",
			args:        json.RawMessage(`{"summary":false,"storage_type":"local","key":"user_id"}`),
			wantSummary: false,
			wantType:    "local",
			wantKey:     "user_id",
		},
		{
			name:        "invalid JSON falls back to defaults",
			args:        json.RawMessage(`{bad json`),
			wantSummary: true,
			wantType:    "",
			wantKey:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseStorageParams(tc.args)
			if got.Summary != tc.wantSummary {
				t.Errorf("Summary = %v, want %v", got.Summary, tc.wantSummary)
			}
			if got.StorageType != tc.wantType {
				t.Errorf("StorageType = %q, want %q", got.StorageType, tc.wantType)
			}
			if got.Key != tc.wantKey {
				t.Errorf("Key = %q, want %q", got.Key, tc.wantKey)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// filterStorageMap
// ---------------------------------------------------------------------------

func TestFilterStorageMap(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"theme":    "dark",
		"language": "en",
		"count":    42,
	}

	tests := []struct {
		name     string
		key      string
		wantLen  int
		wantKeys []string
	}{
		{
			name:     "empty key returns all",
			key:      "",
			wantLen:  3,
			wantKeys: []string{"theme", "language", "count"},
		},
		{
			name:     "key found returns single entry",
			key:      "theme",
			wantLen:  1,
			wantKeys: []string{"theme"},
		},
		{
			name:     "key not found returns empty map",
			key:      "nonexistent",
			wantLen:  0,
			wantKeys: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterStorageMap(data, tc.key)
			if len(got) != tc.wantLen {
				t.Errorf("len = %d, want %d", len(got), tc.wantLen)
			}
			for _, k := range tc.wantKeys {
				if _, ok := got[k]; !ok {
					t.Errorf("missing key %q in result", k)
				}
			}
		})
	}
}

func TestFilterStorageMap_PreservesValue(t *testing.T) {
	t.Parallel()
	data := map[string]any{"key1": "value1"}
	got := filterStorageMap(data, "key1")
	if got["key1"] != "value1" {
		t.Errorf("value = %v, want 'value1'", got["key1"])
	}
}

func TestFilterStorageMap_EmptyData(t *testing.T) {
	t.Parallel()

	// Empty key on empty map returns the empty map
	got := filterStorageMap(map[string]any{}, "")
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}

	// Non-empty key on empty map returns empty map
	got = filterStorageMap(map[string]any{}, "missing")
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

// ---------------------------------------------------------------------------
// filterCookies
// ---------------------------------------------------------------------------

func TestFilterCookies(t *testing.T) {
	t.Parallel()

	cookies := []any{
		map[string]any{"name": "session_id", "value": "abc"},
		map[string]any{"name": "theme", "value": "dark"},
		map[string]any{"name": "lang", "value": "en"},
	}

	tests := []struct {
		name    string
		filter  string
		wantLen int
	}{
		{
			name:    "empty filter returns all",
			filter:  "",
			wantLen: 3,
		},
		{
			name:    "match by name",
			filter:  "theme",
			wantLen: 1,
		},
		{
			name:    "no match returns nil",
			filter:  "nonexistent",
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterCookies(cookies, tc.filter)
			if len(got) != tc.wantLen {
				t.Errorf("len = %d, want %d", len(got), tc.wantLen)
			}
		})
	}
}

func TestFilterCookies_MatchedValuePreserved(t *testing.T) {
	t.Parallel()
	cookies := []any{
		map[string]any{"name": "session_id", "value": "abc123", "domain": ".example.com"},
	}
	got := filterCookies(cookies, "session_id")
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	m, ok := got[0].(map[string]any)
	if !ok {
		t.Fatal("result[0] not a map[string]any")
	}
	if m["value"] != "abc123" {
		t.Errorf("value = %v, want 'abc123'", m["value"])
	}
	if m["domain"] != ".example.com" {
		t.Errorf("domain = %v, want '.example.com'", m["domain"])
	}
}

func TestFilterCookies_MultipleSameName(t *testing.T) {
	t.Parallel()
	cookies := []any{
		map[string]any{"name": "token", "value": "a", "path": "/"},
		map[string]any{"name": "token", "value": "b", "path": "/api"},
		map[string]any{"name": "other", "value": "c"},
	}
	got := filterCookies(cookies, "token")
	if len(got) != 2 {
		t.Errorf("len = %d, want 2 (both cookies named 'token')", len(got))
	}
}

func TestFilterCookies_NonMapEntry(t *testing.T) {
	t.Parallel()
	// Non-map entries should be silently skipped
	cookies := []any{
		"not-a-map",
		map[string]any{"name": "valid", "value": "ok"},
	}
	got := filterCookies(cookies, "valid")
	if len(got) != 1 {
		t.Errorf("len = %d, want 1 (non-map entry should be skipped)", len(got))
	}
}

func TestFilterCookies_EmptySlice(t *testing.T) {
	t.Parallel()
	got := filterCookies([]any{}, "anything")
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestFilterCookies_NilSlice(t *testing.T) {
	t.Parallel()
	// Empty name on nil slice returns nil
	got := filterCookies(nil, "")
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

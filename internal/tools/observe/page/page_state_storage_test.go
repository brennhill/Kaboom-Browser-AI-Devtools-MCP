// page_state_storage_test.go — Tests observe storage argument parsing.
package page

import (
	"encoding/json"
	"testing"
)

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

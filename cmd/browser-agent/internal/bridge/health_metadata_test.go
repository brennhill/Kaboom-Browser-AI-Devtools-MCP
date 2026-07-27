// health_metadata_test.go — Verifies daemon health identity and version compatibility parsing.

package bridge

import "testing"

func TestDecodeHealthMetadataResolvesServiceFieldFallbacks(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "service-name", body: `{"version":"v0.8.8","service-name":"kaboom"}`, want: "kaboom"},
		{name: "service", body: `{"version":"v0.8.8","service":"kaboom-agentic-browser"}`, want: "kaboom-agentic-browser"},
		{name: "name", body: `{"version":"v0.8.8","name":"legacy"}`, want: "legacy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, ok := decodeHealthMetadata([]byte(tt.body))
			if !ok {
				t.Fatal("expected valid health metadata")
			}
			if got := meta.resolvedServiceName(); got != tt.want {
				t.Fatalf("resolved service = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionsMatchNormalizesWhitespaceAndVPrefix(t *testing.T) {
	if !versionsMatch(" v0.8.8 ", "0.8.8") {
		t.Fatal("expected normalized versions to match")
	}
	if versionsMatch("0.8.8", "0.8.9") {
		t.Fatal("different versions must not match")
	}
}

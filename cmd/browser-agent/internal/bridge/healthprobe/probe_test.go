// probe_test.go — Verifies canonical daemon health identity and version matching.

package healthprobe

import "testing"

func TestDecodeUsesCanonicalNameOnly(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "name", body: `{"version":"v0.8.8","name":"kaboom-browser-devtools"}`, want: "kaboom-browser-devtools"},
		{name: "service-name rejected", body: `{"version":"v0.8.8","service-name":"kaboom"}`, want: ""},
		{name: "service rejected", body: `{"version":"v0.8.8","service":"kaboom-agentic-browser"}`, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, service, ok := Evaluate([]byte(tt.body), tt.want, "0.8.8")
			if !ok {
				t.Fatal("expected valid health metadata")
			}
			if got := service; got != tt.want {
				t.Fatalf("resolved service = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionsMatchNormalizesWhitespaceAndVPrefix(t *testing.T) {
	compatible, _, _, ok := Evaluate([]byte(`{"version":" v0.8.8 ","name":"kaboom"}`), "kaboom", "0.8.8")
	if !ok || !compatible {
		t.Fatal("expected normalized versions to match")
	}
	compatible, _, _, ok = Evaluate([]byte(`{"version":"0.8.8","name":"kaboom"}`), "kaboom", "0.8.9")
	if !ok || compatible {
		t.Fatal("different versions must not match")
	}
}

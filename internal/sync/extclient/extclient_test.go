// extclient_test.go — Contracts for extension-facing client identity.
// Docs: docs/features/feature/self-testing/index.md

package extclient

import "testing"

func TestClientIdentityClasses(t *testing.T) {
	t.Parallel()
	for clientID, want := range map[string]struct{ extension, probe, allowed bool }{
		"kaboom-extension":           {true, false, true},
		"kaboom-extension/0.9.0":     {true, false, true},
		"kaboom-extension-offscreen": {true, false, true},
		"kaboom-probe":               {false, true, true},
		"kaboom-probe/0.9.0":         {false, true, true},
		"":                           {false, false, false},
		"curl/8.4.0":                 {false, false, false},
		"kaboom-extension-evil.test": {false, false, false},
		"kaboom-probe-evil.test":     {false, false, false},
	} {
		if got := IsExtension(clientID); got != want.extension {
			t.Errorf("IsExtension(%q) = %v, want %v", clientID, got, want.extension)
		}
		if got := IsProbe(clientID); got != want.probe {
			t.Errorf("IsProbe(%q) = %v, want %v", clientID, got, want.probe)
		}
		if got := Allowed(clientID); got != want.allowed {
			t.Errorf("Allowed(%q) = %v, want %v", clientID, got, want.allowed)
		}
	}
}

// A probe must never be mistaken for the extension: the sync runtime keys session
// ownership off exactly this distinction.
func TestProbeAndExtensionClassesAreDisjoint(t *testing.T) {
	t.Parallel()
	for _, clientID := range []string{Extension, ExtensionOffscreen, Extension + "/1.2.3", Probe, Probe + "/1.2.3"} {
		if IsExtension(clientID) && IsProbe(clientID) {
			t.Fatalf("%q classified as both extension and probe", clientID)
		}
	}
}

// runtime_test.go — Pins isolated application lifecycle state.
package appruntime

import (
	"testing"
	"time"
)

func TestUpdateWarningClaimsArePerRuntime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	first := New("0.9.0")
	second := New("0.9.0")

	if !first.ClaimUpdateWarning(now, 24*time.Hour) {
		t.Fatal("first runtime should claim its initial warning")
	}
	if first.ClaimUpdateWarning(now.Add(time.Hour), 24*time.Hour) {
		t.Fatal("same runtime should enforce its cooldown")
	}
	if !second.ClaimUpdateWarning(now.Add(time.Hour), 24*time.Hour) {
		t.Fatal("one runtime must not suppress another runtime")
	}
}

func TestRuntimeOwnsUpgradeProvider(t *testing.T) {
	t.Parallel()
	runtime := New("0.9.0")
	provider := fixedUpgrade{version: "0.9.1"}
	runtime.SetUpgrade(provider)

	_, got, _ := runtime.Upgrade().UpgradeInfo()
	if got != "0.9.1" {
		t.Fatalf("upgrade version = %q, want 0.9.1", got)
	}
}

func TestBridgeRunnerIsPerRuntime(t *testing.T) {
	t.Parallel()
	first := New("0.9.0")
	second := New("0.9.0")
	first.SetBridgeRunner(fakeBridge{id: "first"})
	second.SetBridgeRunner(fakeBridge{id: "second"})

	if first.BridgeRunner().LaunchFingerprint()["id"] == second.BridgeRunner().LaunchFingerprint()["id"] {
		t.Fatal("bridge runner leaked between application runtimes")
	}
}

type fixedUpgrade struct{ version string }

func (f fixedUpgrade) UpgradeInfo() (bool, string, time.Time) { return true, f.version, time.Time{} }

type fakeBridge struct{ id string }

func (f fakeBridge) IsServerRunning(int) bool              { return false }
func (f fakeBridge) WaitForServer(int, time.Duration) bool { return false }
func (f fakeBridge) EnsureIOIsolation(string) error        { return nil }
func (f fakeBridge) LaunchFingerprint() map[string]any     { return map[string]any{"id": f.id} }
func (f fakeBridge) RunMode(int, string, int)              {}

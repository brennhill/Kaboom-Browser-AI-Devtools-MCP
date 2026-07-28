package nativeinstall

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestFileConfigTargets_VSCodeUsesServersKey is the regression test for the
// VS Code MCP config key: VS Code reads servers from a top-level "servers"
// key in mcp.json, not "mcpServers".
func TestFileConfigTargets_VSCodeUsesServersKey(t *testing.T) {
	configs := fileConfigTargets("/Users/tester")

	foundVSCode := false
	for _, cfg := range configs {
		if cfg.name != "VS Code" {
			continue
		}
		foundVSCode = true
		if cfg.key != "servers" {
			t.Errorf("VS Code config key = %q, want %q", cfg.key, "servers")
		}
		if cfg.isCustom {
			t.Error("VS Code config should use the standard {command,args} entry shape")
		}
	}
	if !foundVSCode {
		t.Fatal("fileConfigTargets missing a VS Code entry for this OS")
	}

	// Other standard clients keep the conventional mcpServers key.
	for _, name := range []string{"Cursor", "Windsurf", "Gemini CLI"} {
		for _, cfg := range configs {
			if cfg.name == name && cfg.key != "mcpServers" {
				t.Errorf("%s config key = %q, want %q", name, cfg.key, "mcpServers")
			}
		}
	}
}

// TestEnvWithout_RemovesVariable verifies the variable is absent entirely,
// not merely present-but-empty (which nesting guards still detect).
func TestEnvWithout_RemovesVariable(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")

	env := envWithout("CLAUDECODE")
	for _, kv := range env {
		if strings.HasPrefix(kv, "CLAUDECODE=") {
			t.Fatalf("envWithout left %q in the environment", kv)
		}
	}

	// Unrelated variables must survive.
	t.Setenv("KABOOM_ENVWITHOUT_PROBE", "keep")
	env = envWithout("CLAUDECODE")
	found := false
	for _, kv := range env {
		if kv == "KABOOM_ENVWITHOUT_PROBE=keep" {
			found = true
		}
	}
	if !found {
		t.Fatal("envWithout dropped an unrelated environment variable")
	}
}

func TestManualExtensionSetupChecklist_IncludesRequiredSteps(t *testing.T) {
	extPath := `/Users/tester/KaboomAgenticDevtoolExtension`
	checklist := manualExtensionSetupChecklist(extPath)
	joined := strings.Join(checklist, "\n")

	required := []string{
		"MANUAL STEP REQUIRED",
		"cannot click browser UI controls",
		"chrome://extensions (or brave://extensions)",
		"Enable Developer mode",
		"Load unpacked",
		extPath,
		"Pin Kaboom",
		"Track This Tab",
	}

	for _, want := range required {
		if !strings.Contains(joined, want) {
			t.Fatalf("checklist missing %q; got:\n%s", want, joined)
		}
	}
}

func TestExtensionInstallDir_DefaultVisiblePath(t *testing.T) {
	t.Setenv("KABOOM_EXTENSION_DIR", "")
	home := "/Users/tester"
	want := filepath.Join(home, "KaboomAgenticDevtoolExtension")

	if got := extensionInstallDir(home); got != want {
		t.Fatalf("extensionInstallDir(%q) = %q, want %q", home, got, want)
	}
}

func TestExtensionInstallDir_EnvOverride(t *testing.T) {
	override := "/tmp/custom-kaboom-ext"
	t.Setenv("KABOOM_EXTENSION_DIR", override)
	home := "/Users/tester"

	if got := extensionInstallDir(home); got != override {
		t.Fatalf("extensionInstallDir(%q) = %q, want env override %q", home, got, override)
	}
}

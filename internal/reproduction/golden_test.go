// Purpose: Golden-file snapshot tests for reproduction script generation output stability.
// Docs: docs/features/feature/reproduction-scripts/index.md

// golden_test.go — Golden file validation for Playwright reproduction scripts.
package reproduction

import (
	"bytes"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"os"
	"regexp"
	"testing"
)

var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

func TestGoldenReproductionPlaywright(t *testing.T) {
	// One pin for the whole recording: the golden then asserts the header states what the
	// artifact depends on, and the per-action fallback block below it.
	pin := &types.WireEnvironmentPin{
		Clock:      &types.WireClockPin{EpochMs: 1705312800000, TimezoneID: "UTC", VirtualTimePolicy: "pause"},
		Viewport:   &types.WireViewportPin{Width: 1280, Height: 720, DeviceScaleFactor: 2},
		RandomSeed: "kaboom-golden",
		Unpinned:   []string{"network responses"},
	}
	actions := []types.EnhancedAction{
		{
			Type:        "navigate",
			Timestamp:   1705312800000,
			ToURL:       "https://app.example.com/login",
			Environment: pin,
		},
		{
			Type:      "click",
			Timestamp: 1705312801000,
			Selectors: map[string]any{"testId": "email-input"},
			URL:       "https://app.example.com/login",
			// All three locators, so the golden guards the emitted fallback block itself:
			// a change that silently drops the AX or coordinate line fails here.
			AX: &types.WireAXLocator{Ref: "ax_88", Role: "textbox", Name: "Email"},
			Viewport: &types.WireViewportLocator{
				X: 400, Y: 220, Width: 240, Height: 40,
				FrameURL:         "https://app.example.com/login",
				ViewportWidth:    1280,
				ViewportHeight:   720,
				DevicePixelRatio: 2,
			},
			Environment: pin,
		},
		{
			Type:        "input",
			Timestamp:   1705312802000,
			Selectors:   map[string]any{"testId": "email-input"},
			Value:       "user@test.com",
			URL:         "https://app.example.com/login",
			AX:          &types.WireAXLocator{Ref: "ax_88", Role: "textbox", Name: "Email"},
			Environment: pin,
		},
		{
			Type:        "keypress",
			Timestamp:   1705312803000,
			Key:         "Enter",
			URL:         "https://app.example.com/login",
			Environment: pin,
		},
	}

	opts := Params{
		BaseURL:      "https://app.example.com",
		ErrorMessage: "Login button not responding",
	}

	script := GeneratePlaywrightScript(actions, opts)

	// Normalize: remove any dynamic timestamps in comments
	re := regexp.MustCompile(`// Generated at .*`)
	normalizedScript := re.ReplaceAll([]byte(script), []byte("// Generated at TIMESTAMP"))

	goldenPath := "testdata/reproduction-playwright.golden.txt"

	if updateGolden {
		err := os.WriteFile(goldenPath, normalizedScript, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		t.Logf("Updated golden file (%d bytes)", len(normalizedScript))
	} else {
		goldenData, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("Failed to read golden file (run with UPDATE_GOLDEN=1 first): %v", err)
		}

		if !bytes.Equal(normalizedScript, goldenData) {
			t.Errorf("Golden file mismatch for %s", goldenPath)
			t.Errorf("Expected:\n%s", string(goldenData))
			t.Errorf("Got:\n%s", string(normalizedScript))
			t.Fatalf("Run with UPDATE_GOLDEN=1 to update golden files")
		}
		t.Logf("Reproduction golden file validation passed (%d bytes)", len(normalizedScript))
	}
}

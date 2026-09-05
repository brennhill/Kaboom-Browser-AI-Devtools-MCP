// Purpose: Covers three-locator emission and environment-pin reporting in reproduction artifacts.
// Why: A step described only by a selector dies on the first re-render.
// Docs: docs/features/feature/session-to-test/index.md
package reproduction

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func clickWithAllThreeLocators() types.EnhancedAction {
	return types.EnhancedAction{
		Type:      "click",
		Timestamp: 1705312801000,
		URL:       "https://app.example.com/checkout",
		Selectors: map[string]any{"testId": "place-order"},
		AX: &types.WireAXLocator{
			Ref:  "ax_412",
			Role: "button",
			Name: "Place order",
		},
		Viewport: &types.WireViewportLocator{
			X: 120, Y: 240, Width: 96, Height: 32,
			FrameURL:         "https://app.example.com/checkout",
			ViewportWidth:    1280,
			ViewportHeight:   720,
			DevicePixelRatio: 2,
		},
	}
}

func TestBuildLocatorsReturnsAllThreeInFallbackOrder(t *testing.T) {
	t.Parallel()

	locators := buildLocators(clickWithAllThreeLocators())
	if len(locators) != 3 {
		t.Fatalf("buildLocators returned %d locators, want 3: %#v", len(locators), locators)
	}
	want := []string{strategySelector, strategyAX, strategyCoordinate}
	for i, strategy := range want {
		if locators[i].Strategy != strategy {
			t.Errorf("locator %d strategy = %q, want %q", i, locators[i].Strategy, strategy)
		}
	}
	if fallbackOrder()[0] != strategySelector || fallbackOrder()[2] != strategyCoordinate {
		t.Errorf("fallbackOrder = %v, want selector first and coordinate last", fallbackOrder())
	}
}

func TestBuildLocatorsOmitsStrategiesTheRecordingNeverCaptured(t *testing.T) {
	t.Parallel()

	action := types.EnhancedAction{Type: "click", Selectors: map[string]any{"id": "submit"}}
	locators := buildLocators(action)
	if len(locators) != 1 || locators[0].Strategy != strategySelector {
		t.Fatalf("buildLocators = %#v, want selector only", locators)
	}
}

func TestBuildLocatorsRecoversTargetWhenSelectorIsMissing(t *testing.T) {
	t.Parallel()

	// The exact failure this bead exists for: the DOM re-rendered, the selector no longer
	// describes anything, but the accessible name is unchanged. A step with no locator at
	// all cannot be replayed or repaired.
	action := clickWithAllThreeLocators()
	action.Selectors = nil

	locators := buildLocators(action)
	if len(locators) != 2 {
		t.Fatalf("buildLocators returned %d locators, want ax + coordinate: %#v", len(locators), locators)
	}
	if locators[0].Strategy != strategyAX {
		t.Fatalf("first fallback = %q, want %q", locators[0].Strategy, strategyAX)
	}
	if !strings.Contains(locators[0].Playwright, "Place order") {
		t.Errorf("ax locator %q does not carry the accessible name", locators[0].Playwright)
	}
}

func TestAXLocatorCarriesRefRoleAndName(t *testing.T) {
	t.Parallel()

	locators := buildLocators(clickWithAllThreeLocators())
	ax := locators[1]
	for _, want := range []string{"ax_412", "button", "Place order"} {
		if !strings.Contains(ax.Human, want) {
			t.Errorf("ax human description %q missing %q", ax.Human, want)
		}
	}
}

func TestCoordinateLocatorCarriesViewportAndFrame(t *testing.T) {
	t.Parallel()

	locators := buildLocators(clickWithAllThreeLocators())
	coordinate := locators[2]
	// A bare x/y is unusable: replayed at a different window size or device scale it lands
	// somewhere else entirely, and in the wrong frame it lands on the wrong document.
	for _, want := range []string{"120", "240", "1280x720", "https://app.example.com/checkout"} {
		if !strings.Contains(coordinate.Human, want) {
			t.Errorf("coordinate description %q missing %q", coordinate.Human, want)
		}
	}
}

func TestPlaywrightScriptEmitsFallbacksForEveryTargetedStep(t *testing.T) {
	t.Parallel()

	script := GeneratePlaywrightScript([]types.EnhancedAction{clickWithAllThreeLocators()}, Params{})
	if !strings.Contains(script, "await page.getByTestId('place-order').click();") {
		t.Fatalf("primary step missing from script:\n%s", script)
	}
	for _, want := range []string{"selector -> ax -> coordinate", "ax_412", "page.mouse.click(120, 240)"} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q:\n%s", want, script)
		}
	}
}

func TestKaboomScriptEmitsFallbacksForEveryTargetedStep(t *testing.T) {
	t.Parallel()

	script := GenerateKaboomScript([]types.EnhancedAction{clickWithAllThreeLocators()}, Params{})
	for _, want := range []string{"fallback ax:", "Place order", "fallback coordinate:", "(120, 240)"} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q:\n%s", want, script)
		}
	}
}

func pinnedEnvironment() *types.WireEnvironmentPin {
	return &types.WireEnvironmentPin{
		Clock: &types.WireClockPin{
			EpochMs:           1788480000000,
			TimezoneID:        "America/New_York",
			VirtualTimePolicy: "pause",
		},
		Geolocation: &types.WireGeoPin{Latitude: 37.7749, Longitude: -122.4194, AccuracyM: 10},
		Viewport:    &types.WireViewportPin{Width: 1280, Height: 720, DeviceScaleFactor: 2},
		RandomSeed:  "kaboom-7",
		Unpinned:    []string{"network responses"},
	}
}

func TestEnvironmentPinIsReportedInBothBackends(t *testing.T) {
	t.Parallel()

	action := clickWithAllThreeLocators()
	action.Environment = pinnedEnvironment()
	actions := []types.EnhancedAction{action}

	// A test that silently depends on a pinned clock is worse than one that does not pin:
	// it passes on the recording machine and fails everywhere else with no stated cause.
	for name, script := range map[string]string{
		"playwright": GeneratePlaywrightScript(actions, Params{}),
		"kaboom":     GenerateKaboomScript(actions, Params{}),
	} {
		for _, want := range []string{"1788480000000", "America/New_York", "37.7749", "1280x720", "kaboom-7", "network responses"} {
			if !strings.Contains(script, want) {
				t.Errorf("%s script does not report %q:\n%s", name, want, script)
			}
		}
	}
}

func TestUnpinnedEnvironmentIsStatedRatherThanOmitted(t *testing.T) {
	t.Parallel()

	actions := []types.EnhancedAction{clickWithAllThreeLocators()}
	for name, script := range map[string]string{
		"playwright": GeneratePlaywrightScript(actions, Params{}),
		"kaboom":     GenerateKaboomScript(actions, Params{}),
	} {
		if !strings.Contains(script, "Environment not pinned") {
			t.Errorf("%s script omits the unpinned-environment statement:\n%s", name, script)
		}
	}
}

func TestPinChangeMidSessionIsReportedRatherThanCollapsed(t *testing.T) {
	t.Parallel()

	first := clickWithAllThreeLocators()
	first.Environment = pinnedEnvironment()
	second := clickWithAllThreeLocators()
	second.Timestamp = first.Timestamp + 1000
	// A navigation clears CDP overrides. Reporting only the first pin would claim the whole
	// recording ran under a clock that lapsed halfway through.
	second.Environment = nil

	script := GenerateKaboomScript([]types.EnhancedAction{first, second}, Params{})
	if !strings.Contains(script, "changed during the recording") {
		t.Errorf("kaboom script does not report the mid-session pin change:\n%s", script)
	}
}

func TestBuildResultReportsLocatorCoverageAndPinState(t *testing.T) {
	t.Parallel()

	action := clickWithAllThreeLocators()
	action.Environment = pinnedEnvironment()
	actions := []types.EnhancedAction{action}

	result := BuildResult("script", Params{}, actions, actions)
	if !result.Metadata.EnvironmentPinned {
		t.Error("Metadata.EnvironmentPinned = false, want true")
	}
	if len(result.Metadata.FallbackOrder) != 3 {
		t.Errorf("Metadata.FallbackOrder = %v, want three strategies", result.Metadata.FallbackOrder)
	}
	if result.Metadata.LocatorCoverage[strategyAX] != 1 || result.Metadata.LocatorCoverage[strategyCoordinate] != 1 {
		t.Errorf("Metadata.LocatorCoverage = %v, want one action per strategy", result.Metadata.LocatorCoverage)
	}
}

// Purpose: Builds the three independent locators recorded per step and reports the environment
//          a session pinned, for both reproduction backends.
// Why: A step described only by a CSS selector dies on the first re-render, and a test that
//      silently depends on a pinned clock passes only on the machine that recorded it.
// Docs: docs/features/feature/session-to-test/index.md

package reproduction

import (
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// The three independent ways a recorded target can be re-found.
const (
	strategySelector   = "selector"
	strategyAX         = "ax"
	strategyCoordinate = "coordinate"
)

// fallbackOrder is the order a replay must try the emitted locators in.
//
// selector first: it resolves inside the page with no debugger attach, it is the only
// strategy Playwright expresses as a first-class stepLocator, and it is the most precise
// answer while the markup is unchanged.
//
// ax second: role plus accessible name survives DOM restructuring, class churn and
// wrapper elements, because it describes what the control MEANS rather than where it
// sits. It costs an accessibility-tree snapshot over CDP, so it is not tried first.
//
// coordinate last: a point always resolves to something, so it can never report "not
// found" — it silently hits whatever now occupies that point. That makes it the correct
// answer only once both semantic strategies have failed, and only under the viewport it
// was measured in, which is why the emitted coordinate carries that viewport and frame.
// A function, not a package-level slice: an exported or shared slice is mutable state any
// caller could reorder, and the order is the contract.
func fallbackOrder() []string { return []string{strategySelector, strategyAX, strategyCoordinate} }

// fallbackOrderLabel renders fallbackOrder() for artifact headers.
const fallbackOrderLabel = "selector -> ax -> coordinate"

// stepLocator is one independent way to re-find a recorded target.
type stepLocator struct {
	// Strategy is one of the Strategy* constants.
	Strategy string
	// Playwright is the stepLocator or call expression, empty when the strategy has none.
	Playwright string
	// Human is the one-line description used by the kaboom-native backend.
	Human string
}

// buildLocators returns every stepLocator the recording captured for an action, in fallbackOrder.
// Strategies the recording never captured are omitted rather than emitted empty: a stepLocator
// nothing can act on is worse than one fewer answer.
func buildLocators(action types.EnhancedAction) []stepLocator {
	var locators []stepLocator
	if selector := PlaywrightLocator(action.Selectors); selector != "" {
		locators = append(locators, stepLocator{
			Strategy:   strategySelector,
			Playwright: selector,
			Human:      DescribeElement(action),
		})
	}
	if ax := axLocator(action.AX); ax != nil {
		locators = append(locators, *ax)
	}
	if coordinate := coordinateLocator(action.Viewport); coordinate != nil {
		locators = append(locators, *coordinate)
	}
	return locators
}

func axLocator(ax *types.WireAXLocator) *stepLocator {
	if ax == nil || (ax.Role == "" && ax.Name == "") {
		return nil
	}
	human := fmt.Sprintf("role=%s name=%q", ax.Role, ax.Name)
	if ax.Ref != "" {
		human += " (ref " + ax.Ref + ")"
	}
	return &stepLocator{Strategy: strategyAX, Playwright: pwRoleLocator(ax.Role, ax.Name), Human: human}
}

func coordinateLocator(viewport *types.WireViewportLocator) *stepLocator {
	if viewport == nil {
		return nil
	}
	human := fmt.Sprintf("(%d, %d) in %s", viewport.X, viewport.Y, describeViewportFrame(viewport))
	return &stepLocator{
		Strategy:   strategyCoordinate,
		Playwright: fmt.Sprintf("page.mouse.click(%d, %d)", viewport.X, viewport.Y),
		Human:      human,
	}
}

func describeViewportFrame(viewport *types.WireViewportLocator) string {
	var parts []string
	if viewport.ViewportWidth > 0 && viewport.ViewportHeight > 0 {
		parts = append(parts, fmt.Sprintf("%dx%d", viewport.ViewportWidth, viewport.ViewportHeight))
	}
	if viewport.DevicePixelRatio > 0 {
		parts = append(parts, fmt.Sprintf("@%gx", viewport.DevicePixelRatio))
	}
	if viewport.FrameURL != "" {
		parts = append(parts, "frame "+viewport.FrameURL)
	}
	if len(parts) == 0 {
		return "an unrecorded viewport"
	}
	return strings.Join(parts, " ")
}

// renderLocatorHuman describes a stepLocator in prose, for the kaboom-native backend.
func renderLocatorHuman(loc stepLocator) string { return loc.Human }

// renderLocatorCode describes a stepLocator as the call a repair would make, with the prose
// kept alongside: the expression alone loses the AX ref and the viewport the point was
// measured in, which is exactly what makes a bare coordinate unreplayable.
func renderLocatorCode(loc stepLocator) string {
	if loc.Playwright == "" {
		return loc.Human
	}
	return loc.Playwright + "  [" + loc.Human + "]"
}

// writeFallbackLocators writes every stepLocator after the primary one, each on its own line.
// The primary is already emitted as the executable step, so repeating it would only pad
// the artifact. Nothing is written when the recording captured a single strategy.
func writeFallbackLocators(
	b *strings.Builder,
	locators []stepLocator,
	indent, header string,
	render func(stepLocator) string,
) {
	if len(locators) < 2 {
		return
	}
	if header != "" {
		fmt.Fprintf(b, "%s%s\n", indent, header)
	}
	for _, loc := range locators[1:] {
		fmt.Fprintf(b, "%sfallback %s: %s\n", indent, loc.Strategy, render(loc))
	}
}

// locatorCoverage counts how many actions carried each strategy, so a caller can tell a
// session recorded with all three locators from one that only ever had selectors.
func locatorCoverage(actions []types.EnhancedAction) map[string]int {
	coverage := map[string]int{}
	for _, action := range actions {
		for _, loc := range buildLocators(action) {
			coverage[loc.Strategy]++
		}
	}
	return coverage
}

// ============================================
// Environment pin reporting
// ============================================

// unpinnedNotice is the statement emitted when the session pinned nothing. Stating it is
// the point: a reader cannot otherwise tell "not pinned" from "pinning was not reported".
const unpinnedNotice = "Environment not pinned: this run inherits the machine clock, timezone, locale, location and viewport."

// pinnedNotice heads the pin report. A test that depends on a pinned clock without saying
// so passes on the recording machine and fails everywhere else with no stated cause, which
// is worse than a test that pins nothing.
const pinnedNotice = "Environment pinned by the recording session. This test depends on it:"

// pinChangedNotice is emitted when actions in one recording carry different pins. A
// navigation clears CDP overrides, so a single header would claim a pin that lapsed.
const pinChangedNotice = "Environment pin changed during the recording; the lines above describe the first pinned action only."

// environmentPinLines renders one line per pinned knob, or the unpinned notice.
func environmentPinLines(pin *types.WireEnvironmentPin) []string {
	if pin == nil {
		return []string{unpinnedNotice}
	}
	var lines []string
	if clock := describeClockPin(pin.Clock); clock != "" {
		lines = append(lines, clock)
	}
	if pin.Geolocation != nil {
		lines = append(lines, fmt.Sprintf("geolocation: %g, %g (accuracy %gm)",
			pin.Geolocation.Latitude, pin.Geolocation.Longitude, pin.Geolocation.AccuracyM))
	}
	if pin.Viewport != nil {
		lines = append(lines, describeViewportPin(pin.Viewport))
	}
	if pin.RandomSeed != "" {
		lines = append(lines, "random seed: "+pin.RandomSeed+" (Math.random and crypto.getRandomValues)")
	}
	if len(pin.Unpinned) > 0 {
		// Named explicitly: the knobs a session asked for and did not get are exactly the
		// ones a replay will diverge on, and a silent omission reads as "pinned".
		lines = append(lines, "NOT pinned: "+strings.Join(pin.Unpinned, ", "))
	}
	if len(lines) == 0 {
		return []string{unpinnedNotice}
	}
	return append([]string{pinnedNotice}, lines...)
}

func describeClockPin(clock *types.WireClockPin) string {
	if clock == nil {
		return ""
	}
	var parts []string
	if clock.EpochMs != 0 {
		parts = append(parts, fmt.Sprintf("epoch_ms %d", clock.EpochMs))
	}
	if clock.TimezoneID != "" {
		parts = append(parts, "timezone "+clock.TimezoneID)
	}
	if clock.VirtualTimePolicy != "" {
		parts = append(parts, "virtual time policy "+clock.VirtualTimePolicy)
	}
	if len(parts) == 0 {
		return ""
	}
	return "clock: " + strings.Join(parts, ", ")
}

func describeViewportPin(viewport *types.WireViewportPin) string {
	line := fmt.Sprintf("viewport: %dx%d", viewport.Width, viewport.Height)
	if viewport.DeviceScaleFactor > 0 {
		line += fmt.Sprintf(" @%gx", viewport.DeviceScaleFactor)
	}
	if viewport.Mobile {
		line += " (mobile)"
	}
	return line
}

// sessionPin returns the first pin recorded in a session and whether the pin changed
// part-way through. Both are needed: the pin describes what the artifact depends on, and
// the change says the description does not hold for every step.
func sessionPin(actions []types.EnhancedAction) (pin *types.WireEnvironmentPin, changed bool) {
	seen := false
	var first *types.WireEnvironmentPin
	for i := range actions {
		current := actions[i].Environment
		if !seen {
			first, seen = current, true
			continue
		}
		if !samePin(first, current) {
			changed = true
		}
	}
	return first, changed
}

func samePin(a, b *types.WireEnvironmentPin) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return strings.Join(environmentPinLines(a), "|") == strings.Join(environmentPinLines(b), "|")
}

// writeEnvironmentPin writes the pin report with the given comment prefix, so both
// backends state the same facts in their own comment syntax.
func writeEnvironmentPin(b *strings.Builder, actions []types.EnhancedAction, prefix string) {
	pin, changed := sessionPin(actions)
	for _, line := range environmentPinLines(pin) {
		b.WriteString(prefix + line + "\n")
	}
	if changed {
		b.WriteString(prefix + pinChangedNotice + "\n")
	}
	b.WriteString(prefix + "Locator fallback order: " + fallbackOrderLabel + "\n")
}

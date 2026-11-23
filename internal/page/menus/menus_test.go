// menus_test.go — TDD tests for 3-layer menu heuristic.
// Docs: docs/features/feature/auto-fix/index.md

package menus

import "testing"

var defaultCfg = DefaultConfig()

// =============================================================================
// Layer 1: Semantic grouping
// =============================================================================

func TestDiscover_SemanticNav(t *testing.T) {
	t.Parallel()
	elements := []RawElement{
		{Text: "Home", Href: "/", Tag: "a", ParentTag: "nav", BBox: BBox{X: 100, Y: 10, Width: 60, Height: 30}, Visible: true, Index: 0},
		{Text: "About", Href: "/about", Tag: "a", ParentTag: "nav", BBox: BBox{X: 170, Y: 10, Width: 60, Height: 30}, Visible: true, Index: 1},
		{Text: "Contact", Href: "/contact", Tag: "a", ParentTag: "nav", BBox: BBox{X: 240, Y: 10, Width: 70, Height: 30}, Visible: true, Index: 2},
	}

	result := Discover(elements, defaultCfg)

	if len(result.Main) == 0 {
		t.Fatal("expected main menu group from <nav> parent")
	}
	if len(result.Main[0].Items) != 3 {
		t.Errorf("expected 3 items in main, got %d", len(result.Main[0].Items))
	}
	if result.Main[0].Source != "semantic" {
		t.Errorf("source = %q, want semantic", result.Main[0].Source)
	}
}

func TestDiscover_SemanticFooter(t *testing.T) {
	t.Parallel()
	elements := []RawElement{
		{Text: "Privacy", Href: "/privacy", Tag: "a", ParentTag: "footer", BBox: BBox{X: 100, Y: 850, Width: 60, Height: 20}, Visible: true, Index: 0},
		{Text: "Terms", Href: "/terms", Tag: "a", ParentTag: "footer", BBox: BBox{X: 170, Y: 850, Width: 50, Height: 20}, Visible: true, Index: 1},
	}

	result := Discover(elements, defaultCfg)

	if len(result.Footer) == 0 {
		t.Fatal("expected footer menu group from <footer> parent")
	}
	if len(result.Footer[0].Items) != 2 {
		t.Errorf("expected 2 items in footer, got %d", len(result.Footer[0].Items))
	}
}

func TestDiscover_SemanticAside(t *testing.T) {
	t.Parallel()
	elements := []RawElement{
		{Text: "Dashboard", Href: "/dash", Tag: "a", ParentTag: "aside", BBox: BBox{X: 10, Y: 100, Width: 120, Height: 30}, Visible: true, Index: 0},
		{Text: "Settings", Href: "/settings", Tag: "a", ParentTag: "aside", BBox: BBox{X: 10, Y: 140, Width: 120, Height: 30}, Visible: true, Index: 1},
	}

	result := Discover(elements, defaultCfg)

	if len(result.Sidebar) == 0 {
		t.Fatal("expected sidebar menu group from <aside> parent")
	}
}

func TestDiscover_SemanticRole(t *testing.T) {
	t.Parallel()
	elements := []RawElement{
		{Text: "Home", Href: "/", Tag: "div", ParentRole: "navigation", BBox: BBox{X: 100, Y: 10, Width: 60, Height: 30}, Visible: true, Index: 0},
		{Text: "About", Href: "/about", Tag: "div", ParentRole: "navigation", BBox: BBox{X: 170, Y: 10, Width: 60, Height: 30}, Visible: true, Index: 1},
	}

	result := Discover(elements, defaultCfg)

	if len(result.Main) == 0 {
		t.Fatal("expected main menu group from role=navigation parent")
	}
}

// =============================================================================
// Layer 2: Axis alignment
// =============================================================================

func TestDiscover_HorizontalAxisAlignment(t *testing.T) {
	t.Parallel()
	// Three buttons on the same Y axis, no semantic parents — div soup
	elements := []RawElement{
		{Text: "Tab 1", Tag: "button", BBox: BBox{X: 100, Y: 50, Width: 80, Height: 30}, Visible: true, Index: 0},
		{Text: "Tab 2", Tag: "button", BBox: BBox{X: 190, Y: 50, Width: 80, Height: 30}, Visible: true, Index: 1},
		{Text: "Tab 3", Tag: "button", BBox: BBox{X: 280, Y: 50, Width: 80, Height: 30}, Visible: true, Index: 2},
	}

	result := Discover(elements, defaultCfg)

	totalMenuItems := countAllMenuItems(result)
	if totalMenuItems != 3 {
		t.Errorf("expected 3 items in a menu group, got %d total menu items", totalMenuItems)
	}
	if len(result.Ungrouped) != 0 {
		t.Errorf("expected 0 ungrouped, got %d", len(result.Ungrouped))
	}
}

func TestDiscover_VerticalAxisAlignment(t *testing.T) {
	t.Parallel()
	// Sidebar-style: same X, different Y
	elements := []RawElement{
		{Text: "Nav 1", Tag: "a", Href: "/1", BBox: BBox{X: 20, Y: 100, Width: 100, Height: 30}, Visible: true, Index: 0},
		{Text: "Nav 2", Tag: "a", Href: "/2", BBox: BBox{X: 20, Y: 140, Width: 100, Height: 30}, Visible: true, Index: 1},
		{Text: "Nav 3", Tag: "a", Href: "/3", BBox: BBox{X: 20, Y: 180, Width: 100, Height: 30}, Visible: true, Index: 2},
	}

	result := Discover(elements, defaultCfg)

	totalMenuItems := countAllMenuItems(result)
	if totalMenuItems != 3 {
		t.Errorf("expected 3 items in a vertical menu, got %d", totalMenuItems)
	}
}

func TestDiscover_SlightMisalignment(t *testing.T) {
	t.Parallel()
	// Y values differ by < AxisAlignmentThreshold — still a menu
	elements := []RawElement{
		{Text: "A", Tag: "button", BBox: BBox{X: 100, Y: 50, Width: 60, Height: 30}, Visible: true, Index: 0},
		{Text: "B", Tag: "button", BBox: BBox{X: 170, Y: 53, Width: 60, Height: 30}, Visible: true, Index: 1},
		{Text: "C", Tag: "button", BBox: BBox{X: 240, Y: 48, Width: 60, Height: 30}, Visible: true, Index: 2},
	}

	result := Discover(elements, defaultCfg)

	totalMenuItems := countAllMenuItems(result)
	if totalMenuItems != 3 {
		t.Errorf("expected 3 items despite slight Y misalignment, got %d", totalMenuItems)
	}
}

// =============================================================================
// Layer 3: Border proximity
// =============================================================================

func TestDiscover_ProximityRejectsDistantElements(t *testing.T) {
	t.Parallel()
	// Same Y axis but huge gap between them — NOT a menu
	elements := []RawElement{
		{Text: "Left", Tag: "button", BBox: BBox{X: 10, Y: 50, Width: 60, Height: 30}, Visible: true, Index: 0},
		{Text: "Right", Tag: "button", BBox: BBox{X: 800, Y: 50, Width: 60, Height: 30}, Visible: true, Index: 1},
	}

	result := Discover(elements, defaultCfg)

	if len(result.Ungrouped) != 2 {
		t.Errorf("expected 2 ungrouped (too far apart), got %d ungrouped, %d in menus", len(result.Ungrouped), countAllMenuItems(result))
	}
}

func TestDiscover_ProximityConfirmsCloseElements(t *testing.T) {
	t.Parallel()
	// Same Y, small gap — confirmed menu
	elements := []RawElement{
		{Text: "A", Tag: "button", BBox: BBox{X: 100, Y: 50, Width: 60, Height: 30}, Visible: true, Index: 0},
		{Text: "B", Tag: "button", BBox: BBox{X: 170, Y: 50, Width: 60, Height: 30}, Visible: true, Index: 1},
		{Text: "C", Tag: "button", BBox: BBox{X: 240, Y: 50, Width: 60, Height: 30}, Visible: true, Index: 2},
	}

	result := Discover(elements, defaultCfg)

	totalMenuItems := countAllMenuItems(result)
	if totalMenuItems != 3 {
		t.Errorf("expected 3 items (close proximity confirms menu), got %d", totalMenuItems)
	}
}

// =============================================================================
// Classification
// =============================================================================

func TestDiscover_TopPositionClassifiedAsMain(t *testing.T) {
	t.Parallel()
	// No semantic hints, but top of viewport → main
	elements := []RawElement{
		{Text: "Home", Tag: "a", Href: "/", BBox: BBox{X: 200, Y: 15, Width: 60, Height: 30}, Visible: true, Index: 0},
		{Text: "Products", Tag: "a", Href: "/products", BBox: BBox{X: 270, Y: 15, Width: 80, Height: 30}, Visible: true, Index: 1},
		{Text: "Pricing", Tag: "a", Href: "/pricing", BBox: BBox{X: 360, Y: 15, Width: 70, Height: 30}, Visible: true, Index: 2},
	}

	result := Discover(elements, defaultCfg)

	if len(result.Main) == 0 {
		t.Fatal("expected top-of-viewport cluster to be classified as main")
	}
}

func TestDiscover_LeftPositionClassifiedAsSidebar(t *testing.T) {
	t.Parallel()
	// Left edge, vertical stack → sidebar
	cfg := defaultCfg
	cfg.ViewportWidth = 1440
	elements := []RawElement{
		{Text: "Dashboard", Tag: "a", Href: "/dash", BBox: BBox{X: 10, Y: 100, Width: 150, Height: 30}, Visible: true, Index: 0},
		{Text: "Settings", Tag: "a", Href: "/set", BBox: BBox{X: 10, Y: 140, Width: 150, Height: 30}, Visible: true, Index: 1},
		{Text: "Profile", Tag: "a", Href: "/prof", BBox: BBox{X: 10, Y: 180, Width: 150, Height: 30}, Visible: true, Index: 2},
	}

	result := Discover(elements, cfg)

	if len(result.Sidebar) == 0 {
		t.Fatal("expected left-edge vertical cluster to be classified as sidebar")
	}
}

func TestDiscover_BottomPositionClassifiedAsFooter(t *testing.T) {
	t.Parallel()
	cfg := defaultCfg
	cfg.ViewportHeight = 900
	elements := []RawElement{
		{Text: "Privacy", Tag: "a", Href: "/privacy", BBox: BBox{X: 200, Y: 870, Width: 60, Height: 20}, Visible: true, Index: 0},
		{Text: "Terms", Tag: "a", Href: "/terms", BBox: BBox{X: 270, Y: 870, Width: 50, Height: 20}, Visible: true, Index: 1},
	}

	result := Discover(elements, cfg)

	if len(result.Footer) == 0 {
		t.Fatal("expected bottom-of-viewport cluster to be classified as footer")
	}
}

// =============================================================================
// Edge cases
// =============================================================================

func TestDiscover_EmptyInput(t *testing.T) {
	t.Parallel()
	result := Discover(nil, defaultCfg)

	if result.Main == nil || result.Sidebar == nil || result.Footer == nil || result.Other == nil || result.Ungrouped == nil {
		t.Error("all result fields should be non-nil empty slices")
	}
}

func TestDiscover_SingleElement(t *testing.T) {
	t.Parallel()
	elements := []RawElement{
		{Text: "Lonely", Tag: "button", BBox: BBox{X: 500, Y: 400, Width: 80, Height: 30}, Visible: true, Index: 0},
	}

	result := Discover(elements, defaultCfg)

	if len(result.Ungrouped) != 1 {
		t.Errorf("single element should be ungrouped, got %d ungrouped", len(result.Ungrouped))
	}
}

func TestDiscover_InvisibleElementsExcluded(t *testing.T) {
	t.Parallel()
	elements := []RawElement{
		{Text: "Visible", Tag: "button", BBox: BBox{X: 100, Y: 50, Width: 60, Height: 30}, Visible: true, Index: 0},
		{Text: "Hidden", Tag: "button", BBox: BBox{X: 170, Y: 50, Width: 60, Height: 30}, Visible: false, Index: 1},
	}

	result := Discover(elements, defaultCfg)

	if len(result.Ungrouped) != 1 {
		t.Errorf("expected 1 ungrouped (hidden excluded), got %d", len(result.Ungrouped))
	}
	total := countAllMenuItems(result) + len(result.Ungrouped)
	if total != 1 {
		t.Errorf("total visible elements should be 1, got %d", total)
	}
}

func TestDiscover_MixedSemanticAndDivSoup(t *testing.T) {
	t.Parallel()
	elements := []RawElement{
		// Semantic nav
		{Text: "Home", Href: "/", Tag: "a", ParentTag: "nav", BBox: BBox{X: 100, Y: 10, Width: 60, Height: 30}, Visible: true, Index: 0},
		{Text: "About", Href: "/about", Tag: "a", ParentTag: "nav", BBox: BBox{X: 170, Y: 10, Width: 60, Height: 30}, Visible: true, Index: 1},
		// Div-soup tabs (no semantic parent)
		{Text: "Tab 1", Tag: "button", BBox: BBox{X: 100, Y: 200, Width: 80, Height: 30}, Visible: true, Index: 2},
		{Text: "Tab 2", Tag: "button", BBox: BBox{X: 190, Y: 200, Width: 80, Height: 30}, Visible: true, Index: 3},
		{Text: "Tab 3", Tag: "button", BBox: BBox{X: 280, Y: 200, Width: 80, Height: 30}, Visible: true, Index: 4},
		// Standalone button (not part of any menu)
		{Text: "Submit", Tag: "button", BBox: BBox{X: 500, Y: 600, Width: 100, Height: 40}, Visible: true, Index: 5},
	}

	result := Discover(elements, defaultCfg)

	semanticItems := 0
	for _, g := range result.Main {
		if g.Source == "semantic" {
			semanticItems += len(g.Items)
		}
	}
	if semanticItems != 2 {
		t.Errorf("expected 2 semantic main items, got %d", semanticItems)
	}

	totalMenuItems := countAllMenuItems(result)
	if totalMenuItems != 5 {
		t.Errorf("expected 5 total menu items (2 semantic + 3 axis), got %d", totalMenuItems)
	}

	if len(result.Ungrouped) != 1 {
		t.Errorf("expected 1 ungrouped (Submit button), got %d", len(result.Ungrouped))
	}
}

func TestDiscover_SemanticTakesPriorityOverAxis(t *testing.T) {
	t.Parallel()
	// Elements that would cluster by axis, but are already in a semantic group
	elements := []RawElement{
		{Text: "A", Tag: "a", Href: "/a", ParentTag: "nav", BBox: BBox{X: 100, Y: 10, Width: 60, Height: 30}, Visible: true, Index: 0},
		{Text: "B", Tag: "a", Href: "/b", ParentTag: "nav", BBox: BBox{X: 170, Y: 10, Width: 60, Height: 30}, Visible: true, Index: 1},
	}

	result := Discover(elements, defaultCfg)

	// Should be in semantic group, not duplicated in axis groups
	totalMenuItems := countAllMenuItems(result)
	if totalMenuItems != 2 {
		t.Errorf("expected exactly 2 menu items (no duplication), got %d", totalMenuItems)
	}
}

// =============================================================================
// Orientation detection
// =============================================================================

func TestDiscover_HorizontalOrientation(t *testing.T) {
	t.Parallel()
	elements := []RawElement{
		{Text: "A", Tag: "a", Href: "/a", ParentTag: "nav", BBox: BBox{X: 100, Y: 10, Width: 60, Height: 30}, Visible: true, Index: 0},
		{Text: "B", Tag: "a", Href: "/b", ParentTag: "nav", BBox: BBox{X: 170, Y: 10, Width: 60, Height: 30}, Visible: true, Index: 1},
	}

	result := Discover(elements, defaultCfg)

	if len(result.Main) == 0 {
		t.Fatal("expected main menu")
	}
	if result.Main[0].Orientation != "horizontal" {
		t.Errorf("orientation = %q, want horizontal", result.Main[0].Orientation)
	}
}

func TestDiscover_VerticalOrientation(t *testing.T) {
	t.Parallel()
	elements := []RawElement{
		{Text: "A", Tag: "a", Href: "/a", ParentTag: "aside", BBox: BBox{X: 10, Y: 100, Width: 100, Height: 30}, Visible: true, Index: 0},
		{Text: "B", Tag: "a", Href: "/b", ParentTag: "aside", BBox: BBox{X: 10, Y: 140, Width: 100, Height: 30}, Visible: true, Index: 1},
	}

	result := Discover(elements, defaultCfg)

	if len(result.Sidebar) == 0 {
		t.Fatal("expected sidebar menu")
	}
	if result.Sidebar[0].Orientation != "vertical" {
		t.Errorf("orientation = %q, want vertical", result.Sidebar[0].Orientation)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func countAllMenuItems(r Result) int {
	count := 0
	for _, g := range r.Main {
		count += len(g.Items)
	}
	for _, g := range r.Sidebar {
		count += len(g.Items)
	}
	for _, g := range r.Footer {
		count += len(g.Items)
	}
	for _, g := range r.Other {
		count += len(g.Items)
	}
	return count
}

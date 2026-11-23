// menus.go — Clusters page interactables into semantic menu groups.
// Why: Gives AI agents structured navigation data without requiring landmark markup.
// Docs: docs/features/feature/auto-fix/index.md

package menus

// RawElement is a single interactable element as captured by the extension.
type RawElement struct {
	Text       string `json:"text"`
	Href       string `json:"href,omitempty"`
	Tag        string `json:"tag"`
	Type       string `json:"type,omitempty"`
	Role       string `json:"role,omitempty"`
	BBox       BBox   `json:"bbox"`
	ParentTag  string `json:"parent_tag,omitempty"`
	ParentRole string `json:"parent_role,omitempty"`
	Visible    bool   `json:"visible"`
	Index      int    `json:"index"`
}

// BBox is a bounding rectangle from the browser viewport.
type BBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Right returns the right edge X coordinate.
func (b BBox) Right() float64 { return b.X + b.Width }

// Bottom returns the bottom edge Y coordinate.
func (b BBox) Bottom() float64 { return b.Y + b.Height }

// CenterY returns the vertical center.
func (b BBox) CenterY() float64 { return b.Y + b.Height/2 }

// CenterX returns the horizontal center.
func (b BBox) CenterX() float64 { return b.X + b.Width/2 }

// MenuGroup is a cluster of interactable elements identified as a menu.
type MenuGroup struct {
	Classification string     `json:"classification"` // "main", "sidebar", "footer", "other"
	Label          string     `json:"label,omitempty"`
	Items          []MenuItem `json:"items"`
	Orientation    string     `json:"orientation"` // "horizontal", "vertical"
	Source         string     `json:"source"`      // "semantic", "axis", "proximity"
}

// MenuItem is a single item within a menu group.
type MenuItem struct {
	Text  string `json:"text"`
	Href  string `json:"href,omitempty"`
	Tag   string `json:"tag"`
	Role  string `json:"role,omitempty"`
	Index int    `json:"index"`
	BBox  BBox   `json:"-"` // Carried internally for proximity/position checks, stripped from API output
}

// Result is the structured output from menu discovery.
type Result struct {
	Main      []MenuGroup `json:"main"`
	Sidebar   []MenuGroup `json:"sidebar"`
	Footer    []MenuGroup `json:"footer"`
	Other     []MenuGroup `json:"other"`
	Ungrouped []MenuItem  `json:"ungrouped"`
}

// ClaimedIndices returns the set of element indices that belong to any menu group.
func (r Result) ClaimedIndices() map[int]bool {
	m := make(map[int]bool)
	for _, groups := range [][]MenuGroup{r.Main, r.Sidebar, r.Footer, r.Other} {
		for _, g := range groups {
			for _, item := range g.Items {
				m[item.Index] = true
			}
		}
	}
	return m
}

// Config tunes the clustering heuristic thresholds.
type Config struct {
	// AxisAlignmentThreshold is the max Y (or X) deviation for elements
	// to be considered on the same axis. Accounts for slight misalignment.
	AxisAlignmentThreshold float64

	// ProximityMaxGap is the max gap between adjacent element borders
	// for them to be considered part of the same menu.
	ProximityMaxGap float64

	// MinGroupSize is the minimum number of elements to form a menu group.
	MinGroupSize int

	// ViewportWidth and ViewportHeight for positional classification.
	ViewportWidth  float64
	ViewportHeight float64
}

// DefaultConfig returns sensible defaults for menu detection.
func DefaultConfig() Config {
	return Config{
		AxisAlignmentThreshold: 10,
		ProximityMaxGap:        30,
		MinGroupSize:           2,
		ViewportWidth:          1440,
		ViewportHeight:         900,
	}
}

// semanticLandmarks are parent tags/roles that indicate a navigation region.
var semanticLandmarks = map[string]string{
	"nav":    "main",
	"header": "main",
	"footer": "footer",
	"aside":  "sidebar",
}

var semanticRoles = map[string]string{
	"navigation":    "main",
	"banner":        "main",
	"contentinfo":   "footer",
	"complementary": "sidebar",
}

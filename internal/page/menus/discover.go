// discover.go — 3-layer heuristic for clustering interactables into menu groups.
// Why: Semantic elements alone miss div-soup menus. Axis + proximity catches them.
// Docs: docs/features/feature/auto-fix/index.md

package menus

import (
	"math"
	"sort"
)

// Discover runs the 3-layer heuristic on raw elements and returns structured menus.
func Discover(elements []RawElement, cfg Config) Result {
	result := Result{
		Main:      []MenuGroup{},
		Sidebar:   []MenuGroup{},
		Footer:    []MenuGroup{},
		Other:     []MenuGroup{},
		Ungrouped: []MenuItem{},
	}

	visible := filterVisible(elements)
	if len(visible) == 0 {
		return result
	}

	// Layer 1: Group by semantic parent (nav, header, footer, aside, ARIA roles)
	semanticGroups, remaining := groupBySemantic(visible)

	// Semantic groups bypass proximity — landmark parents are authoritative.

	// Layer 2: Cluster remaining by axis alignment
	axisClusters, remaining := clusterByAxis(remaining, cfg)

	// Layer 3: Validate axis clusters by border proximity
	confirmedClusters, rejected := filterByProximity(axisClusters, cfg)

	// Rejected axis clusters go back to ungrouped
	remaining = append(remaining, rejected...)

	// Classify all groups and assign to result buckets
	for _, g := range semanticGroups {
		g.Source = "semantic"
		g.Orientation = detectOrientation(g.Items)
		classifyAndAssign(&result, g, cfg)
	}
	for _, g := range confirmedClusters {
		g.Source = "proximity"
		classifyAndAssign(&result, g, cfg)
	}

	// Remaining elements are ungrouped
	for _, el := range remaining {
		result.Ungrouped = append(result.Ungrouped, toMenuItem(el))
	}

	return result
}

func filterVisible(elements []RawElement) []RawElement {
	out := make([]RawElement, 0, len(elements))
	for _, el := range elements {
		if el.Visible {
			out = append(out, el)
		}
	}
	return out
}

// groupBySemantic clusters elements by their semantic parent tag/role.
// Returns the semantic groups and the remaining ungrouped elements.
func groupBySemantic(elements []RawElement) ([]MenuGroup, []RawElement) {
	// Group by (parentTag, parentRole) key
	type groupKey struct{ tag, role string }
	groups := make(map[groupKey][]RawElement)
	var remaining []RawElement

	for _, el := range elements {
		classification := classifySemanticParent(el.ParentTag, el.ParentRole)
		if classification != "" {
			key := groupKey{el.ParentTag, el.ParentRole}
			groups[key] = append(groups[key], el)
		} else {
			remaining = append(remaining, el)
		}
	}

	var menuGroups []MenuGroup
	for key, elems := range groups {
		classification := classifySemanticParent(key.tag, key.role)
		items := make([]MenuItem, len(elems))
		for i, el := range elems {
			items[i] = toMenuItem(el)
		}
		menuGroups = append(menuGroups, MenuGroup{
			Classification: classification,
			Items:          items,
		})
	}

	return menuGroups, remaining
}

func classifySemanticParent(tag, role string) string {
	if c, ok := semanticLandmarks[tag]; ok {
		return c
	}
	if c, ok := semanticRoles[role]; ok {
		return c
	}
	return ""
}

// clusterByAxis groups elements that share a Y center (horizontal) or X center (vertical).
func clusterByAxis(elements []RawElement, cfg Config) ([]MenuGroup, []RawElement) {
	if len(elements) < cfg.MinGroupSize {
		return nil, elements
	}

	used := make([]bool, len(elements))
	indexPos := make(map[int]int, len(elements))
	for i, el := range elements {
		indexPos[el.Index] = i
	}

	markUsed := func(el RawElement) {
		if pos, ok := indexPos[el.Index]; ok {
			used[pos] = true
		}
	}

	// Try horizontal clusters first (shared Y)
	var clusters []MenuGroup
	yClusters := clusterBySharedValue(elements, func(el RawElement) float64 {
		return el.BBox.CenterY()
	}, cfg.AxisAlignmentThreshold, cfg.MinGroupSize)

	for _, cluster := range yClusters {
		items := make([]MenuItem, len(cluster))
		for i, el := range cluster {
			items[i] = toMenuItem(el)
			markUsed(el)
		}
		clusters = append(clusters, MenuGroup{
			Items:       items,
			Orientation: "horizontal",
		})
	}

	// Try vertical clusters on remaining elements (shared X)
	var unusedForVertical []RawElement
	for i, el := range elements {
		if !used[i] {
			unusedForVertical = append(unusedForVertical, el)
		}
	}

	xClusters := clusterBySharedValue(unusedForVertical, func(el RawElement) float64 {
		return el.BBox.CenterX()
	}, cfg.AxisAlignmentThreshold, cfg.MinGroupSize)

	for _, cluster := range xClusters {
		items := make([]MenuItem, len(cluster))
		for i, el := range cluster {
			items[i] = toMenuItem(el)
			markUsed(el)
		}
		clusters = append(clusters, MenuGroup{
			Items:       items,
			Orientation: "vertical",
		})
	}

	var remaining []RawElement
	for i, el := range elements {
		if !used[i] {
			remaining = append(remaining, el)
		}
	}

	return clusters, remaining
}

// clusterBySharedValue groups elements whose extracted value is within threshold.
func clusterBySharedValue(elements []RawElement, valueFn func(RawElement) float64, threshold float64, minSize int) [][]RawElement {
	if len(elements) < minSize {
		return nil
	}

	// Sort by value
	type indexed struct {
		el  RawElement
		val float64
	}
	sorted := make([]indexed, len(elements))
	for i, el := range elements {
		sorted[i] = indexed{el, valueFn(el)}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].val < sorted[j].val
	})

	var clusters [][]RawElement
	cluster := []indexed{sorted[0]}

	for i := 1; i < len(sorted); i++ {
		if math.Abs(sorted[i].val-cluster[0].val) <= threshold {
			cluster = append(cluster, sorted[i])
		} else {
			if len(cluster) >= minSize {
				elems := make([]RawElement, len(cluster))
				for j, c := range cluster {
					elems[j] = c.el
				}
				clusters = append(clusters, elems)
			}
			cluster = []indexed{sorted[i]}
		}
	}
	if len(cluster) >= minSize {
		elems := make([]RawElement, len(cluster))
		for j, c := range cluster {
			elems[j] = c.el
		}
		clusters = append(clusters, elems)
	}

	return clusters
}

// filterByProximity validates axis clusters — elements must be close together.
func filterByProximity(clusters []MenuGroup, cfg Config) (confirmed []MenuGroup, rejected []RawElement) {
	for _, g := range clusters {
		if proximityCheck(g, cfg) {
			confirmed = append(confirmed, g)
		} else {
			// Reject: convert items back to raw elements (for ungrouped)
			for _, item := range g.Items {
				rejected = append(rejected, RawElement{
					Text:    item.Text,
					Href:    item.Href,
					Tag:     item.Tag,
					Role:    item.Role,
					Index:   item.Index,
					Visible: true,
				})
			}
		}
	}
	return
}

// proximityCheck verifies that adjacent items in a group have small border gaps.
func proximityCheck(g MenuGroup, cfg Config) bool {
	if len(g.Items) < 2 {
		return false
	}

	if g.Orientation == "horizontal" {
		// Sort by X position, compare X against previous right edge
		return adjacentGapsWithin(g.Items, cfg.ProximityMaxGap, itemLeftEdge, itemRightEdge)
	}
	// Sort by Y position, compare Y against previous bottom edge
	return adjacentGapsWithin(g.Items, cfg.ProximityMaxGap, itemTopEdge, itemBottomEdge)
}

// adjacentGapsWithin sorts items by pos and reports whether every gap between
// one item's leading edge and the previous item's far edge stays within maxGap.
func adjacentGapsWithin(items []MenuItem, maxGap float64, pos, farEdge func(MenuItem) float64) bool {
	sorted := make([]MenuItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return pos(sorted[i]) < pos(sorted[j])
	})
	for i := 1; i < len(sorted); i++ {
		gap := pos(sorted[i]) - farEdge(sorted[i-1])
		if gap > maxGap {
			return false
		}
	}
	return true
}

func itemLeftEdge(item MenuItem) float64   { return item.BBox.X }
func itemRightEdge(item MenuItem) float64  { return item.BBox.Right() }
func itemTopEdge(item MenuItem) float64    { return item.BBox.Y }
func itemBottomEdge(item MenuItem) float64 { return item.BBox.Bottom() }

func classifyAndAssign(result *Result, g MenuGroup, cfg Config) {
	classification := g.Classification
	if classification == "" {
		classification = classifyByItemPositions(g, cfg)
	}
	g.Classification = classification

	switch classification {
	case "main":
		result.Main = append(result.Main, g)
	case "sidebar":
		result.Sidebar = append(result.Sidebar, g)
	case "footer":
		result.Footer = append(result.Footer, g)
	default:
		result.Other = append(result.Other, g)
	}
}

// classifyByItemPositions uses the bbox data carried on MenuItems.
func classifyByItemPositions(g MenuGroup, cfg Config) string {
	if len(g.Items) == 0 {
		return "other"
	}

	var avgX, avgY float64
	for _, item := range g.Items {
		avgX += item.BBox.CenterX()
		avgY += item.BBox.CenterY()
	}
	n := float64(len(g.Items))
	avgX /= n
	avgY /= n

	if avgY < cfg.ViewportHeight*0.15 {
		return "main"
	}
	if avgY > cfg.ViewportHeight*0.85 {
		return "footer"
	}
	if avgX < cfg.ViewportWidth*0.25 {
		return "sidebar"
	}
	if avgX > cfg.ViewportWidth*0.75 {
		return "sidebar"
	}
	return "other"
}

// detectOrientation determines if a group of items is horizontal or vertical.
func detectOrientation(items []MenuItem) string {
	if len(items) < 2 {
		return "horizontal"
	}

	first := items[0].BBox
	last := items[len(items)-1].BBox

	xSpread := math.Abs(last.CenterX() - first.CenterX())
	ySpread := math.Abs(last.CenterY() - first.CenterY())

	if ySpread > xSpread {
		return "vertical"
	}
	return "horizontal"
}

func toMenuItem(el RawElement) MenuItem {
	return MenuItem{
		Text:  el.Text,
		Href:  el.Href,
		Tag:   el.Tag,
		Role:  el.Role,
		Index: el.Index,
		BBox:  el.BBox,
	}
}

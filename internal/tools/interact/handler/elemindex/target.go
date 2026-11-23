// target.go — What one entry in an element-index snapshot points at: a selector, an
// accessibility backend id, and where the element sits in the viewport.
//
// A snapshot used to hold plain selector strings. `find` then introduced a SECOND handle
// space — accessibility refs of the form ax_<backendNodeId> — carrying no generation stamp
// and therefore no staleness check, while Chrome reuses a backendNodeId once the node it
// named has been destroyed. A ref handed back after a re-render could resolve to an
// unrelated element and the agent would click the wrong control with no error.
//
// Both handles now name the same Target inside one generation-stamped snapshot, so a ref
// is refused after a re-render for exactly the same reason a numeric index is.
package elemindex

import (
	"strconv"
	"strings"
)

// refPrefix is what an accessibility handle looks like on the wire: ax_<backendNodeId>.
const refPrefix = "ax_"

// Target is one addressable element inside a snapshot.
type Target struct {
	// Selector reaches the element through the DOM. Empty for accessibility candidates,
	// whose whole point is that no CSS selector names them.
	Selector string
	// AXBackendID is Chrome's backend DOM node id. Zero when the target came from the DOM
	// scan rather than the accessibility tree.
	AXBackendID int
	Role        string
	Name        string
	// CenterX/CenterY is the viewport point to address when there is no selector.
	// HasCenter separates "no geometry was resolved" from a legitimate 0,0.
	CenterX   float64
	CenterY   float64
	HasCenter bool
}

// parseRef reads the backend node id out of an "ax_<n>" handle.
//
// Zero is rejected along with malformed input: Chrome never issues backendNodeId 0, and
// accepting it would let the zero value of Target answer for a real element.
func parseRef(ref string) (int, bool) {
	trimmed := strings.TrimSpace(ref)
	if !strings.HasPrefix(trimmed, refPrefix) {
		return 0, false
	}
	id, err := strconv.Atoi(trimmed[len(refPrefix):])
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

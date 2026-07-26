// Purpose: Implements network body and waterfall ingestion, enrichment, ring-buffer storage and retrieval.
// Why: Preserves request/response and request-timing evidence under bounded memory.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/observe/index.md

package capture

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// detectAndSetBinaryFormat infers payload format only when not already set.
//
// Failure semantics:
// - Detection is best-effort; unknown formats leave fields empty without erroring ingestion.
func detectAndSetBinaryFormat(body *NetworkBody) {
	if body.BinaryFormat != "" {
		return
	}
	if len(body.RequestBody) > 0 {
		if format := util.DetectBinaryFormat([]byte(body.RequestBody)); format != nil {
			body.BinaryFormat = format.Name
			body.FormatConfidence = format.Confidence
			return
		}
	}
	if len(body.ResponseBody) > 0 {
		if format := util.DetectBinaryFormat([]byte(body.ResponseBody)); format != nil {
			body.BinaryFormat = format.Name
			body.FormatConfidence = format.Confidence
		}
	}
}

// AddNetworkBodies ingests a batch into the network evidence ring buffer.
//
// Invariants:
// - Each networkBodyEntry stores the body and its ingestion timestamp together.
// - Totals are monotonic (`networkTotalAdded`, `networkErrorTotalAdded`) and never decremented.
// - Active test IDs are snapshotted once per batch for consistent event tagging.
//
// Failure semantics:
// - Batch ingestion never partially fails; over-capacity data is deterministically evicted.
func (c *Capture) AddNetworkBodies(bodies []NetworkBody) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	activeTestIDs := make([]string, 0)
	for testID := range c.extensionState.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}

	c.buffers.appendNetworkBodies(bodies, activeTestIDs, now)
}

// GetNetworkBodyCount returns the current number of network bodies in the buffer.
func (c *Capture) GetNetworkBodyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkCount()
}

// AddNetworkWaterfallEntries adds network waterfall entries to the buffer.
// Each entry is tagged with the page URL and current timestamp.
func (c *Capture) AddNetworkWaterfallEntries(entries []NetworkWaterfallEntry, pageURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.networkWaterfall.appendEntries(entries, pageURL, time.Now())
}

// GetNetworkWaterfallCount returns the current number of waterfall entries.
func (c *Capture) GetNetworkWaterfallCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.networkWaterfall.count()
}

// GetNetworkWaterfallEntries returns all waterfall entries.
func (c *Capture) GetNetworkWaterfallEntries() []NetworkWaterfallEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.networkWaterfall.count() == 0 {
		return []NetworkWaterfallEntry{}
	}
	return c.networkWaterfall.snapshot()
}

// appendEntries appends entries, annotates each one with page URL/timestamp, and enforces capacity.
func (b *NetworkWaterfallBuffer) appendEntries(entries []NetworkWaterfallEntry, pageURL string, now time.Time) {
	for i := range entries {
		entries[i].PageURL = pageURL
		entries[i].Timestamp = now
		b.entries = append(b.entries, entries[i])
	}

	if len(b.entries) <= b.capacity {
		return
	}
	kept := make([]NetworkWaterfallEntry, b.capacity)
	copy(kept, b.entries[len(b.entries)-b.capacity:])
	b.entries = kept
}

// count returns the number of buffered waterfall entries.
func (b *NetworkWaterfallBuffer) count() int {
	return len(b.entries)
}

// snapshot returns a detached copy of buffered entries.
func (b *NetworkWaterfallBuffer) snapshot() []NetworkWaterfallEntry {
	out := make([]NetworkWaterfallEntry, len(b.entries))
	copy(out, b.entries)
	return out
}

// clear removes all entries and returns the number removed.
func (b *NetworkWaterfallBuffer) clear() int {
	count := len(b.entries)
	b.entries = make([]NetworkWaterfallEntry, 0, b.capacity)
	return count
}

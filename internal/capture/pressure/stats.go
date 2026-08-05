// stats.go — Defines bounded capture-stream pressure measurements.
// Docs: docs/features/feature/operational-observability/index.md

package pressure

import "time"

// Stats describes the bounded retention state of one capture stream.
type Stats struct {
	Size      int           `json:"size"`
	Capacity  int           `json:"capacity"`
	Dropped   int64         `json:"dropped_count"`
	OldestAge time.Duration `json:"oldest_age"`
}

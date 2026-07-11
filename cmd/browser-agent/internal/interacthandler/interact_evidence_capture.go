// interact_evidence_capture.go — Captures mutation and screenshot evidence during interact commands.
// Purpose: Provides post-action proof artifacts (DOM diffs, screenshots) without requiring manual reproduction.

package interacthandler

import (
	"strings"
	"time"
)

const (
	// evidenceScreenshotTimeout is the timeout for creating and waiting for
	// screenshot evidence capture queries.
	evidenceScreenshotTimeout = 12 * time.Second

	// evidenceRetryDelay is the pause between evidence capture retry attempts.
	evidenceRetryDelay = 150 * time.Millisecond
)

// EvidenceShot holds a single evidence screenshot result.
type EvidenceShot = evidenceShot

// EvidenceCaptureFn is the pluggable evidence capture function.
// Tests can replace it to avoid real screenshot I/O.
var evidenceCaptureFn func(deps *Deps, clientID string) evidenceShot

func (h *InteractActionHandler) captureEvidenceWithRetry(clientID string) evidenceShot {
	retries := evidenceRetryCount()
	attempts := retries + 1
	last := evidenceShot{Error: "evidence_capture_not_attempted"}

	captureFn := evidenceCaptureFn
	if captureFn == nil && h.deps.DefaultEvidenceCapture != nil {
		captureFn = func(_ *Deps, cid string) evidenceShot {
			return h.deps.DefaultEvidenceCapture(cid)
		}
	}
	if captureFn == nil {
		return evidenceShot{Error: "evidence_capture_not_configured"}
	}

	for i := 0; i < attempts; i++ {
		shot := captureFn(h.deps, clientID)
		shot.Attempts = i + 1
		if strings.TrimSpace(shot.Path) != "" {
			return shot
		}
		if strings.TrimSpace(shot.Error) == "" {
			shot.Error = "evidence_capture_failed"
		}
		last = shot
		if i < attempts-1 {
			time.Sleep(evidenceRetryDelay)
		}
	}

	return last
}

// SetEvidenceCaptureFn overrides the evidence capture function (for testing).
func SetEvidenceCaptureFn(fn func(deps *Deps, clientID string) EvidenceShot) {
	evidenceCaptureFn = fn
}

// ResetEvidenceCaptureFn restores the default evidence capture function.
func ResetEvidenceCaptureFn() {
	evidenceCaptureFn = nil
}

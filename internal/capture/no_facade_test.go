// no_facade_test.go — Guards capture against lifecycle compatibility exports.

package capture

import (
	"os"
	"strings"
	"testing"
)

func TestCaptureHasNoCompatibilityAliases(t *testing.T) {
	t.Parallel()

	model, err := os.ReadFile("model.go")
	if err != nil {
		t.Fatal(err)
	}
	captureSource, err := os.ReadFile("capture.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(model) + string(captureSource)
	for _, forbidden := range []string{
		"LifecycleEvent =",
		"LifecycleListener =",
		"LifecycleObserver =",
		"EventCircuitOpened =",
		"NewLifecycleObserver =",
		"ParseLifecycleEvent =",
		"SetLifecycleCallback(",
		"AddLifecycleCallback(",
		"QueryDispatcher =",
		"QuerySnapshot =",
		"NewQueryDispatcher =",
		"CircuitBreaker =",
		"HealthResponse =",
		"RateLimitResponse =",
		"NewCircuitBreaker =",
		"DebugLogger =",
		"NewDebugLogger =",
		"PlaybackSession =",
		"PlaybackResult =",
		"Coordinates =",
		"LogDiffResult =",
		"DiffLogEntry =",
		"ValueChange =",
		"ActionComparison =",
		"recordingStorageMax =",
		"recordingWarningLevel =",
		"validateRecordingID =",
		"calculateRecordingSize =",
	} {
		if strings.Contains(source, forbidden) {
			t.Errorf("capture retains compatibility surface %q", forbidden)
		}
	}
}

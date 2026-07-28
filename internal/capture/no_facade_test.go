// no_facade_test.go — Guards capture against lifecycle compatibility exports.

package capture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureHasNoCompatibilityAliases(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var source strings.Builder
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		source.Write(contents)
	}
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
		"ResourceEntry =",
		"ResourceDiff =",
		"CausalDiffResult =",
		"WSConnectionTracker =",
		"queryResultTTL =",
		"Store = Capture",
		"Snapshot = CaptureSnapshot",
		"WebSocketEvent =",
		"SamplingInfo =",
		"WebSocketEventFilter =",
		"WebSocketStatusFilter =",
		"WebSocketStatusResponse =",
		"WebSocketConnection =",
		"WebSocketClosedConnection =",
		"WebSocketMessageRate =",
		"WebSocketDirectionStats =",
		"WebSocketLastMessage =",
		"WebSocketMessagePreview =",
		"WebSocketSchema =",
		"WebSocketSamplingStatus =",
		"NetworkWaterfallEntry =",
		"NetworkWaterfallPayload =",
		"NetworkBody =",
		"NetworkBodyFilter =",
		"EnhancedAction =",
		"EnhancedActionFilter =",
		"ExtensionLog =",
		"PollingLogEntry =",
		"HTTPDebugEntry =",
		"BufferClearCounts =",
		"CompleteCommand(",
		"ExtensionStatus struct",
		"UpdateExtensionStatus(",
		"func (c *Capture) EventBuffers(",
		"func (c *Capture) NetworkWaterfallStore(",
		"func (c *Capture) ExtensionLogStore(",
		"func (c *Capture) PerformanceSnapshotStore(",
		"func (c *Capture) GetExtensionVersion(",
		"func (c *Capture) UnsubscribeLifecycle(",
		"func (c *Capture) SaveSettingsToDisk(",
		"func (c *Capture) ClearExtensionLogs(",
		"func (c *Capture) GetNetworkTimestamps(",
		"func (c *Capture) GetWebSocketTimestamps(",
		"func (c *Capture) GetActionTimestamps(",
		"func (c *Capture) GetWebSocketBufferMemory(",
		"func (c *Capture) GetNetworkBodiesBufferMemory(",
		"func (c *Capture) GetNetworkBodyCount(",
		"func (c *Capture) GetNetworkWaterfallCount(",
		"func (c *Capture) GetWebSocketEventCount(",
		"func (c *Capture) GetEnhancedActionCount(",
	} {
		if strings.Contains(source.String(), forbidden) {
			t.Errorf("capture retains compatibility surface %q", forbidden)
		}
	}
}

// http_handlers_owner_test.go — Structural ownership tests for capture HTTP handlers.

package capture

import (
	"reflect"
	"testing"
)

func TestHTTPHandlersOwnRequestBoundaryWithoutCaptureFacade(t *testing.T) {
	handlerMethods := []string{
		"HandleEnhancedActions",
		"HandleHealth",
		"HandleNetworkBodies",
		"HandleNetworkWaterfall",
		"HandlePerformanceSnapshots",
		"HandleQueryResult",
		"HandleRecordingStorage",
		"HandleWebSocketEvents",
		"HandleWebSocketStatus",
	}

	captureType := reflect.TypeOf((*Capture)(nil))
	handlerType := reflect.TypeOf((*HTTPHandlers)(nil))
	for _, method := range handlerMethods {
		if _, exists := captureType.MethodByName(method); exists {
			t.Errorf("Capture retains HTTP compatibility facade %s", method)
		}
		if _, exists := handlerType.MethodByName(method); !exists {
			t.Errorf("HTTPHandlers does not own %s", method)
		}
	}
}

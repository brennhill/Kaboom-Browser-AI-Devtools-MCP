// sync_handler_owner_test.go — Structural ownership tests for extension sync transport.

package integrationtest

import (
	. "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"reflect"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
)

func TestSyncHandlerOwnsTransportWithoutCaptureFacade(t *testing.T) {
	captureType := reflect.TypeOf((*Capture)(nil))
	syncType := reflect.TypeOf((*syncruntime.Handler)(nil))

	for _, method := range []string{"HandleSync", "GetPendingQueriesDisconnectAware"} {
		if _, exists := captureType.MethodByName(method); exists {
			t.Errorf("Capture retains sync compatibility facade %s", method)
		}
		if _, exists := syncType.MethodByName(method); !exists {
			t.Errorf("SyncHandler does not own %s", method)
		}
	}
}

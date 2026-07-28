// health_reader_owner_test.go — Structural ownership tests for aggregate capture health.

package capture

import (
	"reflect"
	"testing"
)

func TestHealthReaderOwnsSnapshotWithoutCaptureFacade(t *testing.T) {
	if _, exists := reflect.TypeOf((*Capture)(nil)).MethodByName("GetHealthSnapshot"); exists {
		t.Error("Capture retains GetHealthSnapshot compatibility facade")
	}
	if _, exists := reflect.TypeOf((*HealthReader)(nil)).MethodByName("Snapshot"); !exists {
		t.Error("HealthReader does not own Snapshot")
	}
}

// no_facade_test.go — Guards capture against lifecycle compatibility exports.

package capture

import (
	"os"
	"strings"
	"testing"
)

func TestCaptureHasNoLifecycleCompatibilitySurface(t *testing.T) {
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
	} {
		if strings.Contains(source, forbidden) {
			t.Errorf("capture retains lifecycle compatibility surface %q", forbidden)
		}
	}
}

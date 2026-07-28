// feature_usage_test.go — Feature-usage observer ownership and concurrency tests.
package capture

import (
	"sync"
	"testing"
)

func TestFeatureUsageObserverNotifiesCurrentCallback(t *testing.T) {
	t.Parallel()

	observer := newFeatureUsageObserver()
	got := make(chan map[string]bool, 1)
	observer.SetCallback(func(features map[string]bool) {
		got <- features
	})

	features := map[string]bool{"screenshot": true}
	observer.Notify(features)

	if received := <-got; !received["screenshot"] {
		t.Fatalf("callback received %#v, want screenshot=true", received)
	}
}

func TestFeatureUsageObserverAllowsConcurrentReplacementAndNotification(t *testing.T) {
	t.Parallel()

	observer := newFeatureUsageObserver()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			observer.SetCallback(func(map[string]bool) {})
		}()
		go func() {
			defer wg.Done()
			observer.Notify(map[string]bool{"screenshot": true})
		}()
	}
	wg.Wait()
}

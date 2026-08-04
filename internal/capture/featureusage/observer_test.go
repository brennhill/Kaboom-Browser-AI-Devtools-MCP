// observer_test.go — Verifies synchronized feature-usage callback ownership.

package featureusage

import (
	"sync"
	"testing"
)

func TestObserverNotifiesCurrentCallback(t *testing.T) {
	t.Parallel()
	observer := New()
	called := false
	observer.SetCallback(func(features map[string]bool) {
		called = features["screenshot"]
	})
	observer.Notify(map[string]bool{"screenshot": true})
	if !called {
		t.Fatal("current callback was not notified")
	}
}

func TestObserverAllowsConcurrentReplacementAndNotification(t *testing.T) {
	t.Parallel()
	observer := New()
	var wait sync.WaitGroup
	for index := 0; index < 50; index++ {
		wait.Add(2)
		go func() {
			defer wait.Done()
			observer.SetCallback(func(map[string]bool) {})
		}()
		go func() {
			defer wait.Done()
			observer.Notify(map[string]bool{"video": true})
		}()
	}
	wait.Wait()
}

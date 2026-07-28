// extension_state_test_helpers_test.go — Lock-safe setup for extension-state tests.
package capture

func mutateExtensionStateForTest(runtime *ExtensionRuntime, mutate func(*ExtensionState)) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	mutate(&runtime.state)
}

func extensionStateSnapshotForTest(runtime *ExtensionRuntime) ExtensionState {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.state
}

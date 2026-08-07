// Purpose: Unit tests for browser-agent server core logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import "testing"

func TestServerRuntimeConfigurationIsInstanceOwned(t *testing.T) {
	first := &Server{}
	second := &Server{}
	first.applyRuntimeConfig(&serverConfig{uploadAutomation: true})
	second.applyRuntimeConfig(&serverConfig{uploadAutomation: false})

	if !first.uploadAutomation || second.uploadAutomation {
		t.Fatal("upload automation configuration leaked between server instances")
	}
}

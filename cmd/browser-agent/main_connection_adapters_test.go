// main_connection_adapters_test.go — Tests MCP runtime adapter boundaries.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"testing"

	terminalintent "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/intent"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/sessionrelay"
)

func TestServerIntentDepsExposeOwnedState(t *testing.T) {
	server := &Server{intentStore: terminalintent.NewStore()}
	deps := &serverIntentDeps{server: server}
	if deps.GetPtyRelays() != nil || deps.GetIntentStore() != server.intentStore {
		t.Fatal("empty server intent dependencies mismatch")
	}
	server.ptyRelays = sessionrelay.NewMap()
	if deps.GetPtyRelays() == nil {
		t.Fatal("relay map was not exposed")
	}
}

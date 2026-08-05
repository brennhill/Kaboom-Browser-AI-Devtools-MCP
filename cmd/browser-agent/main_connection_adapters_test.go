// main_connection_adapters_test.go — Tests MCP runtime adapter boundaries.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal"
	terminalintent "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/intent"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
)

func TestSessionClientRegistryAdapterDelegates(t *testing.T) {
	if newSessionClientRegistryAdapter(nil) != nil {
		t.Fatal("nil registry produced adapter")
	}
	adapter, ok := newSessionClientRegistryAdapter(clientreg.NewClientRegistry()).(*sessionClientRegistryAdapter)
	if !ok {
		t.Fatal("adapter has unexpected type")
	}
	registered := adapter.Register("/tmp/project")
	if registered == nil || adapter.Count() != 1 {
		t.Fatalf("register = %#v, count=%d", registered, adapter.Count())
	}
	list := adapter.List()
	if list == nil {
		t.Fatal("list is nil")
	}
	clients := adapter.reg.List()
	if len(clients) != 1 {
		t.Fatalf("clients = %#v", clients)
	}
	if adapter.Get(clients[0].ID) == nil {
		t.Fatal("registered client could not be retrieved")
	}
	if !adapter.Unregister(clients[0].ID) || adapter.Count() != 0 {
		t.Fatal("registered client could not be removed")
	}
}

func TestServerIntentDepsExposeOwnedState(t *testing.T) {
	server := &Server{intentStore: terminalintent.NewStore()}
	deps := &serverIntentDeps{server: server}
	if deps.GetPtyRelays() != nil || deps.GetIntentStore() != server.intentStore {
		t.Fatal("empty server intent dependencies mismatch")
	}
	server.ptyRelays = terminal.NewMap()
	if deps.GetPtyRelays() == nil {
		t.Fatal("relay map was not exposed")
	}
}

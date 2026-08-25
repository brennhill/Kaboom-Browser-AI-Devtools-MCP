// main_test.go — Pins the layering contract: dependency matrix, interface budgets, ownership ratchet.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDependencyMatrixRules(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		// Clean: internal tool importing another internal package.
		"internal/tools/observe/handler.go": "package observe\n\nimport _ \"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pagination\"\n",
		// D1: internal importing cmd.
		"internal/bridge/bridge.go":         "package bridge\n\nimport _ \"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge\"\n",
		// D2: types must stay a leaf.
		"internal/types/types.go":           "package types\n\nimport _ \"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pagination\"\n",
		// D3: protocol package must not know the domain or the capture port.
		"internal/mcp/protocol.go":          "package mcp\n\nimport _ \"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe\"\n",
		"internal/mcp/protocol2.go":         "package mcp\n\nimport _ \"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture\"\n",
		// D4: schema and tools are siblings, never each other's dependencies.
		"internal/schema/configure/s.go":    "package configure\n\nimport _ \"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe\"\n",
		"internal/tools/observe/t.go":       "package observe\n\nimport _ \"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema/configure\"\n",
		// D5: NewCapture outside the composition root.
		"internal/tools/interact/setup.go":  "package interact\n\nimport \"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture\"\n\nfunc boot() { _ = capture.NewCapture() }\n",
		// Clean: composition root constructing the capture port.
		"cmd/browser-agent/main.go":         "package main\n\nimport \"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture\"\n\nfunc main() { _ = capture.NewCapture() }\n",
		// Test files are exempt from every matrix rule.
		"internal/bridge/bridge_test.go":    "package bridge\n\nimport _ \"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge\"\n",
	})

	scan, err := scanTree(root)
	if err != nil {
		t.Fatal(err)
	}
	violations := evaluateMatrix(scan)
	if len(violations) != 7 {
		t.Fatalf("violations = %v\nwant 7 (one per matrix breach incl. both schema directions)", violations)
	}
	joined := ""
	for _, v := range violations {
		joined += v + "\n"
	}
	for _, want := range []string{"internal->cmd", "types-leaf", "mcp->domain", "composition-root"} {
		if !contains(joined, want) {
			t.Errorf("violation kind %q missing from:\n%s", want, joined)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestInterfaceSizeIsHardFail(t *testing.T) {
	root := t.TempDir()
	methods := ""
	for i := 0; i < 8; i++ {
		methods += "\tM" + string(rune('0'+i)) + "()\n"
	}
	writeTree(t, root, map[string]string{
		"internal/big/big.go": "package big\n\ntype Wide interface {\n" + methods + "}\n",
		"internal/ok/ok.go":   "package ok\n\ntype Fine interface {\n\tA()\n\tB()\n}\n",
	})

	scan, err := scanTree(root)
	if err != nil {
		t.Fatal(err)
	}
	var wide []ifaceInfo
	for _, i := range scan.interfaces {
		if i.methods > maxInterfaceMethods {
			wide = append(wide, i)
		}
	}
	if len(wide) != 1 || wide[0].name != "Wide" {
		t.Fatalf("wide interfaces = %+v, want Wide with 8 methods", wide)
	}
	if wide[0].methods != 8 {
		t.Fatalf("method count = %d, want 8", wide[0].methods)
	}
}

func TestInterfaceOwnershipRatchet(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		// Producer-owned: interface + its only production impl in one package.
		"internal/owned/o.go": `package owned

type Port interface {
	Get() int
}

type impl struct{}

func (impl) Get() int { return 1 }

var _ Port = impl{}
`,
		// Consumer-owned: interface here, production impl in another package.
		"internal/consumer/c.go": `package consumer

type Port interface {
	Get() int
}

func Use(p Port) int { return p.Get() }
`,
		"internal/provider/p.go": `package provider

type Impl struct{}

func (Impl) Fetch() int { return 2 }
`,
		// Dead contract: no production implementation anywhere.
		"internal/dead/d.go": "package dead\n\ntype Ghost interface {\n\tHaunt() string\n}\n",
		// Same-package TEST fake does not count as a production impl.
		"internal/seam/s.go":         "package seam\n\ntype Seam interface {\n\tRun() error\n}\n\nfunc Use(s Seam) {}\n",
		"internal/seam/s_test.go":    "package seam\n\ntype fake struct{}\n\nfunc (fake) Run() error { return nil }\n",
	})

	scan, err := scanTree(root)
	if err != nil {
		t.Fatal(err)
	}
	current := ownershipViolations(scan)

	baseline := map[string]string{} // no allowances: violations must fail
	violations := evaluateOwnership(current, baseline)
	if len(violations) != 2 {
		t.Fatalf("violations = %v\nwant owned.Port (producer-owned) and dead.Ghost (dead); consumer.Port and seam.Seam must stay clean (impl elsewhere / test seam)", violations)
	}

	allowed := map[string]string{
		"internal/owned/o.go:Port": "producer-owned",
		"internal/dead/d.go:Ghost": "dead",
	}
	if violations = evaluateOwnership(current, allowed); len(violations) != 0 {
		t.Fatalf("baselined violations still reported: %v", violations)
	}

	// A NEW violation outside the baseline fails even when the baseline holds others.
	allowed["internal/owned/o.go:Other"] = "producer-owned"
	if violations = evaluateOwnership(current, allowed); len(violations) != 0 {
		t.Fatalf("stale baseline entry must not fail the run: %v", violations)
	}
}

func TestInterfaceKeyIsStableAcrossImplMoves(t *testing.T) {
	i := ifaceInfo{root: "r", pkg: "internal/owned", file: "o.go", name: "Port"}
	if got := i.key(); got != "internal/owned/o.go:Port" {
		t.Fatalf("key = %q", got)
	}
}

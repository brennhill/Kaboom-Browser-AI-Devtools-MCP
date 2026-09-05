// contract.go — Loads and freezes the declared MCP response contract.
//
// PURPOSE: the declared shapes live in a checked-in file so a review sees the
// diff. The file is GENERATED from real responses, never hand-typed: a
// hand-maintained field list drifts from the handler that produces it, which is
// the failure that made the cat-33 regex table the only written statement of
// what any MCP response contains.
//
// Docs: docs/features/feature/quality-gates/index.md
package responsecontract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ContractPath is the declared contract, relative to the repository root.
const ContractPath = ".mcp-response-contract.json"

// toolsListGolden is the shipped tool document. It is the ONLY authority on
// which modes exist, so the ratchet cannot be gamed by editing a mode list.
const toolsListGolden = "cmd/browser-agent/testdata/mcp-tools-list.golden.json"

// EnvelopeQueued is the contract key for the async lifecycle envelope a
// browser-mediated mode answers with before the browser has replied. It is not
// a mode, so it is declared in its own table and excluded from the mode ratchet.
const EnvelopeQueued = "async_queued"

const contractComment = "Declared MCP tool response shapes: field paths and JSON types, never values. " +
	"GENERATED from real in-process responses — do not hand-edit. Regenerate with `make response-contract-update`. " +
	"undeclared_baseline is a ratchet: it must equal the real count of shipped modes with no declared " +
	"shape EXACTLY, so a new undeclared mode fails and an improvement must be locked in."

// Contract is the checked-in declaration of every pinned response shape.
type Contract struct {
	Comment string `json:"_comment"`
	Version int    `json:"version"`
	// UndeclaredBaseline is how many shipped modes have no declared shape.
	UndeclaredBaseline int `json:"undeclared_baseline"`
	// Envelopes declares the response wrappers that are not themselves modes.
	Envelopes map[string]Shape `json:"envelopes"`
	// Modes maps "tool/mode" to its declared response shape.
	Modes map[string]Shape `json:"modes"`
}

// RepoRoot walks up from dir until it finds the module root.
func RepoRoot(dir string) (string, error) {
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}
	return "", fmt.Errorf("no go.mod above the working directory; the contract path cannot be resolved")
}

// Load reads the declared contract from the repository root.
func Load(root string) (*Contract, error) {
	raw, err := os.ReadFile(filepath.Join(root, ContractPath))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ContractPath, err)
	}
	var contract Contract
	if err := json.Unmarshal(raw, &contract); err != nil {
		return nil, fmt.Errorf("decode %s: %w", ContractPath, err)
	}
	if contract.Version != 1 || contract.Modes == nil {
		return nil, fmt.Errorf("%s is not a version 1 contract with a modes table", ContractPath)
	}
	return &contract, nil
}

// Save freezes the contract, deterministically, at the repository root.
func Save(root string, contract *Contract) error {
	contract.Comment = contractComment
	contract.Version = 1
	encoded, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, ContractPath), append(encoded, '\n'), 0o644)
}

// ShippedModes reads every "tool/mode" out of the shipped tool document.
func ShippedModes(root string) (map[string]bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, toolsListGolden))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", toolsListGolden, err)
	}
	var tools []struct {
		Name        string `json:"name"`
		InputSchema struct {
			Properties struct {
				What struct {
					Enum []string `json:"enum"`
				} `json:"what"`
			} `json:"properties"`
		} `json:"inputSchema"` // SPEC:MCP — camelCase required by MCP protocol
	}
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil, fmt.Errorf("decode %s: %w", toolsListGolden, err)
	}
	modes := map[string]bool{}
	for _, tool := range tools {
		for _, mode := range tool.InputSchema.Properties.What.Enum {
			modes[tool.Name+"/"+mode] = true
		}
	}
	if len(modes) == 0 {
		return nil, fmt.Errorf("%s exposed no modes; every count derived from it would be zero", toolsListGolden)
	}
	return modes, nil
}

// Undeclared lists the shipped modes with no declared response shape.
func Undeclared(shipped map[string]bool, contract *Contract) []string {
	missing := make([]string, 0, len(shipped))
	for mode := range shipped {
		if _, declared := contract.Modes[mode]; !declared {
			missing = append(missing, mode)
		}
	}
	sort.Strings(missing)
	return missing
}

// staleModes lists declared modes the shipped tool document no longer exposes. A
// stale entry is worse than a missing one: it counts as coverage that does not
// exist and holds the undeclared baseline artificially low.
func staleModes(shipped map[string]bool, contract *Contract) []string {
	stale := make([]string, 0)
	for mode := range contract.Modes {
		if !shipped[mode] {
			stale = append(stale, mode)
		}
	}
	sort.Strings(stale)
	return stale
}

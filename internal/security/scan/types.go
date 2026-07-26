// types.go — Finding, scan input/result and the scanner handle.
// Purpose: Implements aggregate security scanning across captured network/log evidence.
// Why: Centralizes security checks so risk findings are produced with one coherent severity model.
// Docs: docs/features/feature/security-hardening/index.md

package scan

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type Finding struct {
	Check       string `json:"check"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Evidence    string `json:"evidence"`
	Remediation string `json:"remediation"`
}

type LogEntry = types.LogEntry

type Input struct {
	NetworkBodies    []capture.NetworkBody
	WaterfallEntries []capture.NetworkWaterfallEntry
	ConsoleEntries   []LogEntry
	PageURLs         []string
	URLFilter        string
	Checks           []string
	SeverityMin      string
}

type Result struct {
	Findings  []Finding `json:"findings"`
	Summary   Summary   `json:"summary"`
	ScannedAt time.Time `json:"scanned_at"`
}

type Summary struct {
	TotalFindings int            `json:"total_findings"`
	BySeverity    map[string]int `json:"by_severity"`
	ByCheck       map[string]int `json:"by_check"`
	URLsScanned   int            `json:"urls_scanned"`
}

type Scanner struct {
	mu sync.RWMutex
}

var defaultChecks = []string{"credentials", "pii", "headers", "cookies", "transport", "auth", "network"}

func NewScanner() *Scanner {
	return &Scanner{}
}

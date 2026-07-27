// doctor_fastpath_telemetry.go — Implements fast-path telemetry diagnostics for setup doctor checks.
// Why: Keeps telemetry parsing and threshold policy separate from user-facing setup orchestration.

package health

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
)

// FastPathTelemetrySummary holds aggregated fast-path telemetry stats.
type FastPathTelemetrySummary struct {
	Total      int
	Success    int
	Failure    int
	ErrorCodes map[int]int
	Methods    map[string]int
}

// SummarizeFastPathTelemetryLog parses the telemetry log and returns aggregated stats.
func SummarizeFastPathTelemetryLog(path string, maxLines int) FastPathTelemetrySummary {
	summary := FastPathTelemetrySummary{
		ErrorCodes: map[int]int{},
		Methods:    map[string]int{},
	}
	if maxLines <= 0 {
		return summary
	}

	// #nosec G304 -- path is deterministic under runtime state dir.
	f, err := os.Open(path)
	if err != nil {
		return summary
	}
	defer func() { _ = f.Close() }()

	lines := make([]string, 0, maxLines)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) > maxLines {
			lines = lines[1:]
		}
	}

	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		event, _ := entry["event"].(string)
		if event != "bridge_fastpath_method" {
			continue
		}
		summary.Total++
		if ok, _ := entry["success"].(bool); ok {
			summary.Success++
		} else {
			summary.Failure++
		}
		if method, _ := entry["method"].(string); method != "" {
			summary.Methods[method]++
		}
		if code, ok := entry["error_code"].(float64); ok {
			codeInt := int(code)
			if codeInt != 0 {
				summary.ErrorCodes[codeInt]++
			}
		}
	}
	return summary
}

// EvaluateFastPathFailureThreshold checks if the failure ratio exceeds the given threshold.
func EvaluateFastPathFailureThreshold(summary FastPathTelemetrySummary, minSamples int, maxFailureRatio float64) error {
	if maxFailureRatio < 0 {
		return nil
	}
	if maxFailureRatio > 1 {
		return fmt.Errorf("max failure ratio must be <= 1.0")
	}
	if minSamples < 1 {
		return fmt.Errorf("min samples must be >= 1")
	}
	if summary.Total < minSamples {
		return fmt.Errorf("insufficient samples: got %d, need %d", summary.Total, minSamples)
	}
	ratio := float64(summary.Failure) / float64(summary.Total)
	if ratio > maxFailureRatio {
		return fmt.Errorf("failure ratio %.4f exceeds threshold %.4f (%d/%d failures)", ratio, maxFailureRatio, summary.Failure, summary.Total)
	}
	return nil
}

// PrintFastPathTelemetryDiagnostics prints fast-path telemetry stats to stdout.
func PrintFastPathTelemetryDiagnostics(maxLines int, logPathFn func() (string, error)) (FastPathTelemetrySummary, bool) {
	diag.Print("Checking bridge fast-path telemetry... ")
	path, err := logPathFn()
	if err != nil {
		diag.Println("FAILED")
		diag.Printf("  Cannot resolve telemetry log path: %v\n", err)
		diag.Println()
		return FastPathTelemetrySummary{ErrorCodes: map[int]int{}, Methods: map[string]int{}}, false
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			diag.Println("OK")
			diag.Printf("  Telemetry log: %s\n", path)
			diag.Println("  Status: no fast-path telemetry recorded yet")
			diag.Println()
			return FastPathTelemetrySummary{ErrorCodes: map[int]int{}, Methods: map[string]int{}}, false
		}
		diag.Println("FAILED")
		diag.Printf("  Telemetry log read error: %v\n", statErr)
		diag.Println()
		return FastPathTelemetrySummary{ErrorCodes: map[int]int{}, Methods: map[string]int{}}, false
	}

	summary := SummarizeFastPathTelemetryLog(path, maxLines)
	diag.Println("OK")
	diag.Printf("  Telemetry log: %s\n", path)
	diag.Printf("  Last %d events: total=%d success=%d failure=%d\n", maxLines, summary.Total, summary.Success, summary.Failure)

	if len(summary.Methods) > 0 {
		methods := make([]string, 0, len(summary.Methods))
		for method := range summary.Methods {
			methods = append(methods, method)
		}
		sort.Strings(methods)
		parts := make([]string, 0, len(methods))
		for _, method := range methods {
			parts = append(parts, fmt.Sprintf("%s=%d", method, summary.Methods[method]))
		}
		diag.Printf("  Methods: %s\n", strings.Join(parts, ", "))
	}

	if len(summary.ErrorCodes) > 0 {
		codes := make([]int, 0, len(summary.ErrorCodes))
		for code := range summary.ErrorCodes {
			codes = append(codes, code)
		}
		sort.Ints(codes)
		parts := make([]string, 0, len(codes))
		for _, code := range codes {
			parts = append(parts, fmt.Sprintf("%d=%d", code, summary.ErrorCodes[code]))
		}
		diag.Printf("  Error codes: %s\n", strings.Join(parts, ", "))
	} else {
		diag.Println("  Error codes: none")
	}
	diag.Println()
	return summary, true
}

// Purpose: Converts axe-core accessibility violations into SARIF 2.1.0 reports for GitHub Code Scanning and CI tooling.
// Docs: docs/features/feature/sarif-export/index.md

// export_sarif.go — SARIF 2.1.0 accessibility report generation.
// Converts axe-core accessibility audit results into the Static Analysis
// Results Interchange Format, compatible with GitHub Code Scanning and
// other SARIF-consuming tools.
// Design: Each axe-core violation becomes a SARIF result with rule metadata,
// affected element locations, and remediation guidance.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
// SPEC:SARIF — SARIF 2.1.0 fields use camelCase per OASIS specification.
// SPEC:axe-core — axeViolation.HelpURL uses camelCase per axe-core library output.
//
// Layout:
// - export_sarif_types.go: SARIF and axe-core data models and constants
// - export_sarif_convert.go: rule/result conversion helpers
// - export_sarif_file.go: path validation and file persistence
// Package sarif converts accessibility findings into SARIF 2.1.0 reports.
package sarif

import (
	"encoding/json"
	"fmt"
)

// ExportSARIF converts an axe-core accessibility result to SARIF 2.1.0 format.
// If opts.SaveTo is set, it also writes the SARIF JSON to the specified file.
func ExportSARIF(a11yResultJSON json.RawMessage, opts SARIFExportOptions) (*SARIFLog, error) {
	var axe axeResult
	if err := json.Unmarshal(a11yResultJSON, &axe); err != nil {
		return nil, fmt.Errorf("failed to parse a11y result: %w", err)
	}

	toolVersion := opts.Version
	if toolVersion == "" {
		toolVersion = defaultToolVersion
	}
	log := &SARIFLog{
		Schema:  sarifSchemaURL,
		Version: sarifSpecVersion,
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "Kaboom Agentic Browser",
					Version:        toolVersion,
					InformationURI: "https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP",
					Rules:          []sarifRule{},
				},
			},
			Results: []sarifResult{},
		}},
	}

	run := &log.Runs[0]
	ruleIndices := make(map[string]int)

	convertViolationsToResults(run, ruleIndices, axe.Violations)
	if opts.IncludePasses {
		convertPassesToResults(run, ruleIndices, axe.Passes)
	}

	if opts.SaveTo != "" {
		if err := saveSARIFToFile(log, opts.SaveTo); err != nil {
			return nil, err
		}
	}
	return log, nil
}

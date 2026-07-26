// Purpose: Package heal — analyzes test files for selectors, repairs broken selectors, and runs batch healing.
// Why: Keeps selector healing self-contained so it evolves without touching test generation or classification.
// Docs: docs/features/feature/test-generation/index.md

/*
Package heal implements the generate {type: "test_heal"} action: scanning test
files for selectors, repairing broken ones with confidence scoring, and healing
a whole directory of test files in one batch.

Layout:
  - types.go: request/result types, error codes, and batch limits
  - paths.go: path/directory validation and resolution
  - selectors.go: selector extraction and validation
  - repair.go: selector healing and classification
  - batch.go: batch file walking and aggregate healing
  - summary.go: user-facing summary formatting

Key functions:
  - AnalyzeTestFile: extracts the selectors referenced by a single test file.
  - RepairSelectors: attempts to heal broken CSS selectors with confidence scoring.
  - HealTestBatch: heals selectors across every test file in a directory.

The package is self-contained: it depends on no other Kaboom package.
*/
package heal

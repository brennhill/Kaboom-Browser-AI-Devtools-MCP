// Purpose: Package reproduction — generates Playwright reproduction scripts from captured action timelines.
// Why: Turns observed failures into repeatable scripts for debugging and regression validation.
// Docs: docs/features/feature/reproduction-scripts/index.md

/*
Package reproduction generates Playwright test scripts from captured enhanced-action
timelines, enabling one-click reproduction of observed browser issues.

Key types:
  - Params: parsed arguments for generate({format: "reproduction"}).
  - Script: the generated reproduction script with metadata.

Key functions:
  - Generate: converts enhanced actions into a Playwright script with optional screenshots and error context.
  - FormatAction: converts a single EnhancedAction into a Playwright action line.
*/
package reproduction

// Purpose: Package imagediff — pure-Go pixel diffing for the analyze tool's visual regression modes.
// Why: Isolates screenshot comparison from analyze's argument parsing so the pixel
// pipeline can be exercised without constructing MCP requests or tool dependencies.
// Docs: docs/features/feature/analyze-tool/index.md

/*
Package imagediff compares two screenshots pixel by pixel and reports what changed.

Key types:
  - DiffResult: the full comparison outcome (percentage, counts, verdict, regions).
  - Region: a rectangular area of changed pixels.

Key functions:
  - CompareImages: loads two image files and returns a DiffResult.
  - RebuildChangedGrid: builds the per-pixel changed grid for two decoded images.
  - WriteDiffImage: renders the changed grid to a highlighted PNG.
  - LoadImage: decodes a PNG or JPEG from disk.

File layout:
  - imagediff.go: public types plus CompareImages and the DiffVerdict classifier.
  - grid.go: per-pixel change grid construction and counting.
  - regions.go: flood-fill grouping of changed pixels into rectangles.
  - imageio.go: the package's only filesystem access (decode in, PNG out).
*/
package imagediff

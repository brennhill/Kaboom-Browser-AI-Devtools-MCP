// Purpose: Package util — shared utility functions for binary detection, JSON responses, timestamps, URLs, and safe goroutines.
// Why: Centralizes cross-cutting helpers to avoid duplication across internal packages.

/*
Package util provides shared utility functions used across internal packages.

Key functions:
  - DetectBinaryFormat: detects binary payload formats (MessagePack, CBOR, Protobuf, BSON) via heuristics.
  - JSONResponse: writes a JSON HTTP response with proper content type and status code.
  - ParseTimestamp: parses RFC3339/RFC3339Nano timestamp strings.
  - ExtractURLPath: extracts the path component from a URL string.
  - SafeGo: launches a goroutine with panic recovery for background tasks.

File layout:
  - binary.go: format-detection entry point plus all four per-format detectors.
  - response.go: HTTP JSON response and method-guard helpers.
  - time.go: timestamp parsing and duration formatting.
  - strings.go: string and string-keyed-map helpers.
  - url.go, media.go, safego.go, proc_*.go: URL, media-file, goroutine and process helpers.
*/
package util

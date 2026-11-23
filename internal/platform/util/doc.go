// Purpose: Package util — small context-free helpers for text, URLs, media filenames, timestamps, and safe goroutines.
// Why: Centralizes leaf helpers with no domain owner. Domain-owned helpers live with their context
// (binary detection: observation/binaryformat; JSON HTTP responses: platform/httpx; detached processes: lifecycle/procctl).

/*
Package util provides small context-free helper functions.

Key functions:
  - ParseTimestamp / FormatDuration: timestamp parsing and duration formatting.
  - ExtractURLPath / ExtractOrigin: URL component helpers.
  - SanitizeForFilename / SplitDataURL / DecodeDataURL / BuildScreenshotFilename: media-file helpers.
  - Truncate / SortedMapKeys: string helpers.
  - SafeGo: launches a goroutine with panic recovery for background tasks.

File layout:
  - response.go: SafeGo and time helpers.
  - media.go: media-file, string, and URL helpers.
*/
package util

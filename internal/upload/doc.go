// Purpose: Package upload — multi-stage file upload with security validation, SSRF protection, and OS automation.
// Why: Enforces upload safety boundaries against path traversal, SSRF, and injection attacks across all stages.
// Docs: docs/features/feature/file-upload/index.md

/*
Package upload implements the four-stage file upload pipeline with comprehensive
security validation at each stage.

Stages:
  - Stage 1 (File Read): validates path, reads file, returns base64-encoded content.
  - Stage 2 (Dialog Inject): validates path, queues file dialog injection.
  - Stage 3 (Form Submit): streams multipart form submission with SSRF-safe transport.
  - Stage 4 (OS Automation): injects file path into native dialogs via AppleScript/xdotool/SendKeys.

This package owns the wire vocabulary shared by every stage (the request and
response types, MIME detection, progress tiers, size caps) plus the Stage 1 and
Stage 2 handlers. The rest lives in subpackages that depend on it:

  - uploadsec: the safety kernel — path validation, denylist, SSRF, input sanitizers.
  - formsubmit: Stage 3, multipart streaming form submission.
  - osauto: Stage 4, platform-specific native dialog automation.

Key types:
  - StageResponse: generic response for all upload stage operations.
  - FileReadRequest / FileReadResponse: Stage 1 wire contract.

Key functions:
  - HandleFileRead: Stage 1 handler.
  - HandleDialogInject: Stage 2 handler.
*/
package upload

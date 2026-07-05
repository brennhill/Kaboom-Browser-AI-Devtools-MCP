---
doc_type: tech-spec
feature_id: feature-file-upload
status: shipped
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# File Upload Tech Spec

## Architecture
- Interact upload sub-handler: `cmd/browser-agent/tools_interact_upload_handler.go`
- Upload action implementation: `cmd/browser-agent/tools_interact_upload.go`
- Validation/security layer: `internal/upload/security*.go`, `internal/upload/validators.go`
- OS automation: `internal/upload/os_automation*.go`
- Form submit follow-up: `internal/upload/form_submit*.go`

## Constraints
- Enforce extension/server boundary checks before invoking OS automation.
- Apply platform-specific upload/dismiss strategies by OS implementation.
- Maintain SSRF and path traversal protections in all upload flows.

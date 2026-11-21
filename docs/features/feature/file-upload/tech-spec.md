---
doc_type: tech-spec
feature_id: feature-file-upload
status: shipped
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# File Upload Tech Spec

## Architecture
- Interact action registration: `upload` owner in `cmd/browser-agent/internal/toolinteract/action_owners.go`
- Upload action handler: `cmd/browser-agent/internal/toolinteract/interactupload/upload.go` (`Handler.HandleUpload` validates params and the file path, then `queueUpload` queues the request for the extension's 4-stage escalation: extension inject, form submit, direct API, OS automation)
- Validation/security layer: `internal/upload/uploadsec/` (`input.go`, `path.go`, `denylist.go`, `ssrf.go`, hardlink checks)
- Upload HTTP endpoints: `internal/upload/handlers.go`, `internal/upload/httpapi/handlers.go`
- OS automation: `internal/upload/osauto/` (`dialog.go`, `inject.go`, platform-specific `exec_*_impl.go`)
- Form submit follow-up: `internal/upload/form_submit*.go`

## Constraints
- Enforce extension/server boundary checks before invoking OS automation.
- Apply platform-specific upload/dismiss strategies by OS implementation.
- Maintain SSRF and path traversal protections in all upload flows.

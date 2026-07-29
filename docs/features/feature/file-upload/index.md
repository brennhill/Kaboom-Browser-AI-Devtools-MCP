---
doc_type: feature_index
feature_id: feature-file-upload
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-29
code_paths:
  - cmd/browser-agent/internal/toolinteract/interactupload/upload.go
  - cmd/browser-agent/internal/toolinteract/action_owners.go
  - cmd/browser-agent/internal/toolguard/guards.go
  - cmd/browser-agent/server.go
  - cmd/browser-agent/config.go
  - cmd/browser-agent/tools_core.go
  - internal/upload/httpapi/handlers.go
  - internal/upload/handlers.go
  - internal/upload/form_submit.go
  - internal/upload/types.go
  - internal/upload/uploadsec/path.go
  - internal/upload/uploadsec/denylist.go
  - internal/upload/uploadsec/ssrf.go
  - internal/upload/uploadsec/input.go
  - internal/upload/osauto/inject.go
  - internal/upload/osauto/pid.go
  - scripts/smoke-tests/upload-server.py
test_paths:
  - cmd/browser-agent/internal/toolinteract/interactupload/upload_test.go
  - internal/upload/httpapi/contracts_test.go
  - internal/upload/httpapi/handlers_test.go
  - cmd/browser-agent/upload_integration_test.go
  - cmd/browser-agent/upload_handlers_test.go
  - internal/upload/handlers_test.go
  - internal/upload/form_submit_writer_test.go
  - internal/upload/uploadsec/path_test.go
  - internal/upload/uploadsec/ssrf_test.go
  - internal/upload/osauto/osauto_test.go
  - scripts/smoke-tests/test-upload-server.py
  - scripts/smoke-tests/15-file-upload.sh
  - scripts/tests/browser/cat-24-upload.sh
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# File Upload

> **2026-07-27:** Fully migrated upload callers and tests to the canonical
> `internal/upload`, `internal/upload/httpapi`, `internal/upload/osauto`, and
> `internal/upload/uploadsec` owners. The root aliases and the
> `cmd/browser-agent/internal/uploadhandler` compatibility package were deleted.

## TL;DR
- Status: shipped
- Tool: `interact`
- Action: `upload`

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Canonical Note
Upload is security-first: path validation and policy checks must pass before any OS-level dialog automation runs.
The local Stage 3 upload fixture accepts both fixed-length and chunked multipart
requests so its CSRF verification matches the streaming production client.
Multipart writer panics are recovered at the goroutine boundary and returned
through the existing writer-error channel, so callers receive a failed
submission instead of a process crash or silent partial request.

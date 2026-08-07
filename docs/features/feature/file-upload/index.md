---
doc_type: feature_index
feature_id: feature-file-upload
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - cmd/browser-agent/internal/toolinteract/interactupload/upload.go
  - cmd/browser-agent/internal/toolinteract/action_owners.go
  - cmd/browser-agent/internal/toolguard/guards.go
  - cmd/browser-agent/server.go
  - cmd/browser-agent/config.go
  - cmd/browser-agent/internal/startupconfig/paths.go
  - cmd/browser-agent/internal/startupconfig/runtime.go
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
  - scripts/smoke-tests/upload/upload-server.py
test_paths:
  - cmd/browser-agent/internal/startupconfig/paths_test.go
  - cmd/browser-agent/internal/startupconfig/runtime_test.go
  - internal/upload/uploadsec/injectiontests/injection_test.go
  - scripts/contracts/smokeupload/contracts_test.go
  - internal/upload/osauto/pid_test.go
  - cmd/browser-agent/internal/toolinteract/interactupload/upload_test.go
  - internal/upload/httpapi/contracts_test.go
  - internal/upload/httpapi/handlers_test.go
  - internal/upload/httpapi/handler_instances_test.go
  - cmd/browser-agent/upload_integration_test.go
  - internal/upload/behavior_test.go
  - internal/upload/handlers_test.go
  - internal/upload/form_submit_writer_test.go
  - internal/upload/uploadsec/path_test.go
  - internal/upload/uploadsec/ssrf_test.go
  - internal/upload/osauto/osauto_test.go
  - scripts/smoke-tests/upload/test-upload-server.py
  - scripts/smoke-tests/upload/15-file-upload.sh
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
Startup builds the upload boundary inside the deterministic `startupconfig`
owner; root process composition receives only a validated immutable result.
The local Stage 3 upload fixture accepts both fixed-length and chunked multipart
requests so its CSRF verification matches the streaming production client.
Permission-path integration coverage uses a local SSRF-enabled form target and
asserts that no request arrives, keeping the regression independent of external
DNS while exercising production URL validation.
Multipart writer panics are recovered at the goroutine boundary and returned
through the existing writer-error channel, so callers receive a failed
submission instead of a process crash or silent partial request.
Stage 3 owns its HTTP client privately and injects it only at the internal
execution seam used by deterministic transport tests. If a transport rejects
before consuming the streaming body, the pipe reader closes with that error
before the handler waits for its writer, guaranteeing the writer unblocks and
the request returns a visible failure instead of hanging.
The HTTP adapter passes the inbound request context through that same stage, so
client disconnects and server cancellation terminate the outbound upload and
its multipart writer instead of leaving ten-minute background work behind.
All five upload endpoints are methods on one immutable `httpapi.Handlers`
owner. Security policy, OS-automation enablement, response encoding, and stage
dependencies are fixed per instance; no mutable package globals or free-handler
facades remain. Tests can therefore run independently with isolated fakes.
File-read size boundaries use a private injectable limit seam, so the exact
production threshold is preserved while boundary tests operate on tiny files
instead of allocating and encoding 100 MB fixtures. Core upload, HTTP adapter,
security, and interact response contracts live with their respective owners;
root tests retain only true installed-server integration coverage.
OS-dialog dismissal status mapping is tested through injected stage functions
in `upload/httpapi`; unit tests never invoke host `osascript`, `xdotool`, or
PowerShell commands and therefore produce deterministic results on every CI OS.
Installed upload integration assertions decode MCP payloads through the shared
browser-agent result helper; the deleted observe-contract fixture owns no test
parsing surface.

---
doc_type: product-spec
feature_id: feature-file-upload
status: shipped
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# File Upload Product Spec

## Purpose
Provide deterministic file upload automation for browser workflows, including native picker-driven flows and direct input assignment paths.

## Requirements
- `UPLOAD_PROD_001`: Accept absolute local file paths only.
- `UPLOAD_PROD_002`: Reject unsafe or policy-denied paths.
- `UPLOAD_PROD_003`: Support optional submit behavior after successful upload.
- `UPLOAD_PROD_004`: Return structured errors for OS automation and validation failures.

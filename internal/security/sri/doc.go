// doc.go — Package documentation for Subresource Integrity hash generation.
// Purpose: Generates Subresource Integrity hashes and related metadata from observed script/style resources.
// Why: Enables integrity pinning workflows that reduce third-party tampering and supply-chain risk.
// Docs: docs/features/feature/security-hardening/index.md

// Package sri computes Subresource Integrity hashes for the third-party scripts
// and stylesheets a page actually loaded, so they can be pinned against tampering.
//
// Layout:
//   - types.go:    input/output and internal pipeline models
//   - generate.go: filtering and generation pipeline
//   - helpers.go:  hashing/content-type/tag helpers
//   - tooling.go:  MCP/tool adapter
//
// sri is a leaf: it depends only on capture and util, never on a sibling
// security package.
package sri

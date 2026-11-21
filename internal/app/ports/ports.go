// ports.go — Consumer-neutral ports shared by adapters that hand results to the browser-command owner.
// Why: mediaapi and screenrec both accept extension uploads that complete a command. The port cannot
// live in internal/browsercommand (the layering ownership rule rejects producer-owned interfaces) nor in
// internal/queries (queries cannot import browsercommand without a cycle), so it is declared once here.
// Docs: docs/features/feature/query-service/index.md

// Package ports declares small consumer interfaces shared across adapters.
// It must not import internal/mcp, internal/capture/**, net/http or cmd/**
// (enforced by scripts/contracts/layering as part of internal/app/**).
package ports

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand"

// ResultSink accepts a command result that did not travel in a /sync
// exchange. internal/browsercommand.Owner satisfies it.
type ResultSink interface {
	SubmitResult(result browsercommand.Result) browsercommand.Outcome
}

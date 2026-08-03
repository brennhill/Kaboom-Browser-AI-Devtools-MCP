// handler.go — Defines and evaluates versioned QA verification contracts.
// Docs: docs/features/feature/verification-contracts/index.md

package verificationhandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/verification"
)

type params struct {
	Operation     string                          `json:"operation"`
	Contract      verification.Contract           `json:"contract"`
	Results       []verification.AssertionResult  `json:"results,omitempty"`
	Evidence      []evidenceSubmission            `json:"evidence,omitempty"`
	Catalog       []verification.EvidenceArtifact `json:"evidence_catalog,omitempty"`
	MaxAgeSeconds int                             `json:"max_age_seconds,omitempty"`
}

type evidenceSubmission struct {
	AssertionID string `json:"assertion_id"`
	verification.EvidenceInput
}

func Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var parsed params
	if response, failed := mcp.ParseArgs(req, args, &parsed); failed {
		return response
	}
	if parsed.Operation != "define" && parsed.Operation != "evaluate" {
		return mcp.Fail(req, mcp.ErrInvalidParam, "operation must be 'define' or 'evaluate'", "Choose define to validate a contract or evaluate to calculate its verdict", mcp.WithParam("operation"))
	}
	if err := verification.ValidateContract(parsed.Contract); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid verification contract: "+err.Error(), "Supply schema_version '1', a contract_id, and complete unique assertions", mcp.WithParam("contract"))
	}
	if parsed.Operation == "define" {
		return mcp.Succeed(req, "Verification contract defined", map[string]any{
			"status":   "defined",
			"contract": parsed.Contract,
		})
	}
	results, catalog, err := bindEvidence(parsed.Results, parsed.Evidence, parsed.Catalog)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid verification evidence: "+err.Error(), "Supply five-tool provenance, correlation_id, captured_at, and compact content", mcp.WithParam("evidence"))
	}
	evidenceDir, err := state.EvidenceDir()
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal, "Cannot resolve local evidence storage: "+err.Error(), "Check the Kaboom state directory and retry")
	}
	store := verification.Store{Dir: evidenceDir}
	catalog, err = persistAndResolveEvidence(store, results, catalog)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal, "Cannot persist local verification evidence: "+err.Error(), "Check state-directory permissions and artifact integrity, then retry")
	}
	maxAge := 24 * time.Hour
	if parsed.MaxAgeSeconds > 0 {
		maxAge = time.Duration(parsed.MaxAgeSeconds) * time.Second
	}
	result, err := verification.Evaluate(parsed.Contract, results, catalog, time.Now().UTC(), maxAge)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid verification results: "+err.Error(), "Use one valid result per contract assertion", mcp.WithParam("results"))
	}
	return mcp.Succeed(req, "Verification contract evaluated: "+string(result.Verdict), map[string]any{
		"result": result, "evidence_catalog": catalog,
	})
}

func persistAndResolveEvidence(store verification.Store, results []verification.AssertionResult, catalog []verification.EvidenceArtifact) ([]verification.EvidenceArtifact, error) {
	known := make(map[string]bool, len(catalog))
	for _, artifact := range catalog {
		if err := store.Save(artifact); err != nil {
			return nil, err
		}
		known[artifact.Ref.ID] = true
	}
	resolved := append([]verification.EvidenceArtifact(nil), catalog...)
	for _, result := range results {
		for _, ref := range result.Evidence {
			if known[ref.ID] {
				continue
			}
			artifact, err := store.Load(ref.ID)
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, verification.ErrInvalidEvidenceID) {
				// EXPECTED_ABSENCE: a caller may reference missing, removed, or malformed evidence; evaluation must report UNVERIFIED.
				continue
			}
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, artifact)
			known[ref.ID] = true
		}
	}
	return resolved, nil
}

func bindEvidence(results []verification.AssertionResult, submissions []evidenceSubmission, catalog []verification.EvidenceArtifact) ([]verification.AssertionResult, []verification.EvidenceArtifact, error) {
	bound := append([]verification.AssertionResult(nil), results...)
	resultIndex := make(map[string]int, len(bound))
	for index, result := range bound {
		resultIndex[result.AssertionID] = index
	}
	artifacts := append([]verification.EvidenceArtifact(nil), catalog...)
	for _, submission := range submissions {
		index, exists := resultIndex[submission.AssertionID]
		if !exists {
			return nil, nil, fmt.Errorf("evidence names unknown assertion_id %q", submission.AssertionID)
		}
		artifact, err := verification.BuildEvidence(submission.EvidenceInput)
		if err != nil {
			return nil, nil, fmt.Errorf("assertion %q: %w", submission.AssertionID, err)
		}
		artifacts = append(artifacts, artifact)
		bound[index].Evidence = append(bound[index].Evidence, artifact.Ref)
	}
	return bound, artifacts, nil
}

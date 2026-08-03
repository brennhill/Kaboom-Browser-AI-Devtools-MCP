// evidence.go — Redacted, content-addressed QA evidence artifacts.

package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/redaction"
)

const (
	EvidenceSchemaVersion = "1"
	MaxEvidenceBytes      = 64 * 1024
)

type EvidenceInput struct {
	Kind          string         `json:"kind"`
	Tool          string         `json:"tool"`
	Action        string         `json:"action"`
	CorrelationID string         `json:"correlation_id"`
	CapturedAt    time.Time      `json:"captured_at"`
	Content       map[string]any `json:"content"`
}

type EvidenceArtifact struct {
	SchemaVersion string          `json:"schema_version"`
	Ref           EvidenceRef     `json:"ref"`
	Tool          string          `json:"tool"`
	Action        string          `json:"action"`
	Content       json.RawMessage `json:"content"`
}

type evidenceDigest struct {
	SchemaVersion string          `json:"schema_version"`
	Kind          string          `json:"kind"`
	Tool          string          `json:"tool"`
	Action        string          `json:"action"`
	CorrelationID string          `json:"correlation_id"`
	CapturedAt    time.Time       `json:"captured_at"`
	Content       json.RawMessage `json:"content"`
}

func BuildEvidence(input EvidenceInput) (EvidenceArtifact, error) {
	if strings.TrimSpace(input.Kind) == "" || strings.TrimSpace(input.Action) == "" {
		return EvidenceArtifact{}, fmt.Errorf("kind and action are required")
	}
	if !validTool(input.Tool) {
		return EvidenceArtifact{}, fmt.Errorf("tool must be observe, generate, configure, interact, or analyze")
	}
	if strings.TrimSpace(input.CorrelationID) == "" || input.CapturedAt.IsZero() {
		return EvidenceArtifact{}, fmt.Errorf("correlation_id and captured_at are required")
	}
	redacted := redaction.NewRedactionEngine("").RedactMapValues(input.Content)
	content, err := json.Marshal(redacted)
	if err != nil {
		return EvidenceArtifact{}, fmt.Errorf("encode redacted evidence: %w", err)
	}
	if len(content) > MaxEvidenceBytes {
		return EvidenceArtifact{}, fmt.Errorf("evidence exceeds %d-byte compact artifact limit", MaxEvidenceBytes)
	}
	digestInput := evidenceDigest{
		SchemaVersion: EvidenceSchemaVersion, Kind: input.Kind, Tool: input.Tool,
		Action: input.Action, CorrelationID: input.CorrelationID,
		CapturedAt: input.CapturedAt.UTC(), Content: content,
	}
	id, err := digestEvidence(digestInput)
	if err != nil {
		return EvidenceArtifact{}, err
	}
	return EvidenceArtifact{
		SchemaVersion: EvidenceSchemaVersion,
		Ref:           EvidenceRef{ID: id, Kind: input.Kind, CorrelationID: input.CorrelationID, CapturedAt: input.CapturedAt.UTC()},
		Tool:          input.Tool, Action: input.Action, Content: content,
	}, nil
}

func evidenceIsAuthentic(artifact EvidenceArtifact) bool {
	if artifact.SchemaVersion != EvidenceSchemaVersion {
		return false
	}
	id, err := digestEvidence(evidenceDigest{
		SchemaVersion: artifact.SchemaVersion, Kind: artifact.Ref.Kind,
		Tool: artifact.Tool, Action: artifact.Action,
		CorrelationID: artifact.Ref.CorrelationID, CapturedAt: artifact.Ref.CapturedAt.UTC(),
		Content: artifact.Content,
	})
	return err == nil && id == artifact.Ref.ID
}

func digestEvidence(input evidenceDigest) (string, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("encode evidence digest: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func validTool(tool string) bool {
	switch tool {
	case "observe", "generate", "configure", "interact", "analyze":
		return true
	default:
		return false
	}
}

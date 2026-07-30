// Purpose: Guard configure clear behavior for annotation session/detail state.
// Why: Prevent stale annotation replay after configure(what:"clear", buffer:"all").
// Docs: docs/features/feature/annotated-screenshots/index.md

package main

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
)

func TestToolsConfigureClear_AllBuffers_ClearsAnnotationState(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	now := time.Now().UnixMilli()
	h.annotationStore.StoreSession(7, &annotation.Session{
		TabID:     7,
		Timestamp: now,
		PageURL:   "https://example.com/current",
		Annotations: []annotation.Annotation{
			{
				ID:            "ann_old",
				Text:          "old note",
				Timestamp:     now,
				CorrelationID: "detail_old",
			},
		},
	})
	h.annotationStore.AppendToNamedSession("qa-review", &annotation.Session{
		TabID:     7,
		Timestamp: now,
		PageURL:   "https://example.com/current",
		Annotations: []annotation.Annotation{
			{
				ID:            "ann_named",
				Text:          "named stale note",
				Timestamp:     now,
				CorrelationID: "detail_named",
			},
		},
	})
	h.annotationStore.StoreDetail("detail_old", annotation.Detail{
		CorrelationID: "detail_old",
		Selector:      "#old-node",
		Tag:           "div",
	})

	clearResp := callConfigureRaw(h, `{"what":"clear","buffer":"all"}`)
	clearResult := parseToolResult(t, clearResp)
	if clearResult.IsError {
		t.Fatalf("clear all should succeed, got: %s", firstText(clearResult))
	}

	anonResp := callAnalyzeRaw(h, `{"what":"annotations","background":true}`)
	anonResult := parseToolResult(t, anonResp)
	if anonResult.IsError {
		t.Fatalf("analyze annotations should not error after clear, got: %s", firstText(anonResult))
	}
	anonData := extractResultJSON(t, anonResult)
	if got := int(anonData["count"].(float64)); got != 0 {
		t.Fatalf("anonymous annotations count after clear = %d, want 0", got)
	}

	namedResp := callAnalyzeRaw(h, `{"what":"annotations","annot_session":"qa-review","background":true}`)
	namedResult := parseToolResult(t, namedResp)
	if namedResult.IsError {
		t.Fatalf("analyze named annotations should not error after clear, got: %s", firstText(namedResult))
	}
	namedData := extractResultJSON(t, namedResult)
	if got := int(namedData["total_count"].(float64)); got != 0 {
		t.Fatalf("named annotations total_count after clear = %d, want 0", got)
	}

	detailResp := callAnalyzeRaw(h, `{"what":"annotation_detail","correlation_id":"detail_old"}`)
	detailResult := parseToolResult(t, detailResp)
	if !detailResult.IsError {
		t.Fatalf("annotation_detail should be missing after clear, got success: %s", firstText(detailResult))
	}
}

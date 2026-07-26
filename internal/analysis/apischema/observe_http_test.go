// Purpose: Branch-coverage tests for HTTP observation recording and endpoint counting.
// Docs: docs/features/feature/api-schema/index.md

package apischema

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// recordStatusOnly — covers zero status, new status, existing status, cap
// ============================================

func TestRecordStatusOnly(t *testing.T) {
	t.Run("zero status is ignored", func(t *testing.T) {
		s := NewSchemaStore()
		acc := &endpointAccumulator{}
		s.recordStatusOnly(acc, 0)
		if acc.responseShapes != nil {
			t.Error("expected nil responseShapes for zero status")
		}
	})

	t.Run("negative status is ignored", func(t *testing.T) {
		s := NewSchemaStore()
		acc := &endpointAccumulator{}
		s.recordStatusOnly(acc, -1)
		if acc.responseShapes != nil {
			t.Error("expected nil responseShapes for negative status")
		}
	})

	t.Run("new status creates shape", func(t *testing.T) {
		s := NewSchemaStore()
		acc := &endpointAccumulator{}
		s.recordStatusOnly(acc, 200)
		if acc.responseShapes == nil {
			t.Fatal("expected responseShapes to be initialized")
		}
		if acc.responseShapes[200] == nil {
			t.Fatal("expected shape for status 200")
		}
		if acc.responseShapes[200].count != 1 {
			t.Errorf("count = %d, want 1", acc.responseShapes[200].count)
		}
	})

	t.Run("existing status increments count", func(t *testing.T) {
		s := NewSchemaStore()
		acc := &endpointAccumulator{}
		s.recordStatusOnly(acc, 200)
		s.recordStatusOnly(acc, 200)
		s.recordStatusOnly(acc, 200)
		if acc.responseShapes[200].count != 3 {
			t.Errorf("count = %d, want 3", acc.responseShapes[200].count)
		}
	})

	t.Run("cap prevents new shapes beyond limit", func(t *testing.T) {
		s := NewSchemaStore()
		acc := &endpointAccumulator{}
		// Fill to maxResponseShapes
		for i := 1; i <= maxResponseShapes; i++ {
			s.recordStatusOnly(acc, i)
		}
		beforeCount := len(acc.responseShapes)
		// Try adding one more
		s.recordStatusOnly(acc, maxResponseShapes+1)
		if len(acc.responseShapes) != beforeCount {
			t.Errorf("expected cap at %d, got %d", beforeCount, len(acc.responseShapes))
		}
	})
}

// ============================================
// EndpointCount
// ============================================

func TestEndpointCount(t *testing.T) {
	s := NewSchemaStore()
	if s.EndpointCount() != 0 {
		t.Errorf("expected 0 endpoints, got %d", s.EndpointCount())
	}

	// Observe a body to create an endpoint
	s.Observe(capture.NetworkBody{
		Method:       "GET",
		URL:          "https://api.example.com/users",
		Status:       200,
		ResponseBody: `{"id": 1}`,
		ContentType:  "application/json",
	})

	if s.EndpointCount() != 1 {
		t.Errorf("expected 1 endpoint, got %d", s.EndpointCount())
	}
}

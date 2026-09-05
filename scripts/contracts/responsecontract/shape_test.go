// shape_test.go — Tests the shape derivation the whole contract rests on.
//
// If derivation is wrong, every gate built on it is wrong quietly: a deriver
// that returned an empty shape would make the drift gate pass on any response
// at all.
package responsecontract

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func request() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, Method: "tools/call"}
}

func TestShapeRecordsPathsAndTypesNotValues(t *testing.T) {
	t.Parallel()
	first := shapeOfPayload(map[string]any{"count": float64(2), "scope": "current_page"})
	second := shapeOfPayload(map[string]any{"count": float64(9999), "scope": "all"})

	if len(Diff("observe/errors", first, second)) != 0 {
		t.Fatalf("different values produced different shapes: %v vs %v", first, second)
	}
	if len(first.Fields) != 2 {
		t.Fatalf("fields = %v, want count and scope", first.Fields)
	}
}

func TestShapeDescendsIntoObjectsAndArrayElements(t *testing.T) {
	t.Parallel()
	shape := shapeOfPayload(map[string]any{
		"errors":   []any{map[string]any{"message": "boom", "line": float64(3)}},
		"metadata": map[string]any{"is_stale": false},
	})

	for _, path := range []string{"errors", "errors[]", "errors[].line", "errors[].message", "metadata", "metadata.is_stale"} {
		if !hasPath(shape, path) {
			t.Errorf("derived shape has no %q: %v", path, shape.Fields)
		}
	}
}

func TestArrayShapeDoesNotDependOnHowManyElementsTheFixtureHeld(t *testing.T) {
	t.Parallel()
	one := shapeOfPayload(map[string]any{"errors": []any{map[string]any{"message": "a"}}})
	three := shapeOfPayload(map[string]any{"errors": []any{
		map[string]any{"message": "a"}, map[string]any{"message": "b"}, map[string]any{"message": "c"}}})

	if drifts := Diff("observe/errors", one, three); len(drifts) != 0 {
		t.Fatalf("element count changed the shape: %v", Details(drifts))
	}
}

func TestEnvelopeIsRecognisedByItsLifecycleFields(t *testing.T) {
	t.Parallel()
	direct := shapeOfPayload(map[string]any{"count": float64(0)})
	if direct.Kind != kindDirect {
		t.Fatalf("kind = %q, want %q", direct.Kind, kindDirect)
	}
	envelope := shapeOfPayload(map[string]any{
		"correlation_id": "dom_1", "lifecycle_status": "queued", "result": map[string]any{"count": float64(1)}})
	if envelope.Kind != kindEnvelope {
		t.Fatalf("kind = %q, want %q", envelope.Kind, kindEnvelope)
	}
	if !hasPath(envelope, "result.count") {
		t.Fatalf("the nested payload was not descended into: %v", envelope.Fields)
	}
}

func TestPayloadIsTakenFromTheFirstLineThatBeginsWithABrace(t *testing.T) {
	t.Parallel()
	// analyze/draw_session's recovery hint embeds {what:'draw_history'} in its
	// prose. Slicing from the first brace anywhere parsed the hint as the
	// payload and declared a contract made of the hint's keys.
	payload, err := payloadOf("No drawing session. Try observe({what:'draw_history'}) instead.\n{\"sessions\":[]}")
	if err != nil {
		t.Fatal(err)
	}
	if _, present := payload["sessions"]; !present {
		t.Fatalf("payload = %v, want the JSON line rather than the prose brace", payload)
	}
}

func TestAResponseWithNoJSONPayloadIsRefusedNotDeclaredEmpty(t *testing.T) {
	t.Parallel()
	// FAIL LOUD. Declaring an empty shape for a prose-only response would pin a
	// contract that every future response satisfies.
	if _, err := ShapeOfResponse(mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion, ID: 1, Result: mcp.TextResponse("no data")}); err == nil {
		t.Fatal("a prose-only response was accepted as a declarable shape")
	}
}

func TestAnMCPErrorIsRefusedAsAContract(t *testing.T) {
	t.Parallel()
	// A browser-mediated mode with no extension degrades to an error whose body
	// looks like a legitimate payload. Freezing that would pin the degraded
	// shape as the contract forever.
	failed := mcp.Fail(request(), mcp.ErrNoData, "Extension is not connected", "Connect the extension.")
	if _, err := ShapeOfResponse(failed); err == nil {
		t.Fatal("an MCP error response was accepted as a declarable shape")
	}
}

func TestShapeOfARealSucceedResponseMatchesItsPayload(t *testing.T) {
	t.Parallel()
	// Derivation runs against the production response builder, not a hand-built
	// string, so a change to how Kaboom frames a response is caught here.
	response := mcp.Succeed(request(), "Browser errors", map[string]any{
		"count": 1, "errors": []any{map[string]any{"message": "boom"}}, "scope": "current_page"})
	shape, err := ShapeOfResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	if shape.Kind != kindDirect || !hasPath(shape, "errors[].message") || !hasPath(shape, "count") {
		t.Fatalf("shape = %+v", shape)
	}
}

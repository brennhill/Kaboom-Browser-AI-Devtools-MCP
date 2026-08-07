// command_test.go — Tests fixture command transport lifecycle and pressure behavior.

package transport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestExecuteRejectsCancellationDisconnectionAndPressure(t *testing.T) {
	t.Parallel()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Execute(cancelled, nil, "qa", nil, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled command error = %v", err)
	}
	disconnected := capture.NewCapture()
	t.Cleanup(disconnected.Close)
	if _, err := Execute(context.Background(), disconnected, "qa", nil, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("disconnected command error = %v", err)
	}
	connected := capture.NewCapture()
	t.Cleanup(connected.Close)
	capturefixture.Connect(connected)
	for index := 0; index < queries.MaxPendingQueries; index++ {
		if _, err := connected.Queries().CreatePendingQuery(queries.PendingQuery{Type: "occupied"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Execute(context.Background(), connected, "qa", nil, time.Second); err == nil {
		t.Fatal("saturated command queue accepted fixture command")
	}
}

func TestExecuteReturnsCorrelatedExtensionResult(t *testing.T) {
	t.Parallel()
	captureStore := capture.NewCapture()
	t.Cleanup(captureStore.Close)
	capturefixture.Connect(captureStore)
	completed := make(chan struct{})
	go func() {
		defer close(completed)
		captureStore.Queries().WaitForPendingQueries(time.Second)
		pending := captureStore.Queries().GetPendingQueries()
		if len(pending) > 0 {
			captureStore.Queries().SetQueryResultWithClient(pending[0].ID, json.RawMessage(`{"restored":true}`), "")
		}
	}()
	result, err := Execute(context.Background(), captureStore, "qa_restore", json.RawMessage(`{"fixture":"corrupt"}`), time.Second)
	<-completed
	if err != nil || string(result) != `{"restored":true}` {
		t.Fatalf("fixture result = %s, error = %v", result, err)
	}
}

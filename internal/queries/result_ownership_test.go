// result_ownership_test.go — Verifies query payloads never alias dispatcher-owned state.
package queries

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSetQueryResultDetachesCallerPayload(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, err := qd.CreatePendingQuery(PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	payload := json.RawMessage(`{"stable":true}`)
	qd.SetQueryResult(id, payload)
	payload[2] = 'X'

	got, found := qd.TakeQueryResult(id)
	if !found {
		t.Fatal("stored result was not found")
	}
	if string(got) != `{"stable":true}` {
		t.Fatalf("stored result = %s, want detached original payload", got)
	}
}

func TestPendingQuerySnapshotsDetachPayload(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	params := json.RawMessage(`{"selector":"main"}`)
	if _, err := qd.CreatePendingQuery(PendingQuery{Type: "dom", Params: params}); err != nil {
		t.Fatal(err)
	}
	params[2] = 'X'

	first := qd.GetPendingQueries()
	if len(first) != 1 || string(first[0].Params) != `{"selector":"main"}` {
		t.Fatalf("first snapshot = %+v, want detached original params", first)
	}
	first[0].Params[2] = 'Y'

	second := qd.GetPendingQueries()
	if len(second) != 1 || string(second[0].Params) != `{"selector":"main"}` {
		t.Fatalf("second snapshot = %+v, want dispatcher state unchanged", second)
	}
}

func TestCommandResultSnapshotsDetachPayload(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-detached", "query-detached", time.Minute)
	payload := json.RawMessage(`{"stable":true}`)
	qd.ApplyCommandResult("corr-detached", "complete", payload, "")
	payload[2] = 'X'

	first, found := qd.GetCommandResult("corr-detached")
	if !found {
		t.Fatal("command result was not found")
	}
	if string(first.Result) != `{"stable":true}` {
		t.Fatalf("stored command result = %s, want detached original payload", first.Result)
	}
	first.Result[2] = 'Y'

	second, found := qd.GetCommandResult("corr-detached")
	if !found {
		t.Fatal("command result disappeared after snapshot mutation")
	}
	if string(second.Result) != `{"stable":true}` {
		t.Fatalf("second command result = %s, want dispatcher state unchanged", second.Result)
	}
}

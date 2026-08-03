// handler_test.go — Characterizes the saved-sequence handler boundary.

package sequencehandler

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
)

type memoryStore map[string][]byte

func (s memoryStore) Save(namespace, key string, data []byte) error {
	s[namespace+"/"+key] = append([]byte(nil), data...)
	return nil
}

func TestHandlerReportsMalformedSequenceRecovery(t *testing.T) {
	t.Parallel()
	const private = "private-sequence-value"
	valid := []byte(`{"name":"broken","steps":[{"what":"click"}],"private":"private-sequence-value"}`)
	for _, kind := range []statefault.Kind{statefault.Corruption, statefault.PartialWrite} {
		t.Run(string(kind), func(t *testing.T) {
			store := memoryStore{"sequences/broken": statefault.New(kind, private).Payload(valid)}
			diagnostics := statediag.NewCollector()
			handler := New(Deps{Store: store, Diagnostics: diagnostics})
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

			response := handler.List(req, json.RawMessage(`{}`))
			if responseIsError(response) || !containsResult(response, `count\":0`) {
				t.Fatalf("List() response = %s, want empty successful fallback", response.Result)
			}
			got := diagnostics.Snapshot()
			if len(got) != 1 || got[0].Name != "saved_sequence_state" || got[0].Fix == "" {
				t.Fatalf("diagnostics = %#v, want actionable sequence warning", got)
			}
			if strings.Contains(got[0].Detail, private) {
				t.Fatalf("diagnostic leaked persisted sequence: %#v", got[0])
			}
		})
	}
}

func (s memoryStore) Load(namespace, key string) ([]byte, error) {
	data, ok := s[namespace+"/"+key]
	if !ok {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), data...), nil
}

func (s memoryStore) List(namespace string) ([]string, error) {
	var keys []string
	for key := range s {
		if len(key) > len(namespace)+1 && key[:len(namespace)+1] == namespace+"/" {
			keys = append(keys, key[len(namespace)+1:])
		}
	}
	return keys, nil
}

func (s memoryStore) Delete(namespace, key string) error {
	delete(s, namespace+"/"+key)
	return nil
}

func TestHandlerKeepsSequenceCRUDTogether(t *testing.T) {
	t.Parallel()
	store := memoryStore{}
	handler := New(Deps{Store: store})
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	saved := handler.Save(req, json.RawMessage(`{"name":"checkout","tags":["smoke"],"steps":[{"what":"click","selector":"#buy"}]}`))
	if responseIsError(saved) {
		t.Fatalf("save returned error: %s", saved.Result)
	}

	got := handler.Get(req, json.RawMessage(`{"name":"checkout"}`))
	if responseIsError(got) || !containsResult(got, `step_count\":1`) {
		t.Fatalf("get response = %s", got.Result)
	}

	listed := handler.List(req, json.RawMessage(`{"tags":["smoke"]}`))
	if responseIsError(listed) || !containsResult(listed, `count\":1`) {
		t.Fatalf("list response = %s", listed.Result)
	}

	deleted := handler.Delete(req, json.RawMessage(`{"name":"checkout"}`))
	if responseIsError(deleted) {
		t.Fatalf("delete returned error: %s", deleted.Result)
	}
}

func TestHandlerReloadsSavedSequenceAfterRestartGeneration(t *testing.T) {
	store := memoryStore{}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	first := New(Deps{Store: store})
	if responseIsError(first.Save(req, json.RawMessage(`{"name":"restart","steps":[{"what":"click"}]}`))) {
		t.Fatal("initial generation could not save sequence")
	}
	if statefault.New(statefault.Restart, "private").NextGeneration(7) != 8 {
		t.Fatal("restart fixture did not advance generation")
	}
	second := New(Deps{Store: store})
	response := second.Get(req, json.RawMessage(`{"name":"restart"}`))
	if responseIsError(response) || !containsResult(response, `name\":\"restart`) {
		t.Fatalf("new generation did not reload sequence: %s", response.Result)
	}
}

func TestHandlerReportsUnavailableStore(t *testing.T) {
	t.Parallel()
	handler := New(Deps{})
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := handler.List(req, json.RawMessage(`{}`))
	if !responseIsError(resp) || !containsResult(resp, mcp.ErrNotInitialized) {
		t.Fatalf("response = %s", resp.Result)
	}
}

func TestHandlerReportsCanonicalStoreFaultsWithoutPrivateState(t *testing.T) {
	const private = "private-sequence-value"
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	for _, testCase := range []struct {
		operation string
		kind      statefault.Kind
		invoke    func(*Handler) mcp.JSONRPCResponse
	}{
		{operation: "save", kind: statefault.Write, invoke: func(handler *Handler) mcp.JSONRPCResponse {
			return handler.Save(req, json.RawMessage(`{"name":"faulted","steps":[{"what":"click"}]}`))
		}},
		{operation: "load", kind: statefault.Read, invoke: func(handler *Handler) mcp.JSONRPCResponse {
			return handler.Get(req, json.RawMessage(`{"name":"faulted"}`))
		}},
		{operation: "list", kind: statefault.Read, invoke: func(handler *Handler) mcp.JSONRPCResponse {
			return handler.List(req, json.RawMessage(`{}`))
		}},
		{operation: "delete", kind: statefault.Write, invoke: func(handler *Handler) mcp.JSONRPCResponse {
			return handler.Delete(req, json.RawMessage(`{"name":"faulted"}`))
		}},
	} {
		t.Run(testCase.operation, func(t *testing.T) {
			collector := statediag.NewCollector()
			store := statefault.NewStore(
				memoryStore{"sequences/faulted": []byte(`{"name":"faulted","steps":[{"what":"click"}]}`)},
				statefault.New(testCase.kind, private),
			)
			response := testCase.invoke(New(Deps{Store: store, Diagnostics: collector}))
			if strings.Contains(string(response.Result), private) {
				t.Fatal("tool response leaked private persisted state")
			}
			diagnostics := collector.Snapshot()
			if len(diagnostics) != 1 || diagnostics[0].Name != "saved_sequence_state" || diagnostics[0].Fix == "" {
				t.Fatalf("diagnostics = %#v, want one actionable saved-sequence incident", diagnostics)
			}
			if strings.Contains(diagnostics[0].Detail, private) {
				t.Fatal("Doctor diagnostic leaked private persisted state")
			}
		})
	}
}

func containsResult(resp mcp.JSONRPCResponse, fragment string) bool {
	return resp.Result != nil && json.Valid(resp.Result) &&
		stringContains(string(resp.Result), fragment)
}

func stringContains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}

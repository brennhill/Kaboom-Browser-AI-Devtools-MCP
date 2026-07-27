// handler_test.go — Characterizes the saved-sequence handler boundary.

package sequencehandler

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type memoryStore map[string][]byte

func (s memoryStore) Save(namespace, key string, data []byte) error {
	s[namespace+"/"+key] = append([]byte(nil), data...)
	return nil
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

func TestHandlerReportsUnavailableStore(t *testing.T) {
	t.Parallel()
	handler := New(Deps{})
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := handler.List(req, json.RawMessage(`{}`))
	if !responseIsError(resp) || !containsResult(resp, mcp.ErrNotInitialized) {
		t.Fatalf("response = %s", resp.Result)
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

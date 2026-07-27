// registry.go — Canonical five-tool module registry and dispatch validation.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package toolmodule

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type Description struct {
	Name    string `json:"name"`
	Summary string `json:"summary,omitempty"`
}

type Module interface {
	Validate(json.RawMessage) error
	Execute(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	Describe() Description
	Examples() []json.RawMessage
}

type Handler func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse

type Spec struct {
	Name     string
	Summary  string
	Examples []json.RawMessage
	Handle   Handler
}

type methodModule struct {
	spec Spec
}

func (m *methodModule) Validate(args json.RawMessage) error {
	if len(args) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(args, &decoded); err != nil {
		return err
	}
	if _, ok := decoded.(map[string]any); !ok {
		return fmt.Errorf("tool_dispatch: arguments must be a JSON object, got %T. Wrap tool arguments in {}", decoded)
	}
	return nil
}

func (m *methodModule) Execute(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return m.spec.Handle(req, args)
}

func (m *methodModule) Describe() Description {
	return Description{Name: m.spec.Name, Summary: m.spec.Summary}
}

func (m *methodModule) Examples() []json.RawMessage { return m.spec.Examples }

type Registry struct {
	modules map[string]Module
}

func New(specs ...Spec) *Registry {
	registry := &Registry{modules: make(map[string]Module, len(specs))}
	for _, spec := range specs {
		if spec.Name != "" && spec.Handle != nil {
			registry.modules[spec.Name] = &methodModule{spec: spec}
		}
	}
	return registry
}

func (r *Registry) Register(name string, module Module) {
	if r == nil || name == "" || module == nil {
		return
	}
	r.modules[name] = module
}

func (r *Registry) Get(name string) (Module, bool) {
	if r == nil {
		return nil, false
	}
	module, ok := r.modules[name]
	return module, ok
}

func (r *Registry) Dispatch(req mcp.JSONRPCRequest, name string, args json.RawMessage) (mcp.JSONRPCResponse, bool) {
	module, ok := r.Get(name)
	if !ok {
		return mcp.JSONRPCResponse{}, false
	}
	if err := module.Validate(args); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Invalid %s arguments: %v", name, err),
			"Fix the request parameters and try again"), true
	}
	return module.Execute(req, args), true
}

func (r *Registry) Examples(name string) []json.RawMessage {
	module, ok := r.Get(name)
	if !ok {
		return nil
	}
	return module.Examples()
}

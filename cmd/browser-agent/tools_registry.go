// Purpose: Defines the ToolModule interface and pluggable module registry for mode-based tool dispatch.
// Why: Enables tool modes to be registered as self-contained modules with validate/execute/describe contracts.

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// sortedMapKeys returns a stable comma-separated registry key list.
func sortedMapKeys[T any](registry map[string]T) string {
	return strings.Join(util.SortedMapKeys(registry), ", ")
}

// ToolModuleDescription provides lightweight metadata for docs and diagnostics.
type ToolModuleDescription struct {
	Name    string `json:"name"`
	Summary string `json:"summary,omitempty"`
}

// ToolModule is the shared runtime contract for plugin-style tool modules.
//
// Validate should only validate/normalize request arguments.
// Execute performs the tool action and returns a JSON-RPC response.
// Describe and Examples provide module metadata and representative calls.
type ToolModule interface {
	Validate(args json.RawMessage) error
	Execute(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse
	Describe() ToolModuleDescription
	Examples() []json.RawMessage
}

type moduleValidateFunc func(args json.RawMessage) error
type moduleExecuteFunc func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

type toolMethodModule struct {
	name     string
	summary  string
	examples []json.RawMessage
	validate moduleValidateFunc
	execute  moduleExecuteFunc
}

func newToolMethodModule(
	name string,
	summary string,
	examples []json.RawMessage,
	validate moduleValidateFunc,
	execute moduleExecuteFunc,
) ToolModule {
	return &toolMethodModule{
		name:     name,
		summary:  summary,
		examples: examples,
		validate: validate,
		execute:  execute,
	}
}

func (m *toolMethodModule) Validate(args json.RawMessage) error {
	if m.validate == nil {
		return nil
	}
	return m.validate(args)
}

func (m *toolMethodModule) Execute(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return m.execute(req, args)
}

func (m *toolMethodModule) Describe() ToolModuleDescription {
	return ToolModuleDescription{
		Name:    m.name,
		Summary: m.summary,
	}
}

func (m *toolMethodModule) Examples() []json.RawMessage {
	return m.examples
}

func validateJSONObjectArgs(args json.RawMessage) error {
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

// toolModuleRegistry stores tool modules by MCP tool name.
type toolModuleRegistry struct {
	modules map[string]ToolModule
}

func newToolModuleRegistry() *toolModuleRegistry {
	return &toolModuleRegistry{modules: make(map[string]ToolModule)}
}

func (r *toolModuleRegistry) register(name string, module ToolModule) {
	if r == nil || name == "" || module == nil {
		return
	}
	r.modules[name] = module
}

func (r *toolModuleRegistry) get(name string) (ToolModule, bool) {
	if r == nil {
		return nil, false
	}
	module, ok := r.modules[name]
	return module, ok
}

// buildToolModuleRegistry wires available modules.
// During migration only selected tools are registered here.
func (h *ToolHandler) buildToolModuleRegistry() *toolModuleRegistry {
	registry := newToolModuleRegistry()
	registry.register("observe", newToolMethodModule(
		"observe",
		"Read captured browser state: logs, network, screenshots, and async results",
		[]json.RawMessage{json.RawMessage(`{"what":"logs"}`), json.RawMessage(`{"what":"screenshot"}`)},
		validateJSONObjectArgs,
		h.toolObserve,
	))
	registry.register("analyze", newToolMethodModule(
		"analyze",
		"Run analysis checks over DOM, links, accessibility, and audits",
		[]json.RawMessage{json.RawMessage(`{"what":"dom","selector":"body","background":true}`)},
		validateJSONObjectArgs,
		h.toolAnalyze,
	))
	registry.register("generate", newToolMethodModule(
		"generate",
		"Generate artifacts (reproduction, csp, sarif, tests) from captured context",
		[]json.RawMessage{json.RawMessage(`{"what":"reproduction","last_n":20}`)},
		validateJSONObjectArgs,
		h.toolGenerate,
	))
	registry.register("configure", newToolMethodModule(
		"configure",
		"Session settings, diagnostics, and recording utilities",
		[]json.RawMessage{
			json.RawMessage(`{"what":"health"}`),
			json.RawMessage(`{"what":"clear","buffer":"logs"}`),
		},
		validateJSONObjectArgs,
		h.toolConfigure,
	))
	registry.register("interact", newToolMethodModule(
		"interact",
		"Browser automation: navigate, click, type, fill forms, take screenshots, and control any web page",
		[]json.RawMessage{
			json.RawMessage(`{"what":"navigate","url":"https://example.com"}`),
			json.RawMessage(`{"what":"click","selector":"button.submit"}`),
			json.RawMessage(`{"what":"type","selector":"input[name=search]","text":"hello"}`),
		},
		validateJSONObjectArgs,
		h.toolInteract,
	))
	return registry
}

// dispatchViaModules routes a request through the module registry when available.
func (h *ToolHandler) dispatchViaModules(req JSONRPCRequest, name string, args json.RawMessage) (JSONRPCResponse, bool) {
	module, ok := h.toolModules.get(name)
	if !ok {
		return JSONRPCResponse{}, false
	}

	if err := module.Validate(args); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Invalid %s arguments: %v", name, err), "Fix the request parameters and try again"), true
	}

	return module.Execute(req, args), true
}

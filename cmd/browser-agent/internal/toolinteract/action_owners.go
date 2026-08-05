// action_owners.go — Change-coupled interact action owners and their explicit dependencies.
// Purpose: Declares external seams and package-local helpers.
// Why: Decouples handlers from the main package without circular imports.

package toolinteract

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/elemindex"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// RuntimeDeps contains only the command lifecycle capabilities shared by all families.
type RuntimeDeps struct {
	RequireCSPClear        func(mcp.JSONRPCRequest, string) (mcp.JSONRPCResponse, bool)
	EnqueuePendingQuery    func(mcp.JSONRPCRequest, queries.PendingQuery, time.Duration) (mcp.JSONRPCResponse, bool)
	MaybeWaitForCommand    func(mcp.JSONRPCRequest, string, json.RawMessage, string) mcp.JSONRPCResponse
	RecordAIAction         func(string, string, map[string]any)
	DefaultEvidenceCapture func(string) EvidenceShot
}

type DOMDeps struct {
	RequirePilot, RequireExtension, RequireTabTracking toolguard.Check
	RecordDOMPrimitiveAction                           func(string, string, string, string)
}

type BrowserDeps struct {
	RequirePilot, RequireExtension, RequireTabTracking toolguard.Check
	Capture                                            func() *capture.Capture
	InjectCSPBlockedActions                            func(mcp.JSONRPCResponse) mcp.JSONRPCResponse
	GetListenPort                                      func() int
}

type PageDeps struct {
	RequirePilot, RequireExtension, RequireTabTracking toolguard.Check
	Capture                                            func() *capture.Capture
	EnqueuePendingQuery                                func(mcp.JSONRPCRequest, queries.PendingQuery, time.Duration) (mcp.JSONRPCResponse, bool)
	RecordAIAction                                     func(string, string, map[string]any)
	MarkDrawStarted                                    func()
	GetScreenshot                                      func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	GetPageInfo                                        func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
}

type WorkflowDeps struct {
	Capture                      func() *capture.Capture
	ToolAnalyze, ToolExportSARIF func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	Now                          func() time.Time
}

type StorageDeps struct {
	RequirePilot, RequireExtension, RequireTabTracking toolguard.Check
}

type BatchDeps struct {
	RequirePilot, RequireExtension toolguard.Check
	Capture                        func() *capture.Capture
	RecordAIAction                 func(string, string, map[string]any)
	ToolInteract                   func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	ReplayMu                       *sync.Mutex
}

type DOMActions struct {
	runtime              *ActionRuntime
	deps                 DOMDeps
	elementIndexRegistry *elemindex.Registry
}

func NewDOMActions(runtime *ActionRuntime, deps DOMDeps) *DOMActions {
	return &DOMActions{runtime: runtime, deps: deps, elementIndexRegistry: elemindex.New()}
}

type BrowserActions struct {
	runtime *ActionRuntime
	page    *PageActions
	deps    BrowserDeps
}

func NewBrowserActions(runtime *ActionRuntime, page *PageActions, deps BrowserDeps) *BrowserActions {
	return &BrowserActions{runtime: runtime, page: page, deps: deps}
}

type PageActions struct {
	runtime *ActionRuntime
	dom     *DOMActions
	storage *StorageActions
	deps    PageDeps
}

func NewPageActions(runtime *ActionRuntime, dom *DOMActions, storage *StorageActions, deps PageDeps) *PageActions {
	return &PageActions{runtime: runtime, dom: dom, storage: storage, deps: deps}
}

type WorkflowActions struct {
	runtime *ActionRuntime
	dom     *DOMActions
	browser *BrowserActions
	page    *PageActions
	deps    WorkflowDeps
}

func NewWorkflowActions(runtime *ActionRuntime, dom *DOMActions, browser *BrowserActions, page *PageActions, deps WorkflowDeps) *WorkflowActions {
	return &WorkflowActions{runtime: runtime, dom: dom, browser: browser, page: page, deps: deps}
}

type StorageActions struct {
	runtime *ActionRuntime
	deps    StorageDeps
}

func NewStorageActions(runtime *ActionRuntime, deps StorageDeps) *StorageActions {
	return &StorageActions{runtime: runtime, deps: deps}
}

type BatchActions struct {
	runtime *ActionRuntime
	deps    BatchDeps
}

func NewBatchActions(runtime *ActionRuntime, deps BatchDeps) *BatchActions {
	return &BatchActions{runtime: runtime, deps: deps}
}

// Deps holds all external dependencies interact handlers need from the caller.
func marshalQueryParams(fields map[string]any) json.RawMessage {
	return mcp.SafeMarshal(fields, "{}")
}

func checkGuards(req mcp.JSONRPCRequest, guards ...toolguard.Check) (mcp.JSONRPCResponse, bool) {
	for _, guard := range guards {
		if resp, blocked := guard(req); blocked {
			return resp, true
		}
	}
	return mcp.JSONRPCResponse{}, false
}

func checkGuardsWithOpts(req mcp.JSONRPCRequest, opts []func(*mcp.StructuredError), guards ...toolguard.Check) (mcp.JSONRPCResponse, bool) {
	for _, guard := range guards {
		if resp, blocked := guard(req, opts...); blocked {
			return resp, true
		}
	}
	return mcp.JSONRPCResponse{}, false
}

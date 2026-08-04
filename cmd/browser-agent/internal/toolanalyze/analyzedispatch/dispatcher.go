// dispatcher.go — Owns analyze mode routing and cross-feature mode composition.
// Docs: docs/features/feature/analyze-tool/index.md

package analyzedispatch

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mediaapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/combinedaudit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/inspect"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/pageissues"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/verificationhandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/visual"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrouting"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	observe "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

type ModeHandler func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse

type Config struct {
	Analyze          toolanalyze.Deps
	Inspect          inspect.Deps
	Observe          observe.Deps
	Audit            combinedaudit.Deps
	Version          string
	AnnotationStore  *annotation.Store
	Visual           visual.Deps
	ValidateAPI      ModeHandler
	PageSummary      ModeHandler
	Annotations      ModeHandler
	AnnotationDetail ModeHandler
	FeatureGates     ModeHandler
}

type Dispatcher struct {
	config   Config
	registry toolrouting.Registry[struct{}]
}

func NewDispatcher(config Config) *Dispatcher {
	d := &Dispatcher{config: config}
	handlers := map[string]toolrouting.Handler[struct{}]{
		"dom": wrapInspect(config.Inspect, inspect.HandleDOM), "api_validation": func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return d.ValidateAPI(req, args)
		},
		"page_summary": mode(config.PageSummary), "performance": wrapObserve(config.Observe, observe.CheckPerformance),
		"accessibility": wrapObserve(config.Observe, observe.RunA11yAudit), "error_clusters": wrapObserve(config.Observe, observe.AnalyzeErrors),
		"navigation_patterns": wrapObserve(config.Observe, observe.AnalyzeHistory),
		"security_audit":      wrapLocal(config.Analyze, toolanalyze.HandleSecurityAudit),
		"third_party_audit":   wrapLocal(config.Analyze, toolanalyze.HandleThirdPartyAudit),
		"link_health":         wrapLocal(config.Analyze, toolanalyze.HandleLinkHealth),
		"link_validation": func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return toolanalyze.HandleLinkValidation(req, args, config.Version)
		},
		"annotations": mode(config.Annotations), "annotation_detail": mode(config.AnnotationDetail),
		"draw_history": func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return d.DrawHistory(req, args)
		},
		"draw_session": func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return d.DrawSession(req, args)
		},
		"computed_styles":  wrapInspect(config.Inspect, inspect.HandleComputedStyles),
		"forms":            wrapInspect(config.Inspect, inspect.HandleFormDiscovery),
		"form_state":       wrapInspect(config.Inspect, inspect.HandleFormState),
		"form_validation":  wrapInspect(config.Inspect, inspect.HandleFormValidation),
		"data_table":       wrapInspect(config.Inspect, inspect.HandleDataTable),
		"visual_baseline":  wrapVisual(config.Visual, visual.SaveBaseline),
		"visual_diff":      wrapVisual(config.Visual, visual.DiffBaseline),
		"visual_baselines": wrapVisual(config.Visual, visual.ListBaselines),
		"navigation":       wrapLocal(config.Analyze, toolanalyze.HandleNavigation),
		"page_structure":   wrapLocal(config.Analyze, toolanalyze.HandlePageStructure),
		"audit": func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return combinedaudit.Handle(config.Audit, req, args)
		},
		"page_issues": wrapLocal(config.Analyze, pageissues.Handle), "feature_gates": mode(config.FeatureGates),
		"performance_trace": wrapLocal(config.Analyze, HandlePerformanceTrace),
		"react_profile":     wrapLocal(config.Analyze, HandleReactProfile),
		"verification": func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return verificationhandler.Handle(req, args)
		},
	}
	d.registry = toolrouting.Registry[struct{}]{
		Handlers: handlers,
		Resolution: toolrouting.Resolution{
			ToolName: "analyze", ValidModes: strings.Join(util.SortedMapKeys(handlers), ", "),
		},
	}
	return d
}

func (d *Dispatcher) Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return toolrouting.Dispatch(struct{}{}, req, args, d.registry)
}

func (d *Dispatcher) ValidModes() []string { return util.SortedMapKeys(d.registry.Handlers) }

func (d *Dispatcher) ValidateAPI(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return d.config.ValidateAPI(req, args)
}

func (d *Dispatcher) DrawHistory(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	dir, err := mediaapi.ScreenshotsDir()
	return annotation.ListDrawHistory(req, dir, err)
}

func (d *Dispatcher) DrawSession(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	dir, err := mediaapi.ScreenshotsDir()
	return annotation.LoadDrawSession(d.config.AnnotationStore, req, args, dir, err)
}

func mode(fn ModeHandler) toolrouting.Handler[struct{}] {
	return func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(req, args)
	}
}

func wrapLocal(deps toolanalyze.Deps, fn func(toolanalyze.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[struct{}] {
	return func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(deps, req, args)
	}
}

func wrapInspect(deps inspect.Deps, fn func(inspect.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[struct{}] {
	return func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(deps, req, args)
	}
}

func wrapObserve(deps observe.Deps, fn func(observe.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[struct{}] {
	return func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(deps, req, args)
	}
}

func wrapVisual(deps visual.Deps, fn func(visual.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[struct{}] {
	return func(_ struct{}, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(deps, req, args)
	}
}

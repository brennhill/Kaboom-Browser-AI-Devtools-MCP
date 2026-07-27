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
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/visual"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrouting"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	observe "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

type Host interface {
	combinedaudit.Deps
}

type ModeHandler func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse

type Config struct {
	Host             Host
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
	host     Host
	config   Config
	registry toolrouting.Registry[Host]
}

func NewDispatcher(config Config) *Dispatcher {
	d := &Dispatcher{host: config.Host, config: config}
	handlers := map[string]toolrouting.Handler[Host]{
		"dom": wrapInspect(inspect.HandleDOM), "api_validation": func(_ Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return d.ValidateAPI(req, args)
		},
		"page_summary": mode(config.PageSummary), "performance": wrapObserve(observe.CheckPerformance),
		"accessibility": wrapObserve(observe.RunA11yAudit), "error_clusters": wrapObserve(observe.AnalyzeErrors),
		"navigation_patterns": wrapObserve(observe.AnalyzeHistory),
		"security_audit":      wrapLocal(toolanalyze.HandleSecurityAudit), "third_party_audit": wrapLocal(toolanalyze.HandleThirdPartyAudit),
		"link_health": wrapLocal(toolanalyze.HandleLinkHealth),
		"link_validation": func(_ Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return toolanalyze.HandleLinkValidation(req, args, config.Version)
		},
		"annotations": mode(config.Annotations), "annotation_detail": mode(config.AnnotationDetail),
		"draw_history": func(_ Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return d.DrawHistory(req, args)
		},
		"draw_session": func(_ Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return d.DrawSession(req, args)
		},
		"computed_styles": wrapInspect(inspect.HandleComputedStyles), "forms": wrapInspect(inspect.HandleFormDiscovery),
		"form_state": wrapInspect(inspect.HandleFormState), "form_validation": wrapInspect(inspect.HandleFormValidation),
		"data_table":       wrapInspect(inspect.HandleDataTable),
		"visual_baseline":  wrapVisual(config.Visual, visual.SaveBaseline),
		"visual_diff":      wrapVisual(config.Visual, visual.DiffBaseline),
		"visual_baselines": wrapVisual(config.Visual, visual.ListBaselines),
		"navigation":       wrapLocal(toolanalyze.HandleNavigation), "page_structure": wrapLocal(toolanalyze.HandlePageStructure),
		"audit": func(h Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return combinedaudit.Handle(h, req, args)
		},
		"page_issues": wrapLocal(pageissues.Handle), "feature_gates": mode(config.FeatureGates),
	}
	d.registry = toolrouting.Registry[Host]{
		Handlers: handlers, AliasDefs: toolrouting.DefaultModeActionAliases,
		Resolution: toolrouting.Resolution{
			ToolName: "analyze", ValidModes: strings.Join(util.SortedMapKeys(handlers), ", "),
			ValueAliases: map[string]toolrouting.ValueAlias{
				"a11y":    {Canonical: "accessibility", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
				"history": {Canonical: "navigation_patterns", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
			},
		},
	}
	return d
}

func (d *Dispatcher) Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return toolrouting.Dispatch(d.host, req, args, d.registry)
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

func mode(fn ModeHandler) toolrouting.Handler[Host] {
	return func(_ Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse { return fn(req, args) }
}

func wrapLocal(fn func(toolanalyze.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[Host] {
	return func(h Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func wrapInspect(fn func(inspect.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[Host] {
	return func(h Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func wrapObserve(fn func(observe.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[Host] {
	return func(h Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func wrapVisual(deps visual.Deps, fn func(visual.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[Host] {
	return func(_ Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(deps, req, args)
	}
}

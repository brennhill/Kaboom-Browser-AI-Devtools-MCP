// Purpose: Defines the built-in baseline noise-rule catalog shipped with noise filtering.
// Why: One catalog, one file — the category tables are pure data with no cross-references and
// have only ever been edited as a single unit, so splitting them added files without adding seams.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import "time"

// builtinRuleSpec is the authoring form of a built-in rule: identity plus match
// criteria, with CreatedAt supplied at build time by builtinRules.
type builtinRuleSpec struct {
	ID             string
	Category       string
	Classification string
	MatchSpec      NoiseMatchSpec
}

func builtinRules() []NoiseRule {
	now := time.Now()
	specs := collectBuiltinRuleSpecs()
	rules := make([]NoiseRule, 0, len(specs))
	for _, spec := range specs {
		rules = append(rules, NoiseRule{
			ID:             spec.ID,
			Category:       spec.Category,
			Classification: spec.Classification,
			MatchSpec:      spec.MatchSpec,
			CreatedAt:      now,
		})
	}
	return rules
}

// collectBuiltinRuleSpecs concatenates the category tables below.
// Order is part of the observable contract: ListRules and the persisted rule set
// preserve it, so categories must stay in this sequence.
func collectBuiltinRuleSpecs() []builtinRuleSpec {
	specs := make([]builtinRuleSpec, 0, 64)
	specs = append(specs, builtinBrowserRuleSpecs()...)
	specs = append(specs, builtinDevToolingRuleSpecs()...)
	specs = append(specs, builtinAnalyticsRuleSpecs()...)
	specs = append(specs, builtinFrameworkRuleSpecs()...)
	specs = append(specs, builtinWebSocketRuleSpecs()...)
	return specs
}

// ============================================
// Browser environment: extensions, favicons, cookie and deprecation notices.
// ============================================

func builtinBrowserRuleSpecs() []builtinRuleSpec {
	return []builtinRuleSpec{
		{
			ID:             "builtin_chrome_extension",
			Category:       "console",
			Classification: "extension",
			MatchSpec: NoiseMatchSpec{
				SourceRegex: `(chrome|moz)-extension://`,
			},
		},
		{
			ID:             "builtin_favicon",
			Category:       "network",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `favicon\.ico`,
			},
		},
		{
			ID:             "builtin_sourcemap_404",
			Category:       "network",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				URLRegex:  `\.map(\?|$)`,
				StatusMin: 400,
				StatusMax: 499,
			},
		},
		{
			ID:             "builtin_cors_preflight",
			Category:       "network",
			Classification: "infrastructure",
			MatchSpec: NoiseMatchSpec{
				Method:    "OPTIONS",
				StatusMin: 200,
				StatusMax: 299,
			},
		},
		{
			ID:             "builtin_service_worker",
			Category:       "console",
			Classification: "infrastructure",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(?i)(service.?worker|ServiceWorker).*(regist|install|activat|updated)`,
			},
		},
		{
			ID:             "builtin_passive_listener",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `non-passive event listener`,
			},
		},
		{
			ID:             "builtin_deprecation",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `^\[Deprecation\]`,
			},
		},
		{
			ID:             "builtin_devtools_sourcemap",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `DevTools failed to load source map`,
			},
		},
		{
			ID:             "builtin_err_blocked",
			Category:       "console",
			Classification: "extension",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `net::ERR_BLOCKED_BY_CLIENT`,
			},
		},
		{
			ID:             "builtin_samesite_cookie",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Indicate whether to send a cookie`,
			},
		},
		{
			ID:             "builtin_third_party_cookie",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `third-party cookie will be blocked`,
			},
		},
	}
}

// ============================================
// Dev tooling: HMR, source maps, dev-server chatter.
// ============================================

func builtinDevToolingRuleSpecs() []builtinRuleSpec {
	return []builtinRuleSpec{
		{
			ID:             "builtin_hmr_console",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `^\[(vite|HMR|webpack|next)\]`,
			},
		},
		{
			ID:             "builtin_hmr_network",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(__vite_ping|hot-update\.(json|js)|__webpack_hmr|sockjs-node|_next/webpack-hmr|webpack-dev-server)`,
			},
		},
		{
			ID:             "builtin_react_devtools",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(Download the React DevTools|React DevTools)`,
			},
		},
		{
			ID:             "builtin_angular_dev_mode",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Angular is running in (the )?development mode`,
			},
		},
		{
			ID:             "builtin_vue_devtools",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(Vue\.js|vue-devtools|Vue Devtools)`,
			},
		},
		{
			ID:             "builtin_svelte_hmr",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `\[svelte-hmr\]`,
			},
		},
		{
			ID:             "builtin_fast_refresh",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `\[Fast Refresh\]`,
			},
		},
		{
			ID:             "builtin_next_dev",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `next-dev\.js`,
			},
		},
		{
			ID:             "builtin_vite_prebundle",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(Pre-bundling|Optimized dependencies|new dependencies optimized)`,
			},
		},
		{
			ID:             "builtin_cra_disconnect",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `The development server has disconnected`,
			},
		},
	}
}

// ============================================
// Analytics and session-replay beacons.
// ============================================

func builtinAnalyticsRuleSpecs() []builtinRuleSpec {
	return []builtinRuleSpec{
		{
			ID:             "builtin_google_analytics",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(google-analytics\.com|analytics\.google\.com|googletagmanager\.com|gtag/js)`,
			},
		},
		{
			ID:             "builtin_segment",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(api\.segment\.(io|com)|cdn\.segment\.com)`,
			},
		},
		{
			ID:             "builtin_mixpanel",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(api\.mixpanel\.com|mxpnl\.com)`,
			},
		},
		{
			ID:             "builtin_hotjar",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `\.hotjar\.com`,
			},
		},
		{
			ID:             "builtin_amplitude",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `api\.amplitude\.com`,
			},
		},
		{
			ID:             "builtin_plausible",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `plausible\.io`,
			},
		},
		{
			ID:             "builtin_posthog",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(app\.posthog\.com|us\.posthog\.com|eu\.posthog\.com)`,
			},
		},
		{
			ID:             "builtin_datadog_rum",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `rum\.browser-intake.*\.datadoghq\.(com|eu)`,
			},
		},
		{
			ID:             "builtin_sentry",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `\.ingest\.sentry\.io`,
			},
		},
		{
			ID:             "builtin_logrocket",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(r\.lr-ingest\.io|r\.lr-in\.com)`,
			},
		},
		{
			ID:             "builtin_fullstory",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(rs\.fullstory\.com|fullstory\.com/s/fs\.js)`,
			},
		},
		{
			ID:             "builtin_heap",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(heapanalytics\.com|heap-js\.heap\.io)`,
			},
		},
	}
}

// ============================================
// Framework runtime warnings and internal asset routes.
// ============================================

func builtinFrameworkRuleSpecs() []builtinRuleSpec {
	return []builtinRuleSpec{
		{
			ID:             "builtin_react_key_warning",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Each child in a list should have a unique.*key`,
				Level:        "warning",
			},
		},
		{
			ID:             "builtin_react_update_during_render",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Cannot update a component.*while rendering a different component`,
				Level:        "warning",
			},
		},
		{
			ID:             "builtin_react_strict_mode",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(StrictMode|Strict Mode).*(double|twice)`,
			},
		},
		{
			ID:             "builtin_next_hydration_info",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(hydration|Hydration).*(mismatch|failed|warning)`,
				Level:        "warning",
			},
		},
		{
			ID:             "builtin_next_internal",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `/_next/(static|data|image)/`,
			},
		},
		{
			ID:             "builtin_vite_client",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `/@vite/client`,
			},
		},
		{
			ID:             "builtin_webpack_internal",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `webpack-internal://`,
			},
		},
	}
}

// ============================================
// WebSocket: HMR sockets and devtools inspector connections.
// ============================================

func builtinWebSocketRuleSpecs() []builtinRuleSpec {
	return []builtinRuleSpec{
		{
			ID:             "builtin_ws_hmr",
			Category:       "websocket",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(/__vite_hmr|localhost(:\d+)?/ws(\?|$)|/_next/webpack-hmr|/sockjs-node)`,
			},
		},
		{
			ID:             "builtin_ws_devtools",
			Category:       "websocket",
			Classification: "extension",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(devtools|__browser_inspector)`,
			},
		},
	}
}

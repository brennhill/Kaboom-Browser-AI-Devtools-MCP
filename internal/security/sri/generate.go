// generate.go — Third-party filtering and the SRI hash generation pipeline.
// Purpose: Generates Subresource Integrity hashes with origin filtering and output formatting.
// Why: Separates SRI generation logic from tooling integration and type definitions.
package sri

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// newFilterConfig derives normalized filter state from params/page context.
//
// Failure semantics:
// - Invalid page URLs simply do not contribute first-party origins.
func newFilterConfig(pageURLs []string, params Params) filterConfig {
	cfg := filterConfig{
		firstPartyOrigins: make(map[string]bool),
		originFilter:      make(map[string]bool),
		includeScripts:    true,
		includeStyles:     true,
	}
	for _, pageURL := range pageURLs {
		if origin := util.ExtractOrigin(pageURL); origin != "" {
			cfg.firstPartyOrigins[origin] = true
		}
	}
	for _, o := range params.Origins {
		cfg.originFilter[o] = true
	}
	if len(params.ResourceTypes) > 0 {
		cfg.includeScripts = false
		cfg.includeStyles = false
		for _, rt := range params.ResourceTypes {
			switch rt {
			case "scripts":
				cfg.includeScripts = true
			case "styles":
				cfg.includeStyles = true
			}
		}
	}
	return cfg
}

// shouldIncludeResourceType returns whether resource type passes requested filters.
func (cfg filterConfig) shouldIncludeResourceType(resType string) bool {
	return (resType == "script" && cfg.includeScripts) || (resType == "style" && cfg.includeStyles)
}

// hasVaryUserAgent checks if response headers contain Vary: User-Agent.
func hasVaryUserAgent(headers map[string]string) bool {
	if headers == nil {
		return false
	}
	for key, val := range headers {
		if strings.EqualFold(key, "Vary") && strings.Contains(strings.ToLower(val), "user-agent") {
			return true
		}
	}
	return false
}

// isPlaceholderBody returns true if the response body is a capture placeholder
// rather than actual resource content (e.g., from read timeout or binary detection).
func isPlaceholderBody(body string) bool {
	return len(body) > 2 && body[0] == '[' && body[len(body)-1] == ']'
}

// evaluateBody applies eligibility checks and computes one SRI resource when possible.
//
// Invariants:
// - seenURLs de-duplicates by full resource URL per generation run.
//
// Failure semantics:
// - Placeholder/truncated bodies are surfaced as warnings and skipped from hash output.
func (g *Generator) evaluateBody(body types.NetworkBody, cfg filterConfig, seenURLs map[string]bool) bodyOutcome {
	if body.ResponseBody == "" {
		return bodyOutcome{skip: true}
	}
	origin := util.ExtractOrigin(body.URL)
	if origin == "" || cfg.firstPartyOrigins[origin] {
		return bodyOutcome{skip: true}
	}

	resType := classifyResourceType(body.ContentType)
	if resType == "" {
		return bodyOutcome{thirdParty: true, skip: true}
	}
	if !cfg.shouldIncludeResourceType(resType) || (len(cfg.originFilter) > 0 && !cfg.originFilter[origin]) || seenURLs[body.URL] {
		return bodyOutcome{thirdParty: true, resType: resType, skip: true}
	}
	seenURLs[body.URL] = true

	if isPlaceholderBody(body.ResponseBody) {
		return bodyOutcome{thirdParty: true, resType: resType, placeholder: true}
	}

	if body.ResponseTruncated {
		return bodyOutcome{thirdParty: true, resType: resType, truncated: true}
	}

	hash := computeSHA384(body.ResponseBody)
	return bodyOutcome{
		thirdParty: true,
		resType:    resType,
		varyUA:     hasVaryUserAgent(body.ResponseHeaders),
		resource: Resource{
			URL: body.URL, Type: resType, Hash: hash,
			Crossorigin: "anonymous", TagTemplate: generateTagTemplate(body.URL, hash, resType),
			SizeBytes: len(body.ResponseBody), AlreadyHasSRI: false,
		},
	}
}

// buildWarnings converts skip diagnostics into user-facing warning messages.
func buildWarnings(truncated, placeholder, varyUA []string) []string {
	warnings := make([]string, 0, len(truncated)+len(placeholder)+len(varyUA))
	for _, u := range truncated {
		warnings = append(warnings, fmt.Sprintf("%s — body was truncated, cannot compute SRI hash. Consider increasing capture limit.", u))
	}
	for _, u := range placeholder {
		warnings = append(warnings, fmt.Sprintf("%s — body was not captured (read timeout or binary). Re-navigate the page and retry.", u))
	}
	for _, u := range varyUA {
		warnings = append(warnings, fmt.Sprintf("%s — responds with Vary: User-Agent header. SRI hash may differ across browsers.", u))
	}
	return warnings
}

// Generate analyzes captured bodies and emits hashable third-party script/style resources.
//
// Invariants:
// - Summary counters include filtered/skipped third-party resources for auditability.
//
// Failure semantics:
// - Non-hashable resources are excluded with warnings instead of aborting the run.
func (g *Generator) Generate(bodies []types.NetworkBody, pageURLs []string, params Params) Result {
	cfg := newFilterConfig(pageURLs, params)
	result := Result{Resources: []Resource{}, Warnings: []string{}}
	seenURLs := make(map[string]bool)

	var totalThirdParty, scriptsWithoutSRI, stylesWithoutSRI int
	var truncated, placeholder, varyUA []string

	for _, body := range bodies {
		out := g.evaluateBody(body, cfg, seenURLs)
		if out.thirdParty {
			totalThirdParty++
		}
		switch out.resType {
		case "script":
			scriptsWithoutSRI++
		case "style":
			stylesWithoutSRI++
		}
		if out.placeholder {
			placeholder = append(placeholder, body.URL)
			continue
		}
		if out.truncated {
			truncated = append(truncated, body.URL)
			continue
		}
		if out.skip || out.resType == "" {
			continue
		}
		if out.varyUA {
			varyUA = append(varyUA, body.URL)
		}
		result.Resources = append(result.Resources, out.resource)
	}

	result.Warnings = buildWarnings(truncated, placeholder, varyUA)
	result.Summary = Summary{
		TotalThirdPartyResources: totalThirdParty,
		ScriptsWithoutSRI:        scriptsWithoutSRI,
		StylesWithoutSRI:         stylesWithoutSRI,
		HashesGenerated:          len(result.Resources),
	}
	return result
}

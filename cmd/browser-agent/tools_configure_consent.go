// tools_configure_consent.go — configure(mode='consent'): inspect and change which origins
// kaboom is permitted to DRIVE.
// Docs: docs/features/feature/browser-consent/index.md

package main

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browserconsent"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// consentArgs is the wire shape of configure(mode='consent').
type consentArgs struct {
	Action string `json:"action"`
	Origin string `json:"origin"`
	Scope  string `json:"scope"`
}

// handleConfigureConsent lists, grants, and revokes driving consent.
//
// Granting is a user decision, so it is exposed as an explicit action rather than inferred
// from usage: an agent that could widen its own permissions by acting is not gated at all.
func handleConfigureConsent(policy *browserconsent.Policy, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params consentArgs
	if len(args) > 0 {
		if resp, failed := mcp.ParseArgs(req, args, &params); failed {
			return resp
		}
	}

	switch params.Action {
	case "", "list":
		origins := policy.List()
		return mcp.Succeed(req, fmt.Sprintf("%d origin(s) consented for driving", len(origins)), map[string]any{
			"consented_origins": origins,
			"note":              "Origins kaboom may drive. Loopback origins are allowed by default and are not listed.",
		})

	case "allow", "allow_session":
		if params.Origin == "" {
			return mcp.Fail(req, mcp.ErrMissingParam, "consent 'allow' requires an origin",
				"Pass origin='https://example.com'.", mcp.WithParam("origin"))
		}
		grant := policy.Allow
		scope := "persistent"
		if params.Action == "allow_session" || params.Scope == "session" {
			grant = policy.AllowForSession
			scope = "session"
		}
		if err := grant(params.Origin); err != nil {
			return mcp.Fail(req, mcp.ErrInvalidParam, err.Error(),
				"Pass a full http(s) origin, for example 'https://example.com'.", mcp.WithParam("origin"))
		}
		origin, _ := browserconsent.OriginOf(params.Origin)
		return mcp.Succeed(req, fmt.Sprintf("Kaboom may now drive %s (%s)", origin, scope), map[string]any{
			"granted": origin,
			"scope":   scope,
		})

	case "revoke":
		if params.Origin == "" {
			return mcp.Fail(req, mcp.ErrMissingParam, "consent 'revoke' requires an origin",
				"Pass origin='https://example.com'.", mcp.WithParam("origin"))
		}
		if err := policy.Revoke(params.Origin); err != nil {
			return mcp.Fail(req, mcp.ErrInvalidParam, err.Error(),
				"Pass a full http(s) origin, for example 'https://example.com'.", mcp.WithParam("origin"))
		}
		origin, _ := browserconsent.OriginOf(params.Origin)
		return mcp.Succeed(req, fmt.Sprintf("Revoked driving consent for %s", origin), map[string]any{"revoked": origin})

	case "clear_session":
		policy.ClearSession()
		return mcp.Succeed(req, "Cleared session-scoped driving consent", map[string]any{"cleared": "session"})

	case "allow_localhost", "deny_localhost":
		allow := params.Action == "allow_localhost"
		policy.SetAllowLocalhost(allow)
		return mcp.Succeed(req, fmt.Sprintf("allow_localhost=%v", allow), map[string]any{"allow_localhost": allow})

	default:
		return mcp.Fail(req, mcp.ErrInvalidParam,
			fmt.Sprintf("Unknown consent action %q", params.Action),
			"Use action='list', 'allow', 'allow_session', 'revoke', 'clear_session', 'allow_localhost' or 'deny_localhost'.",
			mcp.WithParam("action"))
	}
}

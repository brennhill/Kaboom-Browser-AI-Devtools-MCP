// handler.go — Owns configure QA fixture validation before browser mutation.

package qafixture

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	fixturecontract "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/qafixture"
)

// Handle validates the current QA fixture contract. Apply and restore are not
// advertised until their transactional browser implementation is available.
func Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		FixtureAction string          `json:"fixture_action"`
		Fixture       json.RawMessage `json:"fixture"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if params.FixtureAction != "validate" {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid fixture_action", "Use fixture_action='validate'", mcp.WithParam("fixture_action"))
	}
	if len(params.Fixture) == 0 || string(params.Fixture) == "null" {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'fixture' is missing", "Add a versioned fixture object", mcp.WithParam("fixture"))
	}
	fixture, err := fixturecontract.Parse(params.Fixture)
	if err != nil {
		message := err.Error()
		if !strings.HasPrefix(message, "invalid fixture JSON") && !strings.HasPrefix(message, "unsupported fixture version") {
			message = "Invalid QA fixture: " + message
		}
		return mcp.Fail(req, mcp.ErrInvalidParam, message, "Correct the fixture contract and validate again", mcp.WithParam("fixture"))
	}
	return mcp.Succeed(req, "QA fixture valid", map[string]any{
		"valid":            true,
		"fixture_version":  fixture.Version,
		"setup_timeout_ms": fixture.SetupTimeoutMs,
	})
}

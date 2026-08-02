// handler.go — Owns configure QA fixture validation and atomic browser application.

package qafixture

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	fixturecontract "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/qafixture"
)

type CommandExecutor func(context.Context, string, json.RawMessage, time.Duration) (json.RawMessage, error)

type Deps struct {
	Context          context.Context
	Execute          CommandExecutor
	NewCorrelationID func() string
}

type Handler struct {
	ctx         context.Context
	coordinator *fixturecontract.Coordinator
}

func New(deps Deps) (*Handler, error) {
	if deps.Context == nil || deps.Execute == nil || deps.NewCorrelationID == nil {
		return nil, errors.New("incomplete_qa_fixture_dependencies")
	}
	coordinator, err := fixturecontract.NewCoordinator(fixturecontract.TransactionDeps{
		NewCorrelationID: deps.NewCorrelationID,
		Snapshot: func(ctx context.Context, fixture fixturecontract.WireQAFixture) (json.RawMessage, error) {
			result, err := executeFixtureCommand(ctx, deps.Execute, "environment_transaction_snapshot", &fixture, "", fixture.SetupTimeoutMs)
			if err != nil {
				return nil, err
			}
			var snapshot struct {
				Success    bool   `json:"success"`
				SnapshotID string `json:"snapshot_id"`
			}
			if json.Unmarshal(result, &snapshot) != nil || !snapshot.Success || snapshot.SnapshotID == "" {
				return nil, errors.New("invalid_fixture_snapshot_result")
			}
			return json.Marshal(struct {
				SnapshotID string `json:"snapshot_id"`
			}{SnapshotID: snapshot.SnapshotID})
		},
		Apply: func(ctx context.Context, fixture fixturecontract.WireQAFixture) (fixturecontract.MutationCounts, error) {
			result, err := executeFixtureCommand(ctx, deps.Execute, "environment_transaction_apply", &fixture, "", fixture.SetupTimeoutMs)
			if err != nil {
				return fixturecontract.MutationCounts{}, err
			}
			var applied struct {
				Success   bool                           `json:"success"`
				Mutations fixturecontract.MutationCounts `json:"mutations"`
			}
			if json.Unmarshal(result, &applied) != nil || !applied.Success {
				return fixturecontract.MutationCounts{}, errors.New("invalid_fixture_apply_result")
			}
			return applied.Mutations, nil
		},
		Restore: func(ctx context.Context, snapshot json.RawMessage) error {
			var reference struct {
				SnapshotID string `json:"snapshot_id"`
			}
			if json.Unmarshal(snapshot, &reference) != nil || reference.SnapshotID == "" {
				return errors.New("invalid_fixture_snapshot_reference")
			}
			result, err := executeFixtureCommand(ctx, deps.Execute, "environment_transaction_restore", nil, reference.SnapshotID, fixturecontract.DefaultSetupTimeoutMs)
			if err != nil {
				return err
			}
			var restored struct {
				Success  bool `json:"success"`
				Restored bool `json:"restored"`
			}
			if json.Unmarshal(result, &restored) != nil || !restored.Success || !restored.Restored {
				return errors.New("invalid_fixture_restore_result")
			}
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return &Handler{ctx: deps.Context, coordinator: coordinator}, nil
}

func (handler *Handler) Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		FixtureAction string          `json:"fixture_action"`
		Fixture       json.RawMessage `json:"fixture"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if params.FixtureAction != "validate" && params.FixtureAction != "apply" {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid fixture_action", "Use fixture_action='validate' or fixture_action='apply'", mcp.WithParam("fixture_action"))
	}
	fixture, response, invalid := parseFixture(req, params.Fixture)
	if invalid {
		return response
	}
	if params.FixtureAction == "validate" {
		return mcp.Succeed(req, "QA fixture valid", map[string]any{
			"valid": true, "fixture_version": fixture.Version, "setup_timeout_ms": fixture.SetupTimeoutMs,
		})
	}
	result, err := handler.coordinator.Apply(handler.ctx, fixture)
	if err != nil {
		return mcp.Fail(req, mcp.ErrExtError, "QA fixture transaction failed: "+result.Status,
			"Inspect configure({what:'doctor'}), correct browser state, and retry the fixture.",
			mcp.WithHint("correlation_id="+result.CorrelationID))
	}
	return mcp.Succeed(req, "QA fixture applied", result)
}

func parseFixture(req mcp.JSONRPCRequest, raw json.RawMessage) (fixturecontract.WireQAFixture, mcp.JSONRPCResponse, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return fixturecontract.WireQAFixture{}, mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'fixture' is missing", "Add a versioned fixture object", mcp.WithParam("fixture")), true
	}
	fixture, err := fixturecontract.Parse(raw)
	if err == nil {
		return fixture, mcp.JSONRPCResponse{}, false
	}
	message := err.Error()
	if !strings.HasPrefix(message, "invalid fixture JSON") && !strings.HasPrefix(message, "unsupported fixture version") {
		message = "Invalid QA fixture: " + message
	}
	return fixturecontract.WireQAFixture{}, mcp.Fail(req, mcp.ErrInvalidParam, message, "Correct the fixture contract and validate again", mcp.WithParam("fixture")), true
}

func executeFixtureCommand(
	ctx context.Context,
	execute CommandExecutor,
	command string,
	fixture *fixturecontract.WireQAFixture,
	snapshotID string,
	timeoutMs int,
) (json.RawMessage, error) {
	params := struct {
		Fixture    *fixturecontract.WireQAFixture `json:"fixture,omitempty"`
		SnapshotID string                         `json:"snapshot_id,omitempty"`
	}{Fixture: fixture, SnapshotID: snapshotID}
	payload, err := json.Marshal(params)
	if err != nil {
		return nil, errors.New("fixture_command_encoding_failed")
	}
	return execute(ctx, command, payload, time.Duration(timeoutMs)*time.Millisecond)
}

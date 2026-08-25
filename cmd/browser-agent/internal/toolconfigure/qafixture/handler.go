// handler.go — Owns configure QA fixture validation and atomic browser application.

package qafixture

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	fixturecontract "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/qafixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

type CommandExecutor func(context.Context, string, json.RawMessage, time.Duration) (json.RawMessage, error)

type LifecycleDiagnostics interface {
	statediag.Reporter
	statediag.Resolver
}

type Deps struct {
	Context             context.Context
	Execute             CommandExecutor
	NewCorrelationID    func() string
	NewTransactionID    func() string
	ExtensionGeneration func() string
	Now                 func() time.Time
	Registry            *fixturecontract.Registry
	Persist             func(*fixturecontract.Registry) error
	OnNotice            func(string)
	Diagnostics         LifecycleDiagnostics
}

type Handler struct {
	ctx         context.Context
	coordinator *fixturecontract.Coordinator
	registry    *fixturecontract.Registry
	persist     func(*fixturecontract.Registry) error
	generation  func() string
	restore     func(context.Context, string) error
	reconcile   func(context.Context, []string) (int, error)
	diagnostics LifecycleDiagnostics
	lifecycleMu sync.Mutex
	startupMu   sync.RWMutex
	startupDone <-chan struct{}
}

func New(deps Deps) (*Handler, error) {
	if deps.Context == nil || deps.Execute == nil || deps.NewCorrelationID == nil || deps.Diagnostics == nil {
		return nil, errors.New("incomplete_qa_fixture_dependencies")
	}
	restore := func(ctx context.Context, snapshotID string) error {
		return restoreEnvironmentSnapshot(ctx, deps.Execute, snapshotID)
	}
	reconcile := func(ctx context.Context, snapshotIDs []string) (int, error) {
		return reconcileFixtureSnapshots(ctx, deps.Execute, snapshotIDs)
	}
	coordinator, err := fixturecontract.NewCoordinator(fixturecontract.TransactionDeps{
		NewCorrelationID:    deps.NewCorrelationID,
		NewTransactionID:    deps.NewTransactionID,
		ExtensionGeneration: deps.ExtensionGeneration,
		Now:                 deps.Now,
		Registry:            deps.Registry,
		Persist:             deps.Persist,
		OnNotice:            deps.OnNotice,
		Snapshot: func(ctx context.Context, fixture fixturecontract.WireQAFixture) (string, error) {
			return snapshotEnvironmentFixture(ctx, deps.Execute, fixture)
		},
		Apply: func(ctx context.Context, fixture fixturecontract.WireQAFixture) (fixturecontract.MutationCounts, error) {
			return applyEnvironmentFixture(ctx, deps.Execute, fixture)
		},
		Restore: restore,
	})
	if err != nil {
		return nil, err
	}
	return &Handler{
		ctx: deps.Context, coordinator: coordinator, registry: deps.Registry,
		persist: deps.Persist, generation: deps.ExtensionGeneration, restore: restore, reconcile: reconcile,
		diagnostics: deps.Diagnostics,
	}, nil
}

func (handler *Handler) Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		FixtureAction string          `json:"fixture_action"`
		Fixture       json.RawMessage `json:"fixture"`
		TransactionID string          `json:"transaction_id"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if params.FixtureAction == "status" {
		return mcp.Succeed(req, "QA fixture transaction status", map[string]any{"transactions": handler.transactionSummaries()})
	}
	if params.FixtureAction == "restore" {
		if params.TransactionID == "" {
			return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'transaction_id' is missing", "Pass the transaction_id returned by apply", mcp.WithParam("transaction_id"))
		}
		alreadyRestored, err := handler.restoreTransaction(handler.ctx, params.TransactionID)
		if err != nil {
			return mcp.Fail(req, mcp.ErrExtError, "QA fixture restore failed: "+err.Error(), "Inspect configure({what:'doctor'}) and retry restore.")
		}
		return mcp.Succeed(req, "QA fixture restored", map[string]any{"transaction_id": params.TransactionID, "restored": true, "already_restored": alreadyRestored})
	}
	if params.FixtureAction != "validate" && params.FixtureAction != "apply" {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid fixture_action", "Use fixture_action='validate', 'apply', 'status', or 'restore'", mcp.WithParam("fixture_action"))
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
	result, err := handler.applyFixture(fixture)
	handler.reportApplyLifecycle(result, err)
	if err != nil {
		return mcp.Fail(req, mcp.ErrExtError, "QA fixture transaction failed: "+result.Status,
			"Inspect configure({what:'doctor'}), correct browser state, and retry the fixture.",
			mcp.WithHint("correlation_id="+result.CorrelationID))
	}
	return mcp.Succeed(req, "QA fixture applied", result)
}

func (handler *Handler) applyFixture(fixture fixturecontract.WireQAFixture) (fixturecontract.TransactionResult, error) {
	if err := handler.waitForStartupRecovery(handler.ctx); err != nil {
		return fixturecontract.TransactionResult{Status: fixturecontract.StatusCanceled}, err
	}
	handler.lifecycleMu.Lock()
	defer handler.lifecycleMu.Unlock()
	return handler.coordinator.Apply(handler.ctx, fixture)
}

func (handler *Handler) RecoverPending(ctx context.Context) []string {
	failures := make([]string, 0)
	for _, record := range handler.registry.Records() {
		if _, err := handler.restoreTransaction(ctx, record.TransactionID); err != nil {
			failures = append(failures, err.Error())
		}
	}
	return failures
}

func (handler *Handler) restoreTransaction(ctx context.Context, transactionID string) (alreadyRestored bool, returnErr error) {
	if err := handler.waitForStartupRecovery(ctx); err != nil {
		return false, err
	}
	handler.lifecycleMu.Lock()
	defer handler.lifecycleMu.Unlock()
	return handler.restoreTransactionLocked(ctx, transactionID)
}

func (handler *Handler) restoreTransactionLocked(ctx context.Context, transactionID string) (alreadyRestored bool, returnErr error) {
	record, exists := handler.registry.Get(transactionID)
	if !exists {
		return true, nil
	}
	defer func() {
		if returnErr != nil {
			handler.diagnostics.Report(statediag.Diagnostic{
				Name: fixtureDiagnosticName(record.CorrelationID), CorrelationID: record.CorrelationID,
				Detail: "QA fixture restoration failed with status " + returnErr.Error() + ".",
				Fix:    "Reconnect the extension, inspect transaction status, and retry restore.",
			})
		}
	}()
	record, err := handler.registry.BeginRestore(transactionID, handler.generation())
	if err != nil {
		return false, err
	}
	if err := handler.persist(handler.registry); err != nil {
		if restoreStateErr := handler.registry.RestoreFailed(transactionID); restoreStateErr != nil {
			return false, errors.New("fixture_transaction_registry_recovery_failed")
		}
		return false, errors.New("fixture_transaction_registry_persist_failed")
	}
	restoreCtx, cancel := context.WithTimeout(ctx, time.Duration(fixturecontract.DefaultSetupTimeoutMs)*time.Millisecond)
	defer cancel()
	if err := handler.restore(restoreCtx, record.SnapshotID); err != nil {
		if restoreStateErr := handler.registry.RestoreFailed(transactionID); restoreStateErr != nil {
			return false, errors.New("fixture_transaction_registry_recovery_failed")
		}
		if persistErr := handler.persist(handler.registry); persistErr != nil {
			return false, errors.New("fixture_transaction_restore_and_persist_failed")
		}
		return false, errors.New("fixture_transaction_restore_failed")
	}
	if err := handler.registry.CompleteRestore(transactionID); err != nil {
		return false, err
	}
	if err := handler.persist(handler.registry); err != nil {
		record.State = fixturecontract.TransactionRestoreRequired
		if addErr := handler.registry.Add(record); addErr != nil {
			return false, errors.New("fixture_transaction_registry_recovery_failed")
		}
		return false, errors.New("fixture_transaction_registry_persist_failed")
	}
	handler.diagnostics.Resolve(fixtureDiagnosticName(record.CorrelationID))
	return false, nil
}

func (handler *Handler) reportApplyLifecycle(result fixturecontract.TransactionResult, err error) {
	if result.CorrelationID == "" {
		return
	}
	name := fixtureDiagnosticName(result.CorrelationID)
	detail := "QA fixture transaction requires browser state restoration."
	fix := "Restore the transaction through configure({what:'qa_fixture',fixture_action:'restore'})."
	if err != nil {
		detail = "QA fixture transaction ended with status " + result.Status + "."
		fix = "Inspect fixture transaction status and retry after correcting browser state."
	}
	handler.diagnostics.Report(statediag.Diagnostic{
		Name: name, CorrelationID: result.CorrelationID, Detail: detail, Fix: fix,
	})
	if err != nil && (result.RolledBack || noMutationFailure(result.Status)) {
		handler.diagnostics.Resolve(name)
	}
}

func noMutationFailure(status string) bool {
	return status == fixturecontract.StatusSnapshotFailed || status == fixturecontract.StatusTimedOut || status == fixturecontract.StatusCanceled
}

func (handler *Handler) reportPendingRecovery(correlationID, detail string) {
	handler.diagnostics.Report(statediag.Diagnostic{
		Name: fixtureDiagnosticName(correlationID), CorrelationID: correlationID, Detail: detail,
		Fix: "Reconnect the extension and retry fixture restoration.",
	})
}

func fixtureDiagnosticName(correlationID string) string {
	return "fixture_transaction_" + correlationID
}

func (handler *Handler) transactionSummaries() []map[string]any {
	records := handler.registry.Records()
	summaries := make([]map[string]any, 0, len(records))
	for _, record := range records {
		summaries = append(summaries, map[string]any{
			"transaction_id": record.TransactionID, "correlation_id": record.CorrelationID,
			"state": record.State, "created_at": record.CreatedAt, "mutations": record.Mutations,
		})
	}
	return summaries
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

func restoreEnvironmentSnapshot(ctx context.Context, execute CommandExecutor, snapshotID string) error {
	if snapshotID == "" {
		return errors.New("invalid_fixture_snapshot_reference")
	}
	result, err := executeFixtureCommand(ctx, execute, "environment_transaction_restore", nil, snapshotID, fixturecontract.DefaultSetupTimeoutMs)
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
}

func reconcileFixtureSnapshots(ctx context.Context, execute CommandExecutor, snapshotIDs []string) (int, error) {
	payload, err := json.Marshal(struct {
		SnapshotIDs []string `json:"snapshot_ids"`
	}{SnapshotIDs: snapshotIDs})
	if err != nil {
		return 0, errors.New("fixture_reconcile_encoding_failed")
	}
	result, err := execute(ctx, "environment_transaction_reconcile", payload, time.Duration(fixturecontract.DefaultSetupTimeoutMs)*time.Millisecond)
	if err != nil {
		return 0, errors.New("fixture_reconcile_failed")
	}
	var reconciled struct {
		Success  bool `json:"success"`
		Pruned   int  `json:"pruned"`
		Retained int  `json:"retained"`
	}
	if json.Unmarshal(result, &reconciled) != nil || !reconciled.Success || reconciled.Pruned < 0 || reconciled.Retained < 0 || reconciled.Retained > len(snapshotIDs) {
		return 0, errors.New("invalid_fixture_reconcile_result")
	}
	return reconciled.Pruned, nil
}

func snapshotEnvironmentFixture(ctx context.Context, execute CommandExecutor, fixture fixturecontract.WireQAFixture) (string, error) {
	result, err := executeFixtureCommand(ctx, execute, "environment_transaction_snapshot", &fixture, "", fixture.SetupTimeoutMs)
	if err != nil {
		return "", err
	}
	var snapshot struct {
		Success    bool   `json:"success"`
		SnapshotID string `json:"snapshot_id"`
	}
	if json.Unmarshal(result, &snapshot) != nil || !snapshot.Success || snapshot.SnapshotID == "" {
		return "", errors.New("invalid_fixture_snapshot_result")
	}
	return snapshot.SnapshotID, nil
}

func applyEnvironmentFixture(ctx context.Context, execute CommandExecutor, fixture fixturecontract.WireQAFixture) (fixturecontract.MutationCounts, error) {
	result, err := executeFixtureCommand(ctx, execute, "environment_transaction_apply", &fixture, "", fixture.SetupTimeoutMs)
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

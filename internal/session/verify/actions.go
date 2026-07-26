// Purpose: Implements verify_fix session actions (start/watch/compare/cancel/status).
// Why: Separates action orchestration from shared types and snapshot helper logic.
// Docs: docs/features/feature/request-session-correlation/index.md

package verify

import (
	"fmt"
	"time"
)

func (vm *VerificationManager) getActiveSessionLocked(sessionID string) (*VerificationSession, error) {
	session, exists := vm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}
	if time.Since(session.CreatedAt) > vm.ttl {
		delete(vm.sessions, sessionID)
		vm.removeFromOrder(sessionID)
		return nil, fmt.Errorf("session %q has expired", sessionID)
	}
	return session, nil
}

// ============================================
// Start Action
// ============================================

// Start captures the baseline (broken) state.
func (vm *VerificationManager) Start(label, urlFilter string) (*StartResult, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Clean up expired sessions first.
	vm.cleanupExpiredLocked()

	// Check session limit.
	if len(vm.sessions) >= maxVerificationSessions {
		return nil, fmt.Errorf("maximum concurrent verification sessions (%d) reached", maxVerificationSessions)
	}

	// Generate session ID.
	vm.idSeq++
	sessionID := fmt.Sprintf("verify-%d-%d", time.Now().Unix(), vm.idSeq)

	// Capture current state as baseline.
	baseline := vm.captureSnapshot(urlFilter)

	session := &VerificationSession{
		ID:        sessionID,
		Label:     label,
		Status:    "baseline_captured",
		URLFilter: urlFilter,
		CreatedAt: time.Now(),
		Baseline:  baseline,
	}

	vm.sessions[sessionID] = session
	vm.order = append(vm.order, sessionID)

	// Build response.
	result := &StartResult{
		VerifSessionID: sessionID,
		Label:          label,
		Status:         "baseline_captured",
		Baseline:       vm.buildBaselineSummary(baseline),
	}

	return result, nil
}

// ============================================
// Watch Action
// ============================================

// Watch begins monitoring for new activity.
func (vm *VerificationManager) Watch(sessionID string) (*WatchResult, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	session, err := vm.getActiveSessionLocked(sessionID)
	if err != nil {
		return nil, err
	}

	// Check status.
	if session.Status == "cancelled" {
		return nil, fmt.Errorf("session %q has been cancelled", sessionID)
	}

	// Set watch state (idempotent).
	now := time.Now()
	session.WatchStartedAt = &now
	session.Status = "watching"

	return &WatchResult{
		VerifSessionID: sessionID,
		Status:         "watching",
		Message:        "Monitoring started. Ask the user to reproduce the scenario.",
	}, nil
}

// ============================================
// Compare Action
// ============================================

// Compare diffs baseline vs current state.
func (vm *VerificationManager) Compare(sessionID string) (*CompareResult, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	session, err := vm.getActiveSessionLocked(sessionID)
	if err != nil {
		return nil, err
	}

	// Must have called watch first.
	if session.WatchStartedAt == nil {
		return nil, fmt.Errorf("session %q: must call 'watch' before 'compare'", sessionID)
	}

	// Capture current state as "after".
	after := vm.captureSnapshot(session.URLFilter)
	session.After = after
	session.Status = "compared"

	// Compute diff.
	result := vm.computeVerification(session.Baseline, after)

	return &CompareResult{
		VerifSessionID: sessionID,
		Status:         "compared",
		Label:          session.Label,
		Result:         result,
	}, nil
}

// ============================================
// Cancel Action
// ============================================

// Cancel discards a verification session.
func (vm *VerificationManager) Cancel(sessionID string) (*CancelResult, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	session, exists := vm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	session.Status = "cancelled"
	delete(vm.sessions, sessionID)
	vm.removeFromOrder(sessionID)

	return &CancelResult{
		VerifSessionID: sessionID,
		Status:         "cancelled",
	}, nil
}

// ============================================
// Status Action
// ============================================

// Status returns the current state of a session.
func (vm *VerificationManager) Status(sessionID string) (*StatusResult, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	session, exists := vm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	return &StatusResult{
		VerifSessionID: sessionID,
		Label:          session.Label,
		Status:         session.Status,
		CreatedAt:      session.CreatedAt,
	}, nil
}

// Purpose: Manages verification session ordering and TTL-based cleanup behavior.
// Why: Keeps session-lifecycle housekeeping logic focused and reusable across actions.
// Docs: docs/features/feature/request-session-correlation/index.md

package verify

import "time"

// removeFromOrder removes a session ID from the order slice.
func (vm *VerificationManager) removeFromOrder(sessionID string) {
	for i, id := range vm.order {
		if id == sessionID {
			newOrder := make([]string, len(vm.order)-1)
			copy(newOrder, vm.order[:i])
			copy(newOrder[i:], vm.order[i+1:])
			vm.order = newOrder
			return
		}
	}
}

// cleanupExpiredLocked removes expired sessions (must hold lock).
func (vm *VerificationManager) cleanupExpiredLocked() {
	now := time.Now()
	expired := make([]string, 0)

	for id, session := range vm.sessions {
		if now.Sub(session.CreatedAt) > vm.ttl {
			expired = append(expired, id)
		}
	}

	for _, id := range expired {
		delete(vm.sessions, id)
		vm.removeFromOrder(id)
	}
}

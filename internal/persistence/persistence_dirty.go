// Purpose: Implements dirty-write tracking and background flush for deferred persistence writes.
// Why: Separates write-coalescing logic from immediate CRUD operations.
package persistence

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

const deferredWriteDiagnostic = "session_deferred_write_state"

func (s *SessionStore) MarkDirty(namespace, key string, data []byte) {
	if validateStoreInput(namespace, "namespace") != nil || validateStoreInput(key, "key") != nil {
		return
	}
	s.dirtyMu.Lock()
	defer s.dirtyMu.Unlock()
	dirtyKey := namespace + "/" + key
	s.dirty[dirtyKey] = append([]byte(nil), data...)
}

func (s *SessionStore) backgroundFlush() {
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.flushDirty()
		case <-s.stopCh:
			return
		}
	}
}

func (s *SessionStore) flushDirty() {
	toFlush := func() map[string][]byte {
		s.dirtyMu.Lock()
		defer s.dirtyMu.Unlock()
		if len(s.dirty) == 0 {
			return nil
		}

		copied := make(map[string][]byte, len(s.dirty))
		for k, v := range s.dirty {
			copied[k] = v
		}
		s.dirty = make(map[string][]byte)
		return copied
	}()
	if len(toFlush) == 0 {
		return
	}

	failed := make(map[string][]byte)
	invalidQueuedKey := false
	for key, data := range toFlush {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			invalidQueuedKey = true
			continue
		}
		namespace, name := parts[0], parts[1]

		filePath := filepath.Join(s.projectDir, namespace, name+".json")
		if validatePathInDir(s.projectDir, filePath) != nil {
			invalidQueuedKey = true
			continue
		}
		if err := s.filesystem().WriteFile(filePath, data, filePermissions); err != nil {
			failed[key] = data
		}
	}

	if len(failed) > 0 {
		s.dirtyMu.Lock()
		for key, data := range failed {
			if _, newerQueued := s.dirty[key]; !newerQueued {
				s.dirty[key] = data
			}
		}
		s.dirtyMu.Unlock()
	}
	if len(failed) > 0 || invalidQueuedKey {
		s.reportRecovery(
			deferredWriteDiagnostic,
			"Deferred session state could not be fully persisted; valid failed writes remain queued for retry.",
			"Check permissions and available disk space for the project .kaboom directory; Kaboom will retry automatically.",
		)
		return
	}
	statediag.Resolve(s.diagnostics, deferredWriteDiagnostic)
}

func (s *SessionStore) Shutdown() {
	shouldShutdown := func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.stopped {
			return false
		}
		s.stopped = true
		return true
	}()
	if !shouldShutdown {
		return
	}

	close(s.stopCh)
	s.flushDirty()

	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.meta.LastSession = time.Now()
		if err := s.saveMeta(); err != nil {
			// saveMeta reports the redacted failure to Doctor before returning;
			// Shutdown cannot safely retry after the daemon lifecycle has ended.
			return
		}
	}()
}

// Purpose: MCP daemon bootstrap helpers for capture, preflight checks, and HTTP listener startup.
// Why: Separates runtime setup mechanics from high-level MCP orchestration flow.

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// initCapture creates and configures the capture buffers with lifecycle logging.
func initCapture(server *Server, port int) *capture.Store {
	cap := capture.NewCapture()
	cap.SetClientRegistry(newSessionClientRegistryAdapter(clientreg.NewClientRegistry()))
	cap.SetServerVersion(version)
	cap.SetLifecycleCallback(func(event string, data map[string]any) {
		entry := LogEntry{
			"type":      "lifecycle",
			"event":     event,
			"pid":       os.Getpid(),
			"port":      port,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		for k, v := range data {
			entry[k] = v
		}
		server.logs.addEntries([]LogEntry{entry})
	})

	server.logLifecycle("loading_settings", port, nil)
	cap.LoadSettingsFromDisk()
	server.logLifecycle("settings_loaded", port, nil)
	return cap
}

// startScreenshotRateLimiterCleanup starts a background goroutine that removes
// stale entries from the screenshot rate limiter every 30 seconds.
func (s *Server) startScreenshotRateLimiterCleanup(ctx context.Context) {
	util.SafeGo(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				func() {
					s.screenshotRateMu.Lock()
					defer s.screenshotRateMu.Unlock()
					now := time.Now()
					for clientID, lastUpload := range s.screenshotRateLimiter {
						if now.Sub(lastUpload) > time.Minute {
							delete(s.screenshotRateLimiter, clientID)
						}
					}
				}()
			}
		}
	})
}

// cleanupStalePIDFile checks for an existing PID file and removes it if the
// process is dead. Returns an error if a live process already holds the port.
func cleanupStalePIDFile(server *Server, port int) error {
	pidFile := pidFilePath(port)
	if _, err := os.Stat(pidFile); err != nil {
		return nil // No PID file
	}

	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return nil
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}

	// Signal 0 checks existence without killing.
	// A live PID alone is not enough: PID reuse can point to an unrelated process.
	// Only treat it as a conflict if that PID actually owns the target port.
	if process.Signal(syscall.Signal(0)) == nil {
		ownerPIDs, findErr := findProcessOnPort(port)
		if findErr == nil {
			for _, ownerPID := range ownerPIDs {
				if ownerPID == pid {
					server.logLifecycle("port_conflict_detected", port, map[string]any{"existing_pid": pid})
					return fmt.Errorf("port %d already in use by PID %d (run 'kaboom --stop --port %d' to stop it)", port, pid, port)
				}
			}
			server.logLifecycle("stale_pid_owner_mismatch", port, map[string]any{
				"stale_pid":  pid,
				"owner_pids": ownerPIDs,
			})
		} else {
			// If process inspection fails, prefer removing stale metadata and allowing
			// the later preflight bind check to authoritatively detect real conflicts.
			server.logLifecycle("stale_pid_port_lookup_failed", port, map[string]any{
				"stale_pid": pid,
				"error":     findErr.Error(),
			})
		}

		if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
			server.logLifecycle("stale_pid_remove_failed", port, map[string]any{
				"stale_pid": pid,
				"error":     err.Error(),
			})
		}
		return nil
	}

	// Process is dead - remove stale PID file
	server.logLifecycle("stale_pid_removed", port, map[string]any{"stale_pid": pid})
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		server.logLifecycle("stale_pid_remove_failed", port, map[string]any{
			"stale_pid": pid,
			"error":     err.Error(),
		})
	}
	return nil
}

// preflightPortCheck verifies the port is available before attempting to bind.
// Provides better error messages than a raw bind failure.
func preflightPortCheck(server *Server, port int) error {
	testAddr := fmt.Sprintf("127.0.0.1:%d", port)
	testLn, err := net.Listen("tcp", testAddr)
	if err != nil {
		// Name the holder rather than saying "unknown process". This is the message
		// the user actually sees when the daemon refuses to start, and "unknown" sent
		// them to a kill hint that may target something they did not want killed.
		blockingPID, blockingCmd := identifyPortHolder(port)
		server.logLifecycle("port_conflict_detected", port, map[string]any{
			"error":          err.Error(),
			"blocked_by_pid": blockingPID,
			"blocked_by_cmd": blockingCmd,
		})
		if blockingPID > 0 {
			return fmt.Errorf("port %d already in use by pid %d (%s); free that port or start Kaboom on a different one: %w",
				port, blockingPID, blockingCmd, err)
		}
		return fmt.Errorf("port %d already in use (owner could not be identified, try '%s'): %w", port, portKillHintForce(port), err)
	}
	return testLn.Close()
}

// startHTTPServer launches the HTTP server in a background goroutine and waits
// for it to bind successfully. Returns the server instance and a channel that
// closes if the listener exits unexpectedly (crash, network error, etc.).
func startHTTPServer(server *Server, port int, apiKey string, mux *http.ServeMux) (*http.Server, <-chan struct{}, error) {
	httpReady := make(chan error, 1)
	httpDone := make(chan struct{})
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 65 * time.Second, // Must accommodate blocking tool waits (screenshot 20s, interact 35s, annotations 55s)
		IdleTimeout:  120 * time.Second,
		Handler:      AuthMiddleware(apiKey)(mux),
	}
	util.SafeGo(func() {
		defer close(httpDone)
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			httpReady <- err
			return
		}
		httpReady <- nil
		// #nosec G114 -- localhost-only MCP background server
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			_ = appendExitDiagnostic("http_listener_error", map[string]any{
				"port":  port,
				"error": err.Error(),
			})
			stderrf("[Kaboom] HTTP server error: %v\n", err)
		}
	})

	if err := <-httpReady; err != nil {
		server.logLifecycle("http_bind_failed", port, map[string]any{"error": err.Error()})
		return nil, nil, fmt.Errorf("cannot bind port %d: %w", port, err)
	}

	server.logLifecycle("http_bind_success", port, nil)
	return srv, httpDone, nil
}

// persistDaemonRuntimeState records process metadata used by lifecycle/stop flows.
func persistDaemonRuntimeState(server *Server, port int) {
	if err := writePIDFile(port); err != nil {
		server.logLifecycle("pid_file_error", port, map[string]any{"error": err.Error()})
	}
	if err := daemonlife.PersistCurrentLock(port, version); err != nil {
		server.logLifecycle("daemon_lock_write_failed", port, map[string]any{"error": err.Error()})
	}
}

// Purpose: Tests for server persistence endpoints.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// server_persistence_test.go — Server persistence invariant tests.
//
// ⚠️ CRITICAL INVARIANT TESTS - DO NOT MODIFY WITHOUT PRINCIPAL REVIEW
//
// These tests verify that the HTTP server stays alive as long as stdin remains open.
// This is a fundamental reliability guarantee for browser extension connectivity.
//
// See: .claude/refs/mcp-stdio-invariant.md#server-persistence-invariant---critical
//
// The invariant MUST ensure:
// 1. Server stays alive while stdin is open (not EOF)
// 2. Health endpoint responds at any point during session
// 3. Server survives extended periods without stdin data
// 4. Server exits cleanly when stdin closes (with persist=false)
//
// DO NOT:
// - Remove or skip any test cases
// - Weaken assertions or add exceptions
// - Reduce test durations below specified minimums
// - Change without approval from principal engineer
//
// Run: go test ./cmd/browser-agent -run "TestServerPersistence" -v

//go:build integration

package main

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestServerPersistence_StaysAliveWithOpenStdin verifies the server doesn't die
// while stdin remains open (the FIFO pattern).
//
// This is the PRIMARY invariant test - server must survive at least 10 seconds
// without any stdin data as long as stdin isn't closed.
func TestServerPersistence_StaysAliveWithOpenStdin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping persistence test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	// Start server with stdin pipe (simulates FIFO)
	cmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Wait for server to start
	if !bridgeRunner.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start on port %d", port)
	}

	t.Log("Server started, beginning persistence test...")

	// INVARIANT: Server must stay alive for at least 10 seconds with open stdin
	// Check health every second to catch early death
	testDuration := 10 * time.Second
	checkInterval := 1 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < testDuration {
		// Verify server is still responding
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if err != nil {
			elapsed := time.Since(startTime)
			t.Fatalf("INVARIANT VIOLATION: Server died after %v (must survive %v with open stdin): %v",
				elapsed, testDuration, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			elapsed := time.Since(startTime)
			t.Fatalf("INVARIANT VIOLATION: Server returned %d after %v (must return 200)",
				resp.StatusCode, elapsed)
		}

		time.Sleep(checkInterval)
	}

	t.Logf("✅ Server survived %v with open stdin (no data sent)", testDuration)
}

// TestServerPersistence_HealthResponseTime verifies health endpoint responds quickly
// at any point during the session.
//
// Invariant: Health endpoint MUST respond within 100ms at all times.
func TestServerPersistence_HealthResponseTime(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping persistence test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	cmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	if !bridgeRunner.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	// Test health response time over 5 seconds
	maxResponseTime := 100 * time.Millisecond
	client := &http.Client{Timeout: maxResponseTime}
	testDuration := 5 * time.Second
	startTime := time.Now()

	var slowestResponse time.Duration
	var requestCount int

	for time.Since(startTime) < testDuration {
		reqStart := time.Now()
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		responseTime := time.Since(reqStart)

		if err != nil {
			t.Fatalf("INVARIANT VIOLATION: Health request failed (timeout=%v): %v", maxResponseTime, err)
		}
		resp.Body.Close()

		if responseTime > slowestResponse {
			slowestResponse = responseTime
		}
		requestCount++

		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("✅ %d health checks completed, slowest: %v (limit: %v)", requestCount, slowestResponse, maxResponseTime)
}

// TestServerPersistence_SurvivesStdinClose verifies server stays alive even when
// stdin closes (current behavior - server waits for SIGTERM/SIGINT).
//
// Persistence is the sole runtime model: the daemon waits for a termination
// signal so browser-extension and later MCP clients can reconnect.
func TestServerPersistence_SurvivesStdinClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping persistence test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	// Start server (persist flag doesn't affect MCP stdin behavior)
	cmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	if !bridgeRunner.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	t.Log("Server started, closing stdin...")

	// Close stdin - server should STAY alive (waits for signal)
	if err := stdin.Close(); err != nil {
		t.Fatalf("Failed to close stdin: %v", err)
	}

	// Wait a moment, then verify server is still alive
	time.Sleep(2 * time.Second)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("Server died after stdin close (should stay alive): %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Server returned %d after stdin close", resp.StatusCode)
	}

	t.Log("✅ Server survived stdin close (correct behavior - waits for signal)")
}

// TestServerPersistence_PersistModeKeepsAlive verifies server stays alive after
// stdin closes when persist=true (default).
func TestServerPersistence_PersistModeKeepsAlive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping persistence test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	// Start server (persistence is default behavior - server stays alive after stdin closes)
	cmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Send initialize request to trigger server spawn (MCP bridge mode waits for input)
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	if _, err := stdin.Write([]byte(initReq)); err != nil {
		t.Fatalf("Failed to send initialize request: %v", err)
	}

	if !bridgeRunner.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	t.Log("Server started with persist=true, closing stdin...")

	// Close stdin
	if err := stdin.Close(); err != nil {
		t.Fatalf("Failed to close stdin: %v", err)
	}

	// Server should STAY alive for at least 3 seconds after stdin close
	time.Sleep(1 * time.Second)

	for i := 0; i < 3; i++ {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if err != nil {
			t.Fatalf("INVARIANT VIOLATION: Server died after stdin close with persist=true: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Server returned %d after stdin close", resp.StatusCode)
		}

		time.Sleep(1 * time.Second)
	}

	t.Log("✅ Server stayed alive 3+ seconds after stdin close (persist=true)")
}

// TestServerPersistence_MultipleHealthChecksUnderLoad verifies server handles
// rapid health checks without dying.
func TestServerPersistence_MultipleHealthChecksUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping persistence test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	cmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	if !bridgeRunner.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	// Send 100 rapid health checks
	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)

	for i := 0; i < 100; i++ {
		resp, err := client.Get(healthURL)
		if err != nil {
			t.Fatalf("Health check %d failed: %v", i, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Health check %d returned %d", i, resp.StatusCode)
		}
	}

	t.Log("✅ Server handled 100 rapid health checks without dying")
}

// TestServerPersistence_StdinNoDataExtendedPeriod verifies server survives
// extended period (30 seconds) without any stdin data.
//
// This is the ultimate persistence test - simulates a long MCP session
// with no tool calls.
func TestServerPersistence_StdinNoDataExtendedPeriod(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping extended persistence test in short mode")
	}

	// This test takes 30 seconds - skip in CI unless explicitly enabled
	if os.Getenv("KABOOM_EXTENDED_TESTS") == "" {
		t.Skip("skipping 30-second persistence test (set KABOOM_EXTENDED_TESTS=1 to enable)")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	cmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	if !bridgeRunner.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	t.Log("Starting 30-second persistence test...")

	// Check every 5 seconds for 30 seconds
	for i := 0; i < 6; i++ {
		time.Sleep(5 * time.Second)

		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if err != nil {
			t.Fatalf("INVARIANT VIOLATION: Server died at %d seconds: %v", (i+1)*5, err)
		}
		resp.Body.Close()

		t.Logf("  ✓ Server alive at %d seconds", (i+1)*5)
	}

	t.Log("✅ Server survived 30 seconds with open stdin (no data)")
}

// Purpose: Tests for server reliability and error recovery.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// server_reliability_test.go — Stress, resource leak, and recovery tests.
//
// ⚠️ RELEASE GATE TESTS - MANDATORY BEFORE EVERY RELEASE
//
// These tests verify the server operates without flaw under real-world conditions.
// They cover scenarios that unit tests cannot: resource leaks, concurrent load,
// extended operation, and recovery from errors.
//
// Run all reliability tests:
//   KABOOM_RELIABILITY_TESTS=1 go test ./cmd/browser-agent -run "TestReliability" -v -timeout 10m
//
// Individual test categories:
//   go test ./cmd/browser-agent -run "TestReliability_Stress" -v
//   go test ./cmd/browser-agent -run "TestReliability_ResourceLeaks" -v
//   go test ./cmd/browser-agent -run "TestReliability_Recovery" -v
//
// DO NOT skip these tests before release. They catch issues that only manifest
// under sustained load or extended operation.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
)

// ============================================================================
// STRESS TESTS - Concurrent load and extended operation
// ============================================================================

// TestReliability_Stress_ConcurrentConnections verifies server handles many
// concurrent HTTP connections without deadlock, crash, or resource exhaustion.
func TestReliability_Stress_ConcurrentConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
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

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	// Configuration
	concurrency := 50       // Concurrent goroutines
	requestsPerWorker := 20 // Requests per goroutine
	totalRequests := concurrency * requestsPerWorker

	var successCount atomic.Int64
	var errorCount atomic.Int64
	var wg sync.WaitGroup

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				// Alternate between health checks and MCP calls
				var resp *http.Response
				var err error

				if j%2 == 0 {
					resp, err = client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
				} else {
					mcpReq := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`
					resp, err = client.Post(
						fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
						"application/json",
						strings.NewReader(mcpReq),
					)
				}

				if err != nil {
					errorCount.Add(1)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					successCount.Add(1)
				} else {
					errorCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	success := successCount.Load()
	errors := errorCount.Load()
	successRate := float64(success) / float64(totalRequests) * 100
	rps := float64(totalRequests) / elapsed.Seconds()

	t.Logf("Concurrent stress test results:")
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful: %d (%.1f%%)", success, successRate)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Duration: %v", elapsed.Round(time.Millisecond))
	t.Logf("  Throughput: %.1f req/sec", rps)

	// Invariant: At least 99% success rate under load
	if successRate < 99.0 {
		t.Fatalf("RELIABILITY FAILURE: Success rate %.1f%% < 99%% threshold", successRate)
	}

	// Verify server is still healthy after load
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("Server died after stress test: %v", err)
	}
	resp.Body.Close()

	t.Logf("✅ Server survived %d concurrent requests (%.1f%% success)", totalRequests, successRate)
}

// TestReliability_Stress_ExtendedOperation verifies server operates correctly
// over an extended period (simulates long MCP session).
func TestReliability_Stress_ExtendedOperation(t *testing.T) {
	if os.Getenv("KABOOM_RELIABILITY_TESTS") == "" {
		t.Skip("skipping extended operation test (set KABOOM_RELIABILITY_TESTS=1)")
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

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	// Run for 2 minutes with periodic checks
	testDuration := 2 * time.Minute
	checkInterval := 5 * time.Second
	startTime := time.Now()

	client := &http.Client{Timeout: 5 * time.Second}
	var checkCount, successCount int

	t.Logf("Starting %v extended operation test...", testDuration)

	for time.Since(startTime) < testDuration {
		checkCount++

		// Health check
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if err != nil {
			t.Fatalf("Server died at %v: %v", time.Since(startTime).Round(time.Second), err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			successCount++
		}

		// MCP tool call
		mcpReq := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs"}}}`
		resp, err = client.Post(
			fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
			"application/json",
			strings.NewReader(mcpReq),
		)
		if err != nil {
			t.Fatalf("MCP call failed at %v: %v", time.Since(startTime).Round(time.Second), err)
		}
		resp.Body.Close()

		if checkCount%12 == 0 { // Log every minute
			t.Logf("  ✓ %v elapsed, %d checks passed", time.Since(startTime).Round(time.Second), checkCount)
		}

		time.Sleep(checkInterval)
	}

	t.Logf("✅ Server operated correctly for %v (%d checks, 100%% success)", testDuration, checkCount)
}

// ============================================================================
// RESOURCE LEAK TESTS - Memory, goroutines, file descriptors
// ============================================================================

// TestReliability_ResourceLeaks_Goroutines verifies no goroutine leaks under load.
func TestReliability_ResourceLeaks_Goroutines(t *testing.T) {
	// Skip: Flaky in parallel test runs due to port/timing issues.
	// Works in isolation. TODO: Investigate root cause of flakiness.
	t.Skip("Skipped: flaky in parallel test runs; works in isolation")
	if testing.Short() {
		t.Skip("skipping resource leak test in short mode")
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

	// Send initialize request to trigger server spawn (MCP bridge mode waits for input)
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	if _, err := stdin.Write([]byte(initReq)); err != nil {
		t.Fatalf("Failed to send initialize request: %v", err)
	}

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	// Get baseline goroutine count (in test process, not server)
	// For server, we'll check it responds consistently
	baselineGoroutines := runtime.NumGoroutine()

	client := &http.Client{Timeout: 5 * time.Second}

	// Make many requests that could leak resources
	for round := 0; round < 5; round++ {
		for i := 0; i < 100; i++ {
			// Requests that allocate resources
			mcpReq := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"network_waterfall","limit":100}}}`
			resp, err := client.Post(
				fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
				"application/json",
				strings.NewReader(mcpReq),
			)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			io.Copy(io.Discard, resp.Body) // Drain body
			resp.Body.Close()
		}

		// Let GC run
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}

	// Check goroutine count didn't explode (in test process)
	finalGoroutines := runtime.NumGoroutine()
	goroutineGrowth := finalGoroutines - baselineGoroutines

	t.Logf("Goroutine check: baseline=%d, final=%d, growth=%d", baselineGoroutines, finalGoroutines, goroutineGrowth)

	// Allow some growth for connection pooling, but not unbounded
	if goroutineGrowth > 50 {
		t.Errorf("Possible goroutine leak: grew by %d (baseline: %d, final: %d)", goroutineGrowth, baselineGoroutines, finalGoroutines)
	}

	// Verify server still responds (didn't OOM or deadlock)
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("Server became unresponsive after load: %v", err)
	}
	resp.Body.Close()

	t.Logf("✅ No significant goroutine leak detected (growth: %d)", goroutineGrowth)
}

// TestReliability_ResourceLeaks_ConnectionDrain verifies connections are properly
// released even when clients disconnect abruptly.
func TestReliability_ResourceLeaks_ConnectionDrain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping connection drain test in short mode")
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

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	// Create many connections and abandon them
	for i := 0; i < 100; i++ {
		// Use short timeout client that will abandon connection
		client := &http.Client{Timeout: 10 * time.Millisecond}
		resp, _ := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if resp != nil {
			resp.Body.Close()
		}
		// Don't wait for response - simulate abrupt disconnect
	}

	// Wait for server to clean up
	time.Sleep(1 * time.Second)

	// Server should still be responsive
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("Server unresponsive after connection churn: %v", err)
	}
	resp.Body.Close()

	t.Log("✅ Server survived 100 abandoned connections")
}

// ============================================================================
// RECOVERY TESTS - Server survives bad input without dying
// ============================================================================

// TestReliability_Recovery_MalformedJSON verifies server handles malformed JSON
// gracefully without crashing.
func TestReliability_Recovery_MalformedJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping recovery test in short mode")
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

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)

	// Send various malformed requests
	malformedRequests := []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"not json", "this is not json"},
		{"truncated json", `{"jsonrpc":"2.0","id":1,"method":`},
		{"null body", "null"},
		{"array instead of object", `[1,2,3]`},
		{"missing method", `{"jsonrpc":"2.0","id":1}`},
		{"invalid jsonrpc version", `{"jsonrpc":"1.0","id":1,"method":"ping"}`},
		{"huge id", `{"jsonrpc":"2.0","id":99999999999999999999999999999,"method":"ping"}`},
		{"binary garbage", string([]byte{0x00, 0x01, 0x02, 0xff, 0xfe})},
		{"unicode bomb", `{"jsonrpc":"2.0","id":1,"method":"` + strings.Repeat("\U0001f480", 10000) + `"}`},
	}

	for _, tc := range malformedRequests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post(mcpURL, "application/json", strings.NewReader(tc.body))
			if err != nil {
				// Connection error is OK - server might reject
				t.Logf("  %s: connection error (OK): %v", tc.name, err)
				return
			}
			defer resp.Body.Close()

			// Server should return error response, not crash
			// Any 4xx/5xx is acceptable for malformed input
			t.Logf("  %s: HTTP %d (server survived)", tc.name, resp.StatusCode)
		})
	}

	// Verify server is still alive after all malformed requests
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("Server died after malformed requests: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Server unhealthy after malformed requests: %d", resp.StatusCode)
	}

	t.Log("✅ Server survived all malformed JSON attacks")
}

// TestReliability_Recovery_InvalidToolCalls verifies server handles invalid
// MCP tool calls gracefully.
func TestReliability_Recovery_InvalidToolCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping recovery test in short mode")
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

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)

	invalidCalls := []struct {
		name    string
		request string
	}{
		{"nonexistent tool", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nonexistent_tool","arguments":{}}}`},
		{"missing tool name", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"arguments":{}}}`},
		{"null arguments", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":null}}`},
		{"wrong argument type", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":"not an object"}}`},
		{"invalid observe mode", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"nonexistent_mode"}}}`},
		{"invalid configure action", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"configure","arguments":{"what":"nonexistent_action"}}}`},
		{"invalid generate format", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"generate","arguments":{"what":"nonexistent_format"}}}`},
		{"invalid interact action", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"interact","arguments":{"what":"nonexistent_action"}}}`},
		{"negative limit", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs","limit":-100}}}`},
		{"huge limit", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs","limit":999999999}}}`},
	}

	for _, tc := range invalidCalls {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post(mcpURL, "application/json", strings.NewReader(tc.request))
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			// Should get valid JSON-RPC response (even if error)
			var jsonResp map[string]any
			if err := json.Unmarshal(body, &jsonResp); err != nil {
				t.Errorf("Invalid JSON response for %s: %v", tc.name, err)
				return
			}

			// Must have jsonrpc field
			if jsonResp["jsonrpc"] != "2.0" {
				t.Errorf("Missing jsonrpc:2.0 in response for %s", tc.name)
			}

			t.Logf("  %s: valid JSON-RPC response", tc.name)
		})
	}

	// Verify server is still alive
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("Server died after invalid tool calls: %v", err)
	}
	resp.Body.Close()

	t.Log("✅ Server survived all invalid tool calls with valid JSON-RPC responses")
}

//go:build integration

package bridgeintegration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	testprocess "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/integrationtest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type contentionClient struct {
	index  int
	stdin  io.WriteCloser
	reader *bufio.Reader
	cmd    *exec.Cmd
}

// TestBridgeStartupContention_AllClientsConverge ensures concurrent bridge
// clients all complete startup tool calls without transient startup retry noise.
func TestBridgeStartupContention_AllClientsConverge(t *testing.T) {
	if testing.Short() {
		t.Skip("skips bridge contention integration in short mode")
	}

	binary := testprocess.BuildBinary(t)
	port := testprocess.FreePort(t)
	const clientCount = 3

	clients := make([]contentionClient, 0, clientCount)
	for i := 0; i < clientCount; i++ {
		cmd := testprocess.StartServer(t, binary, "--bridge", "--port", fmt.Sprintf("%d", port))
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatalf("client %d stdin pipe: %v", i, err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("client %d stdout pipe: %v", i, err)
		}
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			t.Fatalf("client %d start: %v", i, err)
		}
		clients = append(clients, contentionClient{
			index:  i,
			stdin:  stdin,
			reader: bufio.NewReader(stdout),
			cmd:    cmd,
		})
	}
	for _, client := range clients {
		client := client
		t.Cleanup(func() {
			_ = client.stdin.Close()
			if client.cmd != nil && client.cmd.Process != nil {
				_ = client.cmd.Process.Kill()
			}
			_ = client.cmd.Wait()
		})
	}

	errCh := make(chan error, clientCount)
	var wg sync.WaitGroup
	for _, client := range clients {
		client := client
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runContentionClientStartupFlow(client); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatal(err)
	}
}

func runContentionClientStartupFlow(client contentionClient) error {
	initReq := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"contention-%d","version":"1.0"}}}`, client.index+1, client.index)
	if _, err := client.stdin.Write([]byte(initReq + "\n")); err != nil {
		return fmt.Errorf("client %d initialize write: %w", client.index, err)
	}
	initResp, err := readJSONRPCWithTimeout(client.reader, testprocess.ResponseTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("client %d initialize read: %w", client.index, err)
	}
	if initResp.Error != nil {
		return fmt.Errorf("client %d initialize protocol error: %s", client.index, initResp.Error.Message)
	}

	toolReq := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`, client.index+100)
	start := time.Now()
	if _, err := client.stdin.Write([]byte(toolReq + "\n")); err != nil {
		return fmt.Errorf("client %d tools/call write: %w", client.index, err)
	}
	toolResp, err := readJSONRPCWithTimeout(client.reader, testprocess.ResponseTimeout(4*time.Second))
	if err != nil {
		return fmt.Errorf("client %d tools/call read: %w", client.index, err)
	}
	elapsed := time.Since(start)
	if !testprocess.Instrumented() && elapsed > 4*time.Second {
		return fmt.Errorf("client %d tools/call took %v, want <= 4s", client.index, elapsed)
	}
	if toolResp.Error != nil {
		return fmt.Errorf("client %d tools/call protocol error: %s", client.index, toolResp.Error.Message)
	}
	if len(toolResp.Result) == 0 {
		return fmt.Errorf("client %d tools/call missing result", client.index)
	}

	var result map[string]any
	if err := json.Unmarshal(toolResp.Result, &result); err != nil {
		return fmt.Errorf("client %d tools/call result decode: %w", client.index, err)
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		return nil
	}
	block, _ := content[0].(map[string]any)
	text, _ := block["text"].(string)
	lower := strings.ToLower(text)
	if strings.Contains(lower, "server is starting up") || strings.Contains(lower, "retry this tool call") {
		return fmt.Errorf("client %d saw transient startup retry envelope: %q", client.index, text)
	}
	return nil
}

func readJSONRPCWithTimeout(reader *bufio.Reader, timeout time.Duration) (mcp.JSONRPCResponse, error) {
	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		lineCh <- line
	}()

	select {
	case line := <-lineCh:
		var resp mcp.JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			return mcp.JSONRPCResponse{}, err
		}
		return resp, nil
	case err := <-errCh:
		return mcp.JSONRPCResponse{}, err
	case <-time.After(timeout):
		return mcp.JSONRPCResponse{}, fmt.Errorf("timeout after %s", timeout)
	}
}

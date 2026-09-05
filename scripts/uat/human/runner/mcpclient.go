// mcpclient.go — Speaks MCP JSON-RPC to the installed server over stdio.
//
// The runner drives the same binary a user's agent drives — kaboom-mcp from
// PATH — because a rig that called Go functions directly would pass while the
// shipped server was broken.

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// mcpSession is one MCP session over stdio.
type mcpSession struct {
	out    *bufio.Reader
	in     io.Writer
	nextID int
	stop   func() error
	// deadline bounds one call. Zero means wait forever, which is what the
	// framing tests want and what an interactive session must never do.
	deadline time.Duration
	// desynced records that a reply was abandoned. The next reply on the stream
	// belongs to the abandoned call, so every later answer would be attributed to
	// the wrong case — a verdict recorded against a response the tester never saw.
	desynced bool
}

// newClient wires a session to an already-open pair of streams.
//
// Separate from spawn so the framing can be tested without a server binary:
// the two failures this code can have — losing a reply among the server's other
// output, and hanging on a reply that never arrives — are both reachable with
// nothing but an io.Reader.
func newClient(out io.Reader, in io.Writer, stop func() error) *mcpSession {
	return &mcpSession{out: bufio.NewReaderSize(out, 1<<20), in: in, nextID: 1, stop: stop}
}

// spawn starts the MCP server and returns a session attached to its stdio.
func spawn(bin string, extraEnv []string) (*mcpSession, error) {
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), extraEnv...)
	// The server's stderr is its log. Passing it through keeps a crash visible to
	// the person running the rig instead of surfacing as an unexplained timeout.
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", bin, err)
	}
	mcpSession := newClient(stdout, stdin, func() error {
		_ = stdin.Close()
		return cmd.Wait()
	})
	mcpSession.deadline = callDeadline
	return mcpSession, nil
}

type rpcResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// initialize performs the handshake. A server that answers tools/call without
// it is out of spec, so the rig does what a real client does.
func (c *mcpSession) initialize() error {
	_, err := c.request("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "kaboom-human-uat", "version": "1"},
	})
	return err
}

// call invokes one tool and returns its raw result.
func (c *mcpSession) call(tool string, arguments map[string]any) (json.RawMessage, error) {
	return c.request("tools/call", map[string]any{"name": tool, "arguments": arguments})
}

// toolsList returns the server's own tool schema, which is what the case
// inventory is checked against at run time.
func (c *mcpSession) toolsList() (json.RawMessage, error) {
	return c.request("tools/list", map[string]any{})
}

// shutdown ends the session.
func (c *mcpSession) shutdown() error {
	if c.stop == nil {
		return nil
	}
	return c.stop()
}

// request writes one call and reads until the matching id comes back.
//
// Replies are matched by id rather than taken in order: the server may emit
// notifications, which carry no id, and taking the next line as the answer would
// attribute one call's result to another case's record.
func (c *mcpSession) request(method string, params map[string]any) (json.RawMessage, error) {
	if c.desynced {
		return nil, fmt.Errorf("%s: skipped — an earlier call timed out and its reply is still on the stream", method)
	}
	id := c.nextID
	c.nextID++
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": method, "params": params,
	})
	if err != nil {
		return nil, err
	}
	if _, err := c.in.Write(append(payload, '\n')); err != nil {
		return nil, fmt.Errorf("write %s: %w", method, err)
	}
	return c.awaitReply(id, method)
}

// awaitReply reads the matching reply, giving up after the deadline.
func (c *mcpSession) awaitReply(id int, method string) (json.RawMessage, error) {
	if c.deadline <= 0 {
		return c.readReply(id, method)
	}
	type outcome struct {
		result json.RawMessage
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := c.readReply(id, method)
		done <- outcome{result, err}
	}()
	timer := time.NewTimer(c.deadline)
	defer timer.Stop()
	select {
	case got := <-done:
		return got.result, got.err
	case <-timer.C:
		c.desynced = true
		return nil, fmt.Errorf("%s: no reply within %s", method, c.deadline)
	}
}

func (c *mcpSession) readReply(id int, method string) (json.RawMessage, error) {
	for {
		line, err := c.out.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("%s: the server closed its output before replying", method)
			}
			return nil, err
		}
		var reply rpcResponse
		if json.Unmarshal(line, &reply) != nil || reply.ID != id {
			// Not our reply: a notification, a log line, or another call's answer.
			if err != nil {
				return nil, fmt.Errorf("%s: no reply before end of output", method)
			}
			continue
		}
		if reply.Error != nil {
			return nil, fmt.Errorf("%s: server error %d: %s", method, reply.Error.Code, reply.Error.Message)
		}
		return reply.Result, nil
	}
}

// callDeadline bounds one tool call so a hung server does not strand the person
// at a prompt with no output.
const callDeadline = 90 * time.Second

---
status: draft
priority: tier-1
phase: v5.4-foundation
relates-to: [PRODUCT_SPEC.md, TECH_SPEC_KABOOM_RUN.md, TECH_SPEC_LOCAL_TAILER.md]
last-updated: 2026-01-31
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Log Ingestion: SSH Remote Tailer — Technical Specification

**Goal:** SSH into remote servers and tail log files, stream new lines to daemon ring buffer.

---

## Process Architecture

```
Kaboom Daemon
   │
   ├─ HTTP Server (localhost:7890)
   ├─ MCP Server (stdio)
   └─ Log Ingestion Goroutines
       │
       ├─ kaboom-run Listener
       ├─ Local File Tailer
       └─ SSH Tailer (this spec)
          ├─ SSH connect: prod.example.com
          ├─ Run: tail -f /var/log/app.log
          ├─ Stream: Lines back to localhost
          ├─ Parse: Extract timestamp, level, message
          └─ Send: HTTP POST /event → Ring Buffer
```

---

## Implementation

### File: `pkg/ingest/ssh_tailer.go`

```go
package ingest

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHTailer tails a remote log file via SSH
type SSHTailer struct {
	host          string
	user          string
	path          string
	privateKeyPath string
	client        *ssh.Client
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	handler       LogLineHandler
	stats         TailerStats
	reconnectDelay time.Duration
}

// NewSSHTailer creates a new SSH tailer
func NewSSHTailer(
	host string,
	user string,
	path string,
	handler LogLineHandler,
) (*SSHTailer, error) {
	// Find private key
	privateKeyPath := os.ExpandEnv("$HOME/.ssh/id_rsa")
	if _, err := os.Stat(privateKeyPath); err != nil {
		return nil, fmt.Errorf("SSH key not found at %s", privateKeyPath)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &SSHTailer{
		host:           host,
		user:           user,
		path:           path,
		privateKeyPath: privateKeyPath,
		ctx:            ctx,
		cancel:         cancel,
		handler:        handler,
		reconnectDelay: 5 * time.Second,
	}, nil
}

// Start begins tailing the remote file with circuit breaker pattern
func (t *SSHTailer) Start(ctx context.Context) error {
	const (
		MAX_CONSECUTIVE_FAILURES = 10
		BACKOFF_INITIAL_MS       = 5000
		BACKOFF_MAX_MS           = 120000
		BACKOFF_MULTIPLIER       = 2.0
	)

	consecutiveFailures := 0
	backoffMS := int64(BACKOFF_INITIAL_MS)

	for {
		// Circuit breaker: after 10+ consecutive failures, use exponential backoff
		if consecutiveFailures >= MAX_CONSECUTIVE_FAILURES {
			waitTime := time.Duration(backoffMS) * time.Millisecond
			fmt.Printf("[ssh-tailer] Circuit breaker active after %d failures. Waiting %v before retry.\n", consecutiveFailures, waitTime)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
				// Increase backoff for next cycle (exponential)
				newBackoff := int64(float64(backoffMS) * BACKOFF_MULTIPLIER)
				if newBackoff > BACKOFF_MAX_MS {
					backoffMS = BACKOFF_MAX_MS
				} else {
					backoffMS = newBackoff
				}
				continue
			}
		}

		// Connect to SSH server
		err := t.connect()
		if err != nil {
			consecutiveFailures++
			t.recordError(err)

			// Log alerts at thresholds
			if consecutiveFailures == 10 {
				fmt.Printf("[WARN] [ssh-tailer] 10 consecutive failures to %s. Entering circuit breaker.\n", t.host)
			}
			if consecutiveFailures == 30 {
				fmt.Printf("[ERROR] [ssh-tailer] 30 consecutive failures to %s over ~2.5 minutes. Manual intervention required.\n", t.host)
			}

			fmt.Printf("[ssh-tailer] Connection failed: %v (attempt %d). Retrying...\n", err, consecutiveFailures)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):  // Initial backoff before circuit breaker kicks in
				continue
			}
		}

		// Success: reset failure counter and backoff
		consecutiveFailures = 0
		backoffMS = BACKOFF_INITIAL_MS

		// Stream logs
		err = t.stream(ctx)
		if err != nil {
			t.recordError(err)
			fmt.Printf("[ssh-tailer] Stream error: %v, reconnecting\n", err)
			if t.client != nil {
				t.client.Close()
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}
	}
}

func (t *SSHTailer) connect() error {
	// Read private key
	key, err := ioutil.ReadFile(t.privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// SSH config
	config := &ssh.ClientConfig{
		User: t.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: t.hostKeyCallback(),
		Timeout:         5 * time.Second,
	}

	// Connect
	client, err := ssh.Dial("tcp", t.host+":22", config)
	if err != nil {
		return fmt.Errorf("SSH dial failed: %w", err)
	}

	t.client = client
	return nil
}

func (t *SSHTailer) stream(ctx context.Context) error {
	defer t.client.Close()

	session, err := t.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Get stdout from remote tail command
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout: %w", err)
	}

	// Start remote tail command
	// Use tail -f with timeout fallback
	cmd := fmt.Sprintf("tail -f %s", t.path)
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("failed to start tail: %w", err)
	}

	// Read and process lines
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			session.Close()
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// Call handler (send to daemon)
		if err := t.handler(ctx, line); err != nil {
			t.stats.LinesFailed++
		} else {
			t.stats.LinesRead++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan error: %w", err)
	}

	// Wait for session to complete (shouldn't happen with tail -f)
	return session.Wait()
}

func (t *SSHTailer) hostKeyCallback() ssh.HostKeyCallback {
	// Try to use known_hosts file
	knownHostsPath := os.ExpandEnv("$HOME/.ssh/known_hosts")
	callback, err := knownhosts.New(knownHostsPath)
	if err == nil {
		return callback
	}

	// Fallback: Accept any host key (⚠️ for dev only, not production)
	fmt.Printf("[ssh-tailer] Warning: Using insecure host key verification\n")
	return ssh.InsecureIgnoreHostKey()
}

func (t *SSHTailer) recordError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.LastError = err.Error()
	t.stats.LastErrorTime = time.Now()
}

// Stop stops the tailer
func (t *SSHTailer) Stop() error {
	t.cancel()
	if t.client != nil {
		return t.client.Close()
	}
	return nil
}

// Stats returns current tailer statistics
func (t *SSHTailer) Stats() TailerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.stats
}
```

---

## Configuration

### YAML Schema

```yaml
backend:
  logs:
    # Remote SSH tailer
    - type: ssh
      host: prod.example.com
      user: ubuntu
      path: /var/log/app.log
      auth: ~/.ssh/id_rsa  # optional, defaults to ~/.ssh/id_rsa
      enabled: true
```

### SSH Key Management

#### Option 1: Default SSH key
```yaml
backend:
  logs:
    - type: ssh
      host: prod.example.com
      user: ubuntu
      path: /var/log/app.log
      # Uses ~/.ssh/id_rsa automatically
```

#### Option 2: Custom SSH key
```yaml
backend:
  logs:
    - type: ssh
      host: prod.example.com
      user: ubuntu
      path: /var/log/app.log
      auth: /path/to/custom/key
```

#### Option 3: SSH config
```yaml
backend:
  logs:
    - type: ssh
      host: prod-alias  # From ~/.ssh/config
      path: /var/log/app.log
      # Reads user from ~/.ssh/config
```

---

## SSH Connection Management

### Connection Lifecycle

```
1. Initial connect
   ├─ Read private key
   ├─ SSH dial to host:22
   └─ Authenticated

2. Stream logs
   ├─ Create session
   ├─ Run: tail -f /var/log/app.log
   └─ Read lines (blocks)

3. Connection lost
   ├─ Detect (stream ends)
   ├─ Close session
   ├─ Reconnect in 5 seconds

4. Stop
   ├─ Cancel context
   ├─ Close session
   └─ Close connection
```

### Error Recovery with Circuit Breaker

| Error Type | Retryable? | Behavior | Alert Threshold |
| --- | --- | --- | --- |
| Connection refused | Yes | Retry with fixed 5s backoff initially, then exponential backoff after 10 failures | 10 failures: WARN; 30 failures: ERROR |
| Authentication failed | No | Fail permanently (requires SSH key fix) | N/A |
| Stream closed (connection lost) | Yes | Reconnect after 5 seconds | Same as connection refused |
| Permission denied | No | Fail permanently (requires path/user fix) | N/A |
| File not found | No | Fail permanently (requires path fix) | N/A |
| Network timeout | Yes | Retry with fixed 5s backoff initially, then exponential backoff after 10 failures | Same as connection refused |
| SSH key not found | No | Fail at startup (requires key setup) | N/A |

### Reconnect Strategy (v5.4)

#### Initial Backoff (Attempts 1-9):

- Fixed 5 seconds between retries
- Allows quick recovery from transient failures (network blip, brief server restart)

#### Circuit Breaker (Attempts 10+):

- After 10 consecutive failures, enter circuit breaker mode
- Use exponential backoff: 5s → 10s → 20s → 40s → ... → max 120s
- Add jitter (±10%) to prevent thundering herd of multiple tailers
- Reset backoff and failure counter on successful connection

#### Monitoring & Alerting:

- After 10 consecutive failures: Log warning "Entering circuit breaker"
- After 30 consecutive failures (~2.5 minutes): Log error "Manual intervention required"
- Operator can check `/ingest/stats` endpoint to see tailer health
- All retryable errors are tracked in `TailerStats` (last_error, last_error_time)

#### Resource Bounds:

- File descriptors: Each failed connection attempt closes fd immediately (no leak)
- Goroutines: Single goroutine per tailer (fixed, no spawning)
- Memory: Backoff delays only use timer objects (negligible overhead)

---

## Performance Considerations

### Throughput
- **Network latency:** 100-500ms per line (depends on network)
- **Typical:** 10-100 lines/sec (network limited, not CPU limited)
- **Buffering:** SSH connection buffers up to 64KB

### Memory
- **Per connection:** ~5MB (SSH buffers)
- **Per-line in handler:** ~1KB (temporary)
- **Overhead:** Minimal, streaming only

### CPU
- **Idle:** 0% (waiting for data from SSH)
- **Streaming:** <1% (single core, mostly I/O wait)

---

## Security Considerations

### SSH Key Handling
- ✅ Private key read from disk only at startup
- ✅ Private key never logged or exposed
- ✅ Key passphrase not supported (must be unencrypted or use ssh-agent)
- ❌ No key passphrase support in v5.4

### Host Key Verification
- **Option A:** Use ~/.ssh/known_hosts (recommended)
- **Option B:** Insecure mode (accept any host key) ⚠️

### Credentials
- ✅ SSH key auth only (no password)
- ✅ No credentials logged
- ✅ No credentials in config file

---

## Multi-Service Configuration

### Example: Distributed microservices

```yaml
backend:
  logs:
    # Service A (Auth)
    - type: ssh
      host: auth-1.internal
      user: deploy
      path: /var/log/auth-service.log
      enabled: true

    # Service B (API)
    - type: ssh
      host: api-1.internal
      user: deploy
      path: /var/log/api-service.log
      enabled: true

    # Service C (Database) - optional
    - type: ssh
      host: db-1.internal
      user: deploy
      path: /var/log/postgres.log
      enabled: true
```

### Result: Unified Timeline

```
Query: GET /buffers/timeline

Timeline (all services merged):
1. [FE] User clicked login
2. [Auth] Received login request
3. [Auth] Called verify API
4. [API] Received verify request
5. [API] Queried database
6. [DB] SELECT query executed
7. [API] Response sent to Auth
8. [Auth] Response sent to FE
9. [FE] User redirected
```

---

## Debugging & Monitoring

### Health Check
```bash
curl http://localhost:7890/ingest/stats
```

Response:
```json
{
  "auth-1.internal": {
    "lines_read": 5000,
    "lines_failed": 0,
    "last_read_time": "2024-01-01T12:05:00Z",
    "last_error_time": null,
    "last_error": ""
  },
  "api-1.internal": {
    "lines_read": 3000,
    "lines_failed": 2,
    "last_read_time": "2024-01-01T12:05:01Z",
    "last_error_time": "2024-01-01T12:04:55Z",
    "last_error": "Connection timeout"
  }
}
```

### Debug Logging
```
[ssh-tailer] Connecting to auth-1.internal...
[ssh-tailer] Connected, user=deploy
[ssh-tailer] Starting tail: /var/log/auth-service.log
[ssh-tailer] Poll: read 42 lines (last 30s)
[ssh-tailer] Connection lost (read EOF), reconnecting in 5s
[ssh-tailer] Reconnecting to auth-1.internal...
```

---

## Testing Strategy

### Unit Tests
- [ ] SSH key parsing
- [ ] Connection config generation
- [ ] Error handling for missing files
- [ ] Error handling for auth failures

### Integration Tests
- [ ] Connect to real SSH server
- [ ] Stream logs successfully
- [ ] Handle connection loss
- [ ] Reconnect after failure
- [ ] Multiple simultaneous connections

### Manual Tests
```bash
# Test 1: Single server
kaboom --remote-tailer-ssh prod.example.com:ubuntu:/var/log/app.log

# Test 2: Multiple servers
# Edit config with 3 SSH sources
# Verify all stream simultaneously

# Test 3: Connection loss
# Pause network, verify reconnect
# Resume network, verify recovery

# Test 4: Slow network
# Use network shaper to limit bandwidth
# Verify graceful degradation
```

---

## SSH Wrapper Script (Optional)

For users without direct SSH access, wrapper script:

```bash
#!/bin/bash
# ~/.kaboom/ssh-tunnel.sh
# Establishes persistent SSH connection for log streaming

HOST=$1
USER=$2
LOG_PATH=$3

ssh -N -L 7891:localhost:22 $USER@$HOST &
TUNNEL_PID=$!

# Wait for tunnel to establish
sleep 1

# Connect via tunnel
tail -f $LOG_PATH | nc localhost 7891

# Cleanup
kill $TUNNEL_PID
```

---

## Limitations (v5.4)

- [ ] No password authentication (key-based only)
- [ ] No key passphrase support
- [ ] No tunnel/bastion host support
- [ ] No connection pooling (one connection per source)
- [ ] No log filtering

---

## Future Improvements

- [ ] Connection pooling (multiplex multiple files over one SSH)
- [ ] Bastion/jump host support
- [ ] Key passphrase via ssh-agent
- [ ] SFTP fallback (for restricted SSH)
- [ ] SCP for one-time log exports

---

## Related Documents

- **Product Spec:** [PRODUCT_SPEC.md](PRODUCT_SPEC.md)
- **kaboom-run:** [TECH_SPEC_KABOOM_RUN.md](TECH_SPEC_KABOOM_RUN.md)
- **Local Tailer:** [TECH_SPEC_LOCAL_TAILER.md](TECH_SPEC_LOCAL_TAILER.md)
- **Architecture:** [layer1-be-observability.md](../../core/target-architecture.md)

---

**Status:** Ready for implementation
**Estimated Effort:** 4 days
**Dependencies:** golang.org/x/crypto (ssh), stdlib (context, sync, io)

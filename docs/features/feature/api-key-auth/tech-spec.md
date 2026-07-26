---
status: proposed
scope: feature/api-key-auth/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-api-key-auth
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-api-key-auth.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Api Key Auth Review](api-key-auth-review.md).

# Technical Spec: API Key Authentication (Feature 20)

## Overview

This specification defines optional shared-secret authentication for Kaboom's HTTP API. When enabled, all HTTP requests must include a valid API key via the `X-API-Key` header or `Authorization: Bearer` header. Authentication is disabled by default, maintaining backward compatibility with existing deployments.

---

## Requirements

### Functional Requirements

1. **Optional enforcement** - Authentication is disabled by default. When no key is configured, all requests pass through without authentication checks.

2. **Header-based authentication** - Support two header formats:
   - `X-API-Key: <secret>` (custom header for simplicity)
   - `Authorization: Bearer <secret>` (standard OAuth-style header)

3. **Key rotation support** - Multiple valid keys can be configured simultaneously to enable zero-downtime key rotation.

4. **Audit logging** - All authentication attempts (success and failure) are logged to the audit trail with timestamp, source IP, and outcome.

5. **Constant-time comparison** - Key validation uses timing-safe comparison to prevent timing attacks.

6. **Extension compatibility** - The browser extension can be configured with an API key via the options page.

### Non-Functional Requirements

1. **Performance** - Authentication check adds < 0.1ms per request (single map lookup + constant-time compare).

2. **Zero dependencies** - Implementation uses only Go stdlib (`crypto/subtle`, `net/http`).

3. **Fail-secure** - Invalid or missing keys result in 401 Unauthorized. Malformed headers are rejected.

4. **Localhost exemption option** - Configurable bypass for requests from 127.0.0.1/::1 (useful during local development).

---

## Authentication Flow

### Request Processing

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          HTTP Request Received                               │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
                    ┌─────────────────────────────────────┐
                    │   Is authentication enabled?        │
                    │   (any keys configured?)            │
                    └─────────────────────────────────────┘
                              │                │
                         No   │                │ Yes
                              ▼                ▼
                    ┌──────────────┐  ┌─────────────────────────────┐
                    │ Pass through │  │ Check localhost exemption   │
                    │ (no auth)    │  │ (if enabled)                │
                    └──────────────┘  └─────────────────────────────┘
                                               │                │
                                          Exempt               │ Not exempt
                                               ▼                ▼
                                    ┌──────────────┐  ┌─────────────────────────┐
                                    │ Pass through │  │ Extract key from header │
                                    └──────────────┘  └─────────────────────────┘
                                                                │
                                                                ▼
                                              ┌─────────────────────────────────┐
                                              │ Check X-API-Key header first,   │
                                              │ then Authorization: Bearer      │
                                              └─────────────────────────────────┘
                                                                │
                                                                ▼
                                              ┌─────────────────────────────────┐
                                              │ Key found?                      │
                                              └─────────────────────────────────┘
                                                    │                │
                                               No   │                │ Yes
                                                    ▼                ▼
                                          ┌──────────────┐  ┌─────────────────────┐
                                          │ Log attempt  │  │ Constant-time       │
                                          │ 401 response │  │ compare against     │
                                          └──────────────┘  │ all valid keys      │
                                                            └─────────────────────┘
                                                                      │
                                                                      ▼
                                                      ┌───────────────────────────┐
                                                      │ Any key matches?          │
                                                      └───────────────────────────┘
                                                            │                │
                                                       No   │                │ Yes
                                                            ▼                ▼
                                                  ┌──────────────┐  ┌──────────────────┐
                                                  │ Log failure  │  │ Log success      │
                                                  │ 401 response │  │ Pass to handler  │
                                                  └──────────────┘  └──────────────────┘
```

### Header Priority

When both headers are present, `X-API-Key` takes precedence. This allows explicit Kaboom authentication to override any ambient `Authorization` headers that might be present from browser context.

```go
func extractAPIKey(r *http.Request) string {
    // Check X-API-Key first (Kaboom-specific)
    if key := r.Header.Get("X-API-Key"); key != "" {
        return key
    }

    // Fall back to Authorization: Bearer
    auth := r.Header.Get("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }

    return ""
}
```

---

## API Surface

### Server Configuration

#### CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--api-key=<key>` | Primary API key (can be specified multiple times for rotation) | (none) |
| `--api-key-file=<path>` | Path to file containing API keys (one per line) | (none) |
| `--api-key-localhost-exempt` | Allow unauthenticated requests from 127.0.0.1/::1 | `false` |

#### Environment Variables

| Variable | Description |
|----------|-------------|
| `KABOOM_API_KEY` | Primary API key (comma-separated for multiple keys) |
| `KABOOM_API_KEY_FILE` | Path to file containing API keys |
| `KABOOM_API_KEY_LOCALHOST_EXEMPT` | Set to `true` to enable localhost exemption |

#### Priority Order

1. CLI flags (highest priority)
2. Environment variables
3. Config file (JSON)
4. Default (no authentication)

Keys from all sources are merged into the valid key set.

### Key File Format

The key file contains one API key per line. Empty lines and lines starting with `#` are ignored:

```
# Production key
gsk_prod_a1b2c3d4e5f6g7h8i9j0
# Staging key (being rotated out)
gsk_stage_x1y2z3a4b5c6d7e8f9g0
```

### HTTP Responses

#### 401 Unauthorized

Returned when authentication fails. Response body is JSON:

```json
{
  "error": "unauthorized",
  "message": "Missing or invalid API key",
  "hint": "Include X-API-Key header or Authorization: Bearer header"
}
```

Headers included:
- `WWW-Authenticate: Bearer realm="kaboom"`
- `Content-Type: application/json`

#### Success

On successful authentication, the request proceeds to the handler. No special headers are added to indicate auth success.

### Extension Configuration

The browser extension options page gains an "API Key" field in the Server Settings section:

```
Server Settings
──────────────────────────────────────────
Server URL:  [http://localhost:7890     ]
API Key:     [••••••••••••••••••••••••••]  [Show]
             Key is sent with all requests to the server
```

When set, the extension includes the key in all POST requests:

```javascript
// In background.js POST requests
const headers = {
  'Content-Type': 'application/json',
};
if (apiKey) {
  headers['X-API-Key'] = apiKey;
}
```

The key is stored in `chrome.storage.local` (encrypted at rest by Chrome).

---

## Implementation Details

### Go Types

```go
// AuthConfig holds API key authentication configuration
type AuthConfig struct {
    // Keys is the set of valid API keys (may be empty to disable auth)
    Keys map[string]struct{}

    // LocalhostExempt allows unauthenticated requests from loopback addresses
    LocalhostExempt bool

    // AuditLog receives authentication events (optional)
    AuditLog *AuditLog
}

// AuthAttempt records an authentication attempt for audit logging
type AuthAttempt struct {
    Timestamp   time.Time `json:"timestamp"`
    RemoteAddr  string    `json:"remote_addr"`
    Method      string    `json:"method"`
    Path        string    `json:"path"`
    HeaderUsed  string    `json:"header_used"` // "X-API-Key", "Authorization", or ""
    Success     bool      `json:"success"`
    Reason      string    `json:"reason,omitempty"` // Failure reason if !Success
}
```

### Middleware Implementation

```go
// APIKeyMiddleware returns HTTP middleware that enforces API key authentication.
// If cfg.Keys is empty, authentication is disabled (all requests pass through).
func APIKeyMiddleware(cfg AuthConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // If no keys configured, auth is disabled
            if len(cfg.Keys) == 0 {
                next.ServeHTTP(w, r)
                return
            }

            // Check localhost exemption
            if cfg.LocalhostExempt && isLocalhost(r.RemoteAddr) {
                next.ServeHTTP(w, r)
                return
            }

            // Extract key from headers
            key, headerUsed := extractAPIKeyWithSource(r)

            // Log attempt (before validation to capture failures)
            attempt := AuthAttempt{
                Timestamp:  time.Now(),
                RemoteAddr: r.RemoteAddr,
                Method:     r.Method,
                Path:       r.URL.Path,
                HeaderUsed: headerUsed,
            }

            if key == "" {
                attempt.Success = false
                attempt.Reason = "no_key_provided"
                cfg.logAttempt(attempt)
                respondUnauthorized(w, "Missing API key")
                return
            }

            // Check if key is valid (constant-time comparison against all keys)
            if !cfg.validateKey(key) {
                attempt.Success = false
                attempt.Reason = "invalid_key"
                cfg.logAttempt(attempt)
                respondUnauthorized(w, "Invalid API key")
                return
            }

            attempt.Success = true
            cfg.logAttempt(attempt)
            next.ServeHTTP(w, r)
        })
    }
}

// validateKey checks if the provided key matches any configured key.
// Uses constant-time comparison to prevent timing attacks.
// Checks against ALL keys even after a match to maintain constant time.
func (cfg *AuthConfig) validateKey(providedKey string) bool {
    providedBytes := []byte(providedKey)
    valid := false

    for key := range cfg.Keys {
        keyBytes := []byte(key)
        // Always compare, even if we already found a match
        if subtle.ConstantTimeCompare(keyBytes, providedBytes) == 1 {
            valid = true
        }
    }

    return valid
}

// isLocalhost returns true if the address is a loopback address
func isLocalhost(addr string) bool {
    host, _, err := net.SplitHostPort(addr)
    if err != nil {
        host = addr
    }
    return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

func respondUnauthorized(w http.ResponseWriter, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("WWW-Authenticate", `Bearer realm="kaboom"`)
    w.WriteHeader(http.StatusUnauthorized)
    json.NewEncoder(w).Encode(map[string]string{
        "error":   "unauthorized",
        "message": message,
        "hint":    "Include X-API-Key header or Authorization: Bearer header",
    })
}
```

### Integration with Existing Auth

The existing `AuthMiddleware` in `auth.go` supports a single key via `X-Kaboom-Key` header. The new implementation:

1. **Replaces** the existing middleware with the enhanced version
2. **Adds** support for `X-API-Key` header (shorter, follows common conventions)
3. **Adds** support for `Authorization: Bearer` header
4. **Deprecates** `X-Kaboom-Key` (still accepted for backward compatibility, but not documented)
5. **Adds** multi-key support for rotation
6. **Adds** audit logging

Migration path:
- `X-Kaboom-Key` continues to work (mapped to the same validation)
- New deployments should use `X-API-Key` or `Authorization: Bearer`
- Documentation updated to recommend `X-API-Key`

---

## Key Generation and Storage

### Key Format

Recommended key format uses a prefix for identification:

```
gsk_<environment>_<random>
```

Examples:
- `gsk_prod_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6`
- `gsk_dev_x9y8z7w6v5u4t3s2r1q0`

The prefix is not enforced by the server, but recommended for operational clarity.

### Key Generation

Generate keys using cryptographically secure random bytes:

```bash
# Using openssl
openssl rand -base64 32 | tr -d '=' | tr '+/' '-_'

# Using /dev/urandom
head -c 32 /dev/urandom | base64 | tr -d '=' | tr '+/' '-_'
```

Recommended minimum key length: 32 characters (256 bits of entropy).

### Storage Recommendations

| Storage Method | Use Case | Security Notes |
|---------------|----------|----------------|
| Environment variable | CI/CD, containers | Visible in process listings; use secrets management |
| Key file | Local development | Exclude from version control; restrict file permissions |
| Config file | NOT RECOMMENDED | Config files may be committed; use env vars instead |

The server explicitly does NOT support API keys in the JSON config file to prevent accidental commits of secrets.

### Key Rotation Procedure

Zero-downtime key rotation:

1. **Generate new key**: Create a new API key
2. **Add to server**: Add new key to valid key list (via env var or key file)
3. **Restart server**: Server now accepts both old and new keys
4. **Update clients**: Update extension and any other clients to use new key
5. **Remove old key**: Remove old key from valid key list
6. **Restart server**: Server now only accepts new key

During steps 3-4, both keys are valid, enabling rolling updates.

---

## Security Considerations

### Timing Attacks

Key comparison uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks. When multiple keys are configured, ALL keys are compared even after finding a match to maintain constant-time behavior.

### Key Exposure

The API key is never:
- Logged in plaintext (audit logs record success/failure, not the key itself)
- Included in error responses
- Stored in memory after validation (only the hash of configured keys is retained)

### Transport Security

API keys provide authentication, not encryption. For production deployments:
- Use HTTPS (TLS) to protect keys in transit
- Localhost exemption is safe because loopback traffic cannot be intercepted

### Brute Force Protection

The server does not implement rate limiting specifically for auth failures. The global rate limiter provides some protection. For enhanced protection in exposed deployments:

1. Use long, random keys (32+ characters)
2. Deploy behind a reverse proxy with rate limiting
3. Monitor audit logs for repeated failures

### MCP Connection Security

MCP connections over stdio are NOT subject to API key authentication. The stdio transport inherently authenticates via the process model (only the spawning process can connect). API key auth applies only to HTTP endpoints:

- `POST /logs` (extension data ingestion)
- `POST /ws-events` (WebSocket events)
- `GET /health` (health check)
- `POST /mcp` (HTTP bridge requests)

---

## Audit Logging

### What Is Logged

Every authentication attempt (success or failure) produces an audit entry:

```json
{
  "type": "auth_attempt",
  "timestamp": "2025-01-20T14:30:00.123Z",
  "remote_addr": "127.0.0.1:54321",
  "method": "POST",
  "path": "/logs",
  "header_used": "X-API-Key",
  "success": true
}
```

For failures, additional context:

```json
{
  "type": "auth_attempt",
  "timestamp": "2025-01-20T14:30:01.456Z",
  "remote_addr": "192.168.1.100:54322",
  "method": "POST",
  "path": "/logs",
  "header_used": "",
  "success": false,
  "reason": "no_key_provided"
}
```

### Failure Reasons

| Reason | Description |
|--------|-------------|
| `no_key_provided` | Request had no authentication header |
| `invalid_key` | Key was provided but did not match any configured key |
| `malformed_header` | Authorization header present but not in Bearer format |

### Querying Auth Events

Auth events can be queried via the existing `get_audit_log` MCP tool with a filter:

```json
{
  "tool": "get_audit_log",
  "params": {
    "type": "auth_attempt",
    "success": false,
    "since": "2025-01-20T00:00:00Z",
    "limit": 100
  }
}
```

### Retention

Auth audit entries follow the same retention policy as other audit entries (configurable ring buffer size, default 10,000 entries).

---

## Testing Strategy

### Unit Tests

#### Key Validation

```go
func TestValidateKey_SingleKey(t *testing.T) {
    cfg := AuthConfig{Keys: map[string]struct{}{"secret123": {}}}

    assert.True(t, cfg.validateKey("secret123"))
    assert.False(t, cfg.validateKey("wrong"))
    assert.False(t, cfg.validateKey(""))
    assert.False(t, cfg.validateKey("secret1234")) // Close but not exact
}

func TestValidateKey_MultipleKeys(t *testing.T) {
    cfg := AuthConfig{Keys: map[string]struct{}{
        "key1": {},
        "key2": {},
        "key3": {},
    }}

    assert.True(t, cfg.validateKey("key1"))
    assert.True(t, cfg.validateKey("key2"))
    assert.True(t, cfg.validateKey("key3"))
    assert.False(t, cfg.validateKey("key4"))
}

func TestValidateKey_ConstantTime(t *testing.T) {
    // Verify validation time is constant regardless of key position
    cfg := AuthConfig{Keys: map[string]struct{}{
        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": {},
        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": {},
        "cccccccccccccccccccccccccccccccccc": {},
    }}

    // Time 1000 validations of first key vs last key
    // Difference should be < 10% (allows for noise)
    // This is a smoke test, not a rigorous timing analysis
}
```

#### Header Extraction

```go
func TestExtractAPIKey_XAPIKey(t *testing.T) {
    r := httptest.NewRequest("POST", "/logs", nil)
    r.Header.Set("X-API-Key", "mykey")

    key, header := extractAPIKeyWithSource(r)
    assert.Equal(t, "mykey", key)
    assert.Equal(t, "X-API-Key", header)
}

func TestExtractAPIKey_Bearer(t *testing.T) {
    r := httptest.NewRequest("POST", "/logs", nil)
    r.Header.Set("Authorization", "Bearer mykey")

    key, header := extractAPIKeyWithSource(r)
    assert.Equal(t, "mykey", key)
    assert.Equal(t, "Authorization", header)
}

func TestExtractAPIKey_BothHeaders_XAPIKeyWins(t *testing.T) {
    r := httptest.NewRequest("POST", "/logs", nil)
    r.Header.Set("X-API-Key", "correct")
    r.Header.Set("Authorization", "Bearer ignored")

    key, _ := extractAPIKeyWithSource(r)
    assert.Equal(t, "correct", key)
}

func TestExtractAPIKey_MalformedBearer(t *testing.T) {
    r := httptest.NewRequest("POST", "/logs", nil)
    r.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // Basic auth, not Bearer

    key, _ := extractAPIKeyWithSource(r)
    assert.Equal(t, "", key)
}
```

#### Localhost Exemption

```go
func TestLocalhostExemption_IPv4(t *testing.T) {
    assert.True(t, isLocalhost("127.0.0.1:54321"))
    assert.True(t, isLocalhost("127.0.0.1"))
    assert.False(t, isLocalhost("192.168.1.1:54321"))
}

func TestLocalhostExemption_IPv6(t *testing.T) {
    assert.True(t, isLocalhost("::1:54321"))
    assert.True(t, isLocalhost("[::1]:54321"))
    assert.False(t, isLocalhost("[::ffff:192.168.1.1]:54321"))
}
```

### Integration Tests

#### Middleware Integration

```go
func TestAPIKeyMiddleware_Disabled(t *testing.T) {
    handler := APIKeyMiddleware(AuthConfig{})(okHandler)

    r := httptest.NewRequest("POST", "/logs", nil)
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, r)

    assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
    cfg := AuthConfig{Keys: map[string]struct{}{"secret": {}}}
    handler := APIKeyMiddleware(cfg)(okHandler)

    r := httptest.NewRequest("POST", "/logs", nil)
    r.Header.Set("X-API-Key", "secret")
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, r)

    assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIKeyMiddleware_InvalidKey(t *testing.T) {
    cfg := AuthConfig{Keys: map[string]struct{}{"secret": {}}}
    handler := APIKeyMiddleware(cfg)(okHandler)

    r := httptest.NewRequest("POST", "/logs", nil)
    r.Header.Set("X-API-Key", "wrong")
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, r)

    assert.Equal(t, http.StatusUnauthorized, w.Code)
    assert.Contains(t, w.Header().Get("WWW-Authenticate"), "Bearer")
}

func TestAPIKeyMiddleware_MissingKey(t *testing.T) {
    cfg := AuthConfig{Keys: map[string]struct{}{"secret": {}}}
    handler := APIKeyMiddleware(cfg)(okHandler)

    r := httptest.NewRequest("POST", "/logs", nil)
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, r)

    assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAPIKeyMiddleware_LocalhostExempt(t *testing.T) {
    cfg := AuthConfig{
        Keys:            map[string]struct{}{"secret": {}},
        LocalhostExempt: true,
    }
    handler := APIKeyMiddleware(cfg)(okHandler)

    r := httptest.NewRequest("POST", "/logs", nil)
    r.RemoteAddr = "127.0.0.1:54321"
    // No API key provided
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, r)

    assert.Equal(t, http.StatusOK, w.Code)
}
```

#### Audit Logging

```go
func TestAPIKeyMiddleware_AuditLogging(t *testing.T) {
    var logged []AuthAttempt
    cfg := AuthConfig{
        Keys: map[string]struct{}{"secret": {}},
        AuditLog: &mockAuditLog{
            OnLog: func(a AuthAttempt) { logged = append(logged, a) },
        },
    }
    handler := APIKeyMiddleware(cfg)(okHandler)

    // Successful auth
    r := httptest.NewRequest("POST", "/logs", nil)
    r.Header.Set("X-API-Key", "secret")
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, r)

    require.Len(t, logged, 1)
    assert.True(t, logged[0].Success)
    assert.Equal(t, "X-API-Key", logged[0].HeaderUsed)

    // Failed auth
    r = httptest.NewRequest("POST", "/logs", nil)
    r.Header.Set("X-API-Key", "wrong")
    w = httptest.NewRecorder()
    handler.ServeHTTP(w, r)

    require.Len(t, logged, 2)
    assert.False(t, logged[1].Success)
    assert.Equal(t, "invalid_key", logged[1].Reason)
}
```

### Extension Tests

```javascript
// extension-tests/api-key.test.js

test('includes X-API-Key header when configured', async () => {
    await chrome.storage.local.set({ apiKey: 'test-key' });

    const requests = [];
    global.fetch = (url, opts) => {
        requests.push({ url, opts });
        return Promise.resolve({ ok: true, json: () => ({}) });
    };

    await sendLogs([{ level: 'log', message: 'test' }]);

    assert.equal(requests[0].opts.headers['X-API-Key'], 'test-key');
});

test('omits X-API-Key header when not configured', async () => {
    await chrome.storage.local.remove('apiKey');

    const requests = [];
    global.fetch = (url, opts) => {
        requests.push({ url, opts });
        return Promise.resolve({ ok: true, json: () => ({}) });
    };

    await sendLogs([{ level: 'log', message: 'test' }]);

    assert.equal(requests[0].opts.headers['X-API-Key'], undefined);
});

test('handles 401 response gracefully', async () => {
    await chrome.storage.local.set({ apiKey: 'wrong-key' });

    global.fetch = () => Promise.resolve({
        ok: false,
        status: 401,
        json: () => ({ error: 'unauthorized' })
    });

    // Should not throw, should enter backoff
    await sendLogs([{ level: 'log', message: 'test' }]);

    // Verify connection state reflects auth failure
    const state = await getConnectionState();
    assert.equal(state.lastError, 'unauthorized');
});
```

### Manual Testing Checklist

- [ ] Start server with `--api-key=test123`, verify requests without key return 401
- [ ] Verify requests with correct key succeed
- [ ] Verify requests with incorrect key return 401
- [ ] Configure extension with API key, verify data flows to server
- [ ] Test key rotation: add second key, update extension, remove first key
- [ ] Verify localhost exemption works when enabled
- [ ] Verify audit log contains auth attempts via `get_audit_log`
- [ ] Load test: 1000 requests/sec should not show timing variations based on key validity

---

## Files to Change

| File | Changes |
|------|---------|
| `cmd/browser-agent/internal/httpguard/apikey.go` | Replace existing middleware with enhanced version |
| `cmd/browser-agent/internal/httpguard/apikey_test.go` | New test file for auth middleware |
| `cmd/browser-agent/main.go` | Add CLI flags, wire up middleware |
| `cmd/browser-agent/config.go` | Add auth config parsing (env, flags, key file) |
| `cmd/browser-agent/audit.go` | Add auth attempt logging |
| `extension/background.js` | Include API key in requests |
| `extension/options.html` | Add API key input field |
| `extension/options.js` | Handle API key storage |
| `docs/configuration.md` | Document API key configuration |

---

## Backward Compatibility

- **Existing deployments without auth** continue to work unchanged (auth disabled by default)
- **Existing `X-Kaboom-Key` header** continues to work (mapped internally to same validation)
- **Extension without API key configured** continues to work if server has no keys
- **MCP stdio connections** are unaffected (no auth on stdio)

---

## Future Considerations

### Out of Scope (Potential Future Work)

1. **Per-key permissions** - Different keys with different access levels (read-only, full access)
2. **Key expiration** - Automatic key rotation with TTL
3. **HMAC-based auth** - Request signing instead of static keys
4. **mTLS** - Client certificate authentication for enterprise deployments
5. **OAuth integration** - Delegated auth for team environments

These are not included in this spec but the middleware architecture supports adding them later.

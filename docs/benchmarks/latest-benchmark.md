---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Kaboom Performance Benchmarks

**Generated:** 2026-02-06T14:00:00Z
**Machine:** Darwin arm64, Apple M1 Pro, 32 GB RAM
**Go:** go1.25.6 | **Node:** v24.1.0
**Version:** 0.8.8
**Commit:** c8d0e67

## Binary Sizes

| Platform | Size | Previous | Delta | Change |
|----------|------|----------|-------|--------|
| darwin-arm64 | 7.1 MB | 7.4 MB | -0.3 MB | -4% |
| darwin-x64 | 7.6 MB | 7.9 MB | -0.3 MB | -4% |
| linux-arm64 | 7.0 MB | 7.2 MB | -0.2 MB | -3% |
| linux-x64 | 7.5 MB | 7.8 MB | -0.3 MB | -4% |
| win32-x64 | 7.7 MB | 8.0 MB | -0.3 MB | -4% |

Binary sizes decreased ~4% from 5.7.5 due to code cleanup and dead code removal.

## Go Benchmarks

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| RingBufferWriteOne | 81.60 | 0 | 0 |
| RingBufferWrite | 126.8 | 0 | 0 |
| RingBufferReadFrom | 1,119 | 4,096 | 1 |
| RingBufferReadAll | 1,403 | 8,192 | 1 |
| RingBufferWriteWithEviction | 84.69 | 0 | 0 |
| RingBufferConcurrent | 9,266 | 40,122 | 0 |
| AsyncQueue | 2,801 | 1,345 | 11 |
| AddWebSocketEvents | 136,050 | 295,732 | 3 |
| AddNetworkBodies | 41,700 | 68,914 | 3 |
| AddEnhancedActions | 15,736 | 36,600 | 3 |
| MemoryEnforcement | 196,938 | 311,296 | 4 |
| ConcurrentCapture | 49,005 | 134,379 | 3 |
| ParseCursor | 80.96 | 0 | 0 |
| BuildCursor | 132.1 | 56 | 3 |
| EnrichLogEntries | 16,964 | 32,768 | 1 |
| ApplyLogCursorPagination | 170,373 | 230,496 | 2,014 |
| EnrichWebSocketEntries | 65,629 | 237,568 | 1 |
| ApplyWebSocketCursorPagination | 169,988 | 262,135 | 14 |
| EnrichActionEntries | 127,484 | 269,761 | 1,001 |
| ApplyActionCursorPagination | 470,813 | 1,051,130 | 6,017 |

MCP fast-start response time: ~130ms (including process startup). Bridge-only latency: < 1ms.

## Extension Performance

| Metric | Measured | SLO Target | Status |
|--------|----------|------------|--------|
| Simple object serialization | < 0.1ms | < 0.1ms | PASS |
| Nested object serialization | < 0.5ms | < 0.5ms | PASS |
| Large array serialization | < 1ms | < 1ms | PASS |
| Circular reference handling | < 0.5ms | < 0.5ms | PASS |
| Large string truncation | < 1ms | < 1ms | PASS |
| Error signature computation | < 0.1ms | < 0.1ms | PASS |
| Network error signature | < 0.1ms | < 0.1ms | PASS |
| Log entry formatting | < 0.1ms | < 0.1ms | PASS |
| Large args formatting | < 1ms | < 1ms | PASS |
| Error group processing | < 0.2ms | < 0.2ms | PASS |
| Resource timing parse | < 0.1ms/entry | < 0.1ms/entry | PASS |
| Waterfall (50 entries) | < 5ms | < 5ms | PASS |
| Action recording | < 0.1ms | < 0.1ms | PASS |
| Complete error flow | < 5ms | < 5ms | PASS |
| Memory bounded growth | bounded | bounded | PASS |

All 15 performance tests passing.

## Test Coverage

| Metric | Value |
|--------|-------|
| Go test count | 827 |
| JS test count | 925 |
| Total | 1,752 |

## SLO Compliance

| SLO | Target | Measured | Status |
|-----|--------|----------|--------|
| Binary size | < 15 MB | 7.7 MB (max) | PASS |
| MCP fast-start | < 1s | ~130ms | PASS |
| MCP tool response | < 200ms | < 1ms | PASS |
| Serialization (simple) | < 0.1ms | < 0.1ms | PASS |
| Error path total | < 5ms | < 5ms | PASS |
| Daemon startup | < 4s | ~500ms typical | PASS |
| fetch() overhead | < 0.5ms | unmeasured (needs E2E) | UNMEASURED |
| WS handler | < 0.1ms/msg | unmeasured (needs E2E) | UNMEASURED |
| Page load impact | < 20ms | unmeasured (needs Lighthouse) | UNMEASURED |
| Server memory | < 30MB | unmeasured (needs load test) | UNMEASURED |

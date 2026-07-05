---
status: proposed
scope: feature/binary-format-detection/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-binary-format-detection
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Binary Format Detection

> QA plan for the Binary Format Detection feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Binary format detection decodes binary payloads (protobuf, MessagePack, CBOR, etc.) into structured JSON. The decoded content may contain sensitive data that was previously obscured by the binary encoding. Decoding makes hidden data readable.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Decoded protobuf fields exposing auth tokens | Verify that decoded protobuf string fields containing token-like patterns are redacted or flagged | critical |
| DL-2 | Decoded MessagePack maps with PII | Verify decoded JSON does not expose email addresses, phone numbers, or SSNs in clear text without warning | high |
| DL-3 | Session tokens in decoded WebSocket messages | Verify session/auth tokens in decoded binary WS messages are handled with same redaction as text messages | critical |
| DL-4 | API keys in decoded request bodies | Verify that binary request bodies decoded to JSON have the same redaction rules as text JSON bodies | critical |
| DL-5 | Raw binary data exposure | Verify that raw hex dumps or base64 blobs are bounded (first 1KB analyzed, first 10KB decoded) | medium |
| DL-6 | Decoded content in MCP tool responses | Verify `decoded_content` field in `get_websocket_events` and `get_network_bodies` follows existing privacy rules | high |
| DL-7 | Format detection metadata leaking payload structure | Verify format detection confidence and method do not reveal sensitive structural information about proprietary protocols | low |
| DL-8 | Cached format results persisting across sessions | Verify format cache is per-connection and does not leak format data from one connection to another | medium |
| DL-9 | gRPC-Web frame decoding exposing internal service names | Verify decoded gRPC metadata does not expose internal service or method names that should be private | medium |
| DL-10 | Entropy analysis classifying encrypted data | Verify high-entropy payloads are NOT decoded (reported as "encrypted or compressed") to avoid exposing encrypted content structure | high |

### Negative Tests (must NOT leak)
- [ ] Decoded protobuf fields must not expose bearer tokens or API keys without redaction
- [ ] Decoded MessagePack/CBOR content must apply the same privacy rules as JSON bodies
- [ ] Raw binary hex dumps must be bounded to prevent large binary payload exposure
- [ ] Encrypted payloads (high entropy) must not be decoded -- only classified
- [ ] Format cache must not serve decoded data from Connection A when queried for Connection B
- [ ] Internal gRPC service method names must not appear in MCP responses without redaction
- [ ] Base64-encoded binary in text messages must not be auto-decoded (text message, not binary detection scope)

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Format name is unambiguous | Format names are standard strings: "protobuf", "msgpack", "cbor", "flatbuffers", "avro", "bson" | [ ] |
| CL-2 | Confidence score is meaningful | Confidence 1.0 = Content-Type match, 0.8 = structural detection, etc. | [ ] |
| CL-3 | Detection method is clear | `detection_method` field explains HOW the format was identified | [ ] |
| CL-4 | Decoded vs. raw distinction | LLM can clearly distinguish decoded JSON content from raw binary description | [ ] |
| CL-5 | "Too short to identify" is clear | Messages <4 bytes have explicit "too short to identify format" message | [ ] |
| CL-6 | Partial decode is flagged | Corrupted binary with partial decode includes error marker at failure point | [ ] |
| CL-7 | Size efficiency is visible | `raw_size` vs `decoded_size` fields show token savings | [ ] |
| CL-8 | "Unknown binary" is distinguishable | Unrecognized format is clearly distinct from an error or empty payload | [ ] |
| CL-9 | Protobuf field numbers are labeled | Schemaless decode output uses field numbers (e.g., "1", "2") not named fields | [ ] |
| CL-10 | Nested message decode is clear | Recursively decoded protobuf shows nesting structure | [ ] |
| CL-11 | Compressed-then-encoded is explained | Two-layer detection (gzip then protobuf) clearly describes both layers | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM might confuse protobuf field numbers with array indices -- verify output format makes field numbering explicit
- [ ] LLM might treat "unknown binary format" as a detection failure rather than a valid classification -- verify messaging is clear
- [ ] LLM might assume confidence 0.8 means "probably wrong" rather than "structural match without Content-Type" -- verify confidence semantics
- [ ] LLM might not realize decoded protobuf lacks field names (schemaless) and attempt to reference fields by name -- verify output format
- [ ] LLM might confuse "encrypted or compressed" with "corrupted" -- verify these are distinct classifications

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (transparent integration -- zero user steps)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| View decoded binary WS messages | 0 steps: automatic via `observe(what: "websocket_events")` | No -- already transparent |
| View decoded binary network bodies | 0 steps: automatic via `observe(what: "network_bodies")` | No -- already transparent |
| View binary format in WS status | 0 steps: automatic via `observe(what: "websocket_status")` | No -- already transparent |
| Understand detection confidence | Read `format_confidence` and `detection_method` fields | No -- metadata is inline |

### Default Behavior Verification
- [ ] Feature works with zero configuration (detection is automatic and lazy)
- [ ] No MCP tool parameter needed to enable/disable binary detection
- [ ] Decoded content appears alongside raw data (not replacing it)
- [ ] Detection only runs when AI queries the data (lazy, not eager)
- [ ] Format cache is transparent (no user-facing cache management)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Content-Type protobuf detection | `Content-Type: application/protobuf` | Format "protobuf", confidence 1.0, method "content_type" | must |
| UT-2 | Content-Type msgpack detection | `Content-Type: application/msgpack` | Format "msgpack", confidence 1.0, method "content_type" | must |
| UT-3 | Content-Type CBOR detection | `Content-Type: application/cbor` | Format "cbor", confidence 1.0, method "content_type" | must |
| UT-4 | Content-Type gRPC-Web detection | `Content-Type: application/grpc-web+proto` | Format "protobuf", confidence 1.0, gRPC framing detected | must |
| UT-5 | MessagePack magic bytes (fixmap) | First byte 0x80-0x8f | Format "msgpack", method "magic_bytes" | must |
| UT-6 | MessagePack magic bytes (fixarray) | First byte 0x90-0x9f | Format "msgpack", method "magic_bytes" | must |
| UT-7 | CBOR magic bytes (map type) | First byte 0xa0-0xbf | Format "cbor", method "magic_bytes" | must |
| UT-8 | BSON magic bytes | First 4 bytes = LE size, last byte 0x00 | Format "bson", method "magic_bytes" | should |
| UT-9 | Avro container magic bytes | Bytes `Obj\x01` | Format "avro", method "magic_bytes" | should |
| UT-10 | Protobuf structural detection | Valid wire format: 2+ fields, sequential numbers | Format "protobuf", confidence ~0.8, method "structural" | must |
| UT-11 | Invalid protobuf (bad wire type) | Binary with wire type > 5 | NOT classified as protobuf | must |
| UT-12 | Protobuf schemaless decode - varint | Field 1 (varint) = 42 | Decoded: `{"1": 42}` | must |
| UT-13 | Protobuf schemaless decode - string | Field 2 (length-delimited) = "Hello" | Decoded: `{"2": "Hello"}` | must |
| UT-14 | Protobuf schemaless decode - nested | Nested message in field 3 | Decoded: `{"3": {"1": true, "2": 12345}}` | must |
| UT-15 | MessagePack full decode | Map with string, number, boolean, array, nil | Correct JSON equivalent | must |
| UT-16 | CBOR full decode | Map with text strings, numbers, arrays | Correct JSON equivalent | must |
| UT-17 | High entropy classification | Random 256 bytes (entropy >7.5) | "encrypted or compressed", not decoded | must |
| UT-18 | Gzip magic bytes | Bytes 0x1f 0x8b | Format "gzip", method "magic_bytes" or "entropy" | must |
| UT-19 | Zlib header detection | Bytes 0x78 0x9c | Format "zlib compressed" | should |
| UT-20 | Short message (<4 bytes) | 3 bytes of binary data | "binary (too short to identify format)" | must |
| UT-21 | Large message size limit | 200KB binary payload | Only first 1KB analyzed, first 10KB decoded | must |
| UT-22 | Text accidentally marked binary | Low entropy (<4.0) binary payload | Attempt text decoding | should |
| UT-23 | gRPC-Web frame prefix | Byte 0x00 followed by protobuf | Detected as protobuf with gRPC framing, individual messages decoded | must |
| UT-24 | Deprecated wire type (group) | Protobuf with group wire type | Parsed but marked "deprecated wire type (group)" | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Binary WS message decode in observe | Extension captures binary WS -> server stores -> `observe(what: "websocket_events")` | `decoded` field present with format and content | must |
| IT-2 | Binary network body decode | Extension captures binary response -> server stores -> `observe(what: "network_bodies")` | `content_format` and `decoded_content` fields present | must |
| IT-3 | WS status with binary formats | Multiple binary WS messages -> `observe(what: "websocket_status")` | Schema variants include binary format names and percentages | must |
| IT-4 | Format cache behavior | 3+ consecutive protobuf messages -> query | First 3 detected via structural, subsequent use cache | should |
| IT-5 | Format cache invalidation | Cached format + message that fails decode | Cache invalidated, full detection re-run | should |
| IT-6 | Compressed protobuf two-layer | gzip-wrapped protobuf response | Detected as gzip, decompressed, then protobuf decoded | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Format detection (magic bytes + structural) | Wall clock time per message | Under 0.5ms | must |
| PT-2 | Protobuf schemaless decode (1KB) | Wall clock time | Under 2ms | must |
| PT-3 | MessagePack decode (1KB) | Wall clock time | Under 1ms | must |
| PT-4 | CBOR decode (1KB) | Wall clock time | Under 1ms | must |
| PT-5 | Entropy calculation (1KB) | Wall clock time | Under 0.1ms | must |
| PT-6 | Format cache hit | Wall clock time | Under 0.01ms | should |
| PT-7 | Memory for decoded content | Memory overhead | 2-3x original binary size | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Mixed text and binary on same WS | Alternating JSON and protobuf messages | Each detected independently | must |
| EC-2 | Compressed protobuf (gzip-wrapped) | gzip(protobuf) payload | Two-layer detection: gzip then protobuf | must |
| EC-3 | gRPC-Web streaming (multiple messages) | HTTP response with multiple gRPC frames | Each message decoded individually | should |
| EC-4 | Invalid/corrupted binary | Starts as protobuf, corrupts midway | Partial decode returned with error marker | must |
| EC-5 | Base64-encoded binary in text message | Text WS message containing base64 blob | NOT auto-detected (out of scope) | must |
| EC-6 | Protobuf with all wire types | Varint, 64-bit, length-delimited, 32-bit | All parsed correctly | should |
| EC-7 | Empty binary payload | 0 bytes | Graceful handling, "empty binary" | must |
| EC-8 | FlatBuffers structural analysis | FlatBuffers payload | Field count and approximate types reported (no full decode) | should |
| EC-9 | MessagePack with extensions | MessagePack ext type markers | Decoded with extension type noted | should |
| EC-10 | CBOR with tags | Tagged CBOR values (datetime, bignum) | Tags decoded and represented in JSON output | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test page that uses WebSocket with binary protocol (protobuf or MessagePack)
- [ ] A test page that makes HTTP requests with binary response bodies (e.g., gRPC-Web endpoint)
- [ ] Alternatively: a page using Socket.io in binary mode (MessagePack)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "observe", "arguments": {"what": "websocket_events"}}` | Page has active binary WebSocket | Binary messages show `binary_format` field (e.g., "protobuf") and decoded content | [ ] |
| UAT-2 | `{"tool": "observe", "arguments": {"what": "websocket_status"}}` | WS connection with binary messages | Schema variants include format names like "protobuf UserUpdate 65%" | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "network_bodies"}}` | Page has made binary API requests | Binary bodies show `content_format` and `decoded_content` fields | [ ] |
| UAT-4 | Send 3+ consecutive protobuf WS messages, then query events | Observe the 4th+ messages | Format detection uses cache (still shows correct format) | [ ] |
| UAT-5 | Send a text WS message after binary messages | Observe the text message | Text message is NOT processed by binary detection, no format fields | [ ] |
| UAT-6 | Make a request to a gzip-compressed protobuf endpoint | Check decoded content | Two-layer detection noted, protobuf content decoded | [ ] |
| UAT-7 | Send a very short binary message (2 bytes) | Check the event | Message shows "binary (too short to identify format)" | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Decoded content follows privacy rules | Send a binary message containing a session token, query events | Token in decoded content follows same redaction as text content | [ ] |
| DL-UAT-2 | Encrypted payloads not decoded | Send random/encrypted binary data | Classified as "encrypted or compressed", no decoded content | [ ] |
| DL-UAT-3 | Large payload bounded | Send >100KB binary message | Only first 1KB analyzed, first 10KB decoded, remainder noted | [ ] |
| DL-UAT-4 | Format cache isolated | Connect two WS to different endpoints, one binary one text | Cache for one connection does not affect the other | [ ] |

### Regression Checks
- [ ] Existing `observe(what: "websocket_events")` still works for text-only WebSocket connections
- [ ] Existing `observe(what: "network_bodies")` still works for JSON/text response bodies
- [ ] Binary detection does not slow down message throughput (lazy evaluation only)
- [ ] Format cache does not grow unbounded (per-connection, invalidated on failure)
- [ ] Existing WebSocket monitoring features (connection tracking, message counting) unaffected

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |

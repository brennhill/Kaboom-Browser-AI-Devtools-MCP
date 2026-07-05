---
status: proposed
scope: feature/binary-format-detection/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-binary-format-detection
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-binary-format-detection.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Binary Format Detection Review](binary-format-detection-review.md).

# Technical Spec: Binary Format Detection

## Purpose

When the AI inspects WebSocket messages or network response bodies that contain binary data, it currently sees a hex dump or a base64 blob — meaningless noise that wastes tokens and provides no insight. A message like `0a 0f 08 01 12 0b 48 65 6c 6c 6f 20 57 6f 72 6c 64` is actually a Protocol Buffer encoding of `{ id: 1, name: "Hello World" }`, but the AI can't tell.

Binary format detection identifies the encoding of binary payloads by examining magic bytes, structural patterns, and contextual clues (Content-Type headers, file extensions, WebSocket subprotocols). Instead of showing raw bytes, the server reports "Protocol Buffers message (likely UserResponse schema: id=1, name='Hello World')" — turning opaque binary into structured information the AI can reason about.

---

## Opportunity & Business Value

**Real-time app debugging**: Many modern apps use binary protocols for performance: Protocol Buffers (gRPC-Web, Google APIs), MessagePack (Socket.io binary mode, Redis), CBOR (IoT, FIDO2), FlatBuffers (game engines), and Avro (Kafka). Without format detection, the AI is blind to the majority of data flowing through these apps.

**WebSocket intelligence**: The WebSocket status tool reports message schemas for JSON messages. Binary format detection extends this to binary WebSocket protocols — the AI can report "87% of WebSocket messages are protobuf UserUpdate, 10% are protobuf Heartbeat, 3% are MessagePack errors."

**gRPC-Web compatibility**: gRPC-Web is increasingly popular for browser-to-server communication. It uses HTTP POST with protobuf bodies. Without binary detection, these API calls appear as meaningless binary blobs in the network body capture. With it, the AI can decode field numbers and wire types, identify message structure, and even suggest `.proto` schema files.

**Reduced token waste**: A 1KB protobuf message displayed as hex is 3KB of tokens (hex + spaces). Decoded to field descriptions, it's 200 bytes. Binary detection reduces AI context consumption by 90%+ for apps with binary protocols.

**Developer-readable output**: Even when the AI can't fully decode a message (no schema available), identifying the format and showing field structure is massively more useful than raw bytes. "Protobuf: field 1 (varint) = 42, field 2 (length-delimited) = 12 bytes, field 3 (varint) = 1706100000" tells the developer the general shape without needing the `.proto` file.

---

## How It Works

### Detection Hierarchy

Binary payloads are identified through a cascade of checks, from most to least confident:

**1. Explicit signals** (highest confidence):
- `Content-Type: application/protobuf` or `application/x-protobuf` or `application/grpc-web+proto`
- `Content-Type: application/msgpack` or `application/x-msgpack`
- `Content-Type: application/cbor`
- `Content-Type: application/avro`
- WebSocket subprotocol header indicating binary format
- gRPC-Web frame prefix (byte 0x00 for data frames, 0x80 for trailers)

**2. Magic bytes** (high confidence):
- MessagePack: First byte in the fixmap (0x80-0x8f), fixarray (0x90-0x9f), or type markers (0xc0-0xdf) range
- CBOR: Major type byte patterns (0x00-0x1b for unsigned int, 0xa0-0xbf for map, etc.)
- FlatBuffers: First 4 bytes are a little-endian offset to the root table
- Avro: Magic bytes `Obj\x01` for Avro container files
- BSON: First 4 bytes are little-endian document size, last byte is 0x00
- Protocol Buffers: No magic bytes (see structural detection below)

**3. Structural detection** (medium confidence, for protobuf):
- Protobuf has no magic bytes, but its wire format is distinctive:
  - Every field starts with a varint tag: `(field_number << 3) | wire_type`
  - Wire types are 0-5 (varint, 64-bit, length-delimited, start-group, end-group, 32-bit)
  - Valid protobuf has field numbers that increment (mostly sequential)
  - Length-delimited fields have a varint length prefix followed by exactly that many bytes
- The detector parses the first 100 bytes as protobuf wire format. If it parses cleanly (no invalid wire types, no length overflows, at least 2 valid fields), it's classified as "likely protobuf" with 0.8 confidence.

**4. Entropy analysis** (low confidence, fallback):
- Random/encrypted data: high entropy (>7.5 bits/byte) → "encrypted or compressed (not decodable)"
- Compressed data: medium-high entropy (6.5-7.5) with gzip magic (1f 8b) or zlib header (78 01/9c/da) → "gzip/zlib compressed"
- Structured binary: medium entropy (4.0-6.5) without matching any format → "unknown binary format"
- Text accidentally marked as binary: low entropy (<4.0) → attempt text decoding

### Partial Decoding

For detected formats, the server performs best-effort decoding without a schema:

**Protocol Buffers** (schemaless decoding):
- Parse wire format: extract field numbers, wire types, and raw values
- Varints are decoded to numbers
- Length-delimited fields: attempt UTF-8 decode (if valid, show as string; otherwise show byte length)
- Nested messages: recursively parse length-delimited fields that look like valid protobuf
- Output: `{ "1": 42, "2": "Hello World", "3": { "1": true, "2": 1706100000 } }`

**MessagePack** (self-describing format):
- Full decode using the type system: maps, arrays, strings, numbers, booleans, nil, binary, extensions
- Output: native JSON equivalent of the MessagePack data

**CBOR** (self-describing format):
- Full decode: maps, arrays, text strings, byte strings, numbers, booleans, null, tags
- Output: native JSON equivalent

**FlatBuffers / Avro / BSON**:
- Structural analysis only (field count, approximate types) — full decode requires schema

### MCP Integration

Binary format detection integrates with existing tools transparently:

1. **`get_websocket_events`**: Binary messages include a `decoded` field with format name and decoded content
2. **`get_network_bodies`**: Binary response bodies include `content_format` and `decoded_content` fields
3. **`get_websocket_status`**: Schema variants include binary format types (e.g., "protobuf UserUpdate 65%, protobuf Heartbeat 30%")

No new MCP tool is needed — the decoding is automatic.

---

## Data Model

### Detected Format

Each binary payload analysis produces:
- Format name: "protobuf", "msgpack", "cbor", "flatbuffers", "avro", "bson", "gzip", "unknown_binary"
- Confidence: 0.0 to 1.0
- Detection method: "content_type", "magic_bytes", "structural", "entropy"
- Decoded content: JSON representation (or null if not decodable)
- Decode errors: Any issues encountered during partial decoding
- Raw size: Original binary size in bytes
- Decoded size: JSON representation size (for token efficiency comparison)

### Format Cache

For WebSocket connections that consistently use the same binary format, the detection result is cached per connection URL. After 3 consecutive messages of the same format, subsequent messages skip detection and go straight to decoding. Cache is invalidated if a message fails to decode with the cached format.

---

## Edge Cases

- **Mixed text and binary on same WebSocket**: Each message is detected independently. The format cache handles the common case but falls back to full detection when formats change.
- **Very short binary messages** (<4 bytes): Too short for reliable detection. Reported as "binary (too short to identify format)" with the raw hex.
- **Compressed protobuf** (gzip-wrapped): Detected as gzip first, decompressed, then the inner format is detected. Two-layer detection.
- **gRPC-Web streaming**: Multiple protobuf messages in one HTTP response, separated by gRPC frame headers. The detector handles the framing and decodes each message individually.
- **Invalid/corrupted binary**: If structural detection says "protobuf" but decoding fails partway through, the decoded portion is returned with an error marker at the failure point.
- **Large binary payloads** (>100KB): Only the first 1KB is analyzed for format detection. Decoding is limited to the first 10KB. Beyond that: "Format: protobuf (decoded first 10KB, 90KB remaining)."
- **Base64-encoded binary in text message**: Not automatically detected (text message, not binary). The AI can recognize base64 patterns in the text response and call a future decode utility if needed.
- **Protobuf with unknown field types** (groups, which are deprecated): Parsed but marked as "deprecated wire type (group)."

---

## Performance Constraints

- Format detection (magic bytes + structural): under 0.5ms per message
- Protobuf schemaless decode (1KB message): under 2ms
- MessagePack/CBOR decode (1KB message): under 1ms
- Entropy calculation (1KB): under 0.1ms
- Format cache hit: under 0.01ms
- Memory for decoded content: proportional to original (JSON is typically 2-3x the binary size)
- No impact on message throughput (detection is lazy — only runs when the AI queries the data)

---

## Test Scenarios

1. Content-Type "application/protobuf" → detected as protobuf with confidence 1.0
2. Content-Type "application/msgpack" → detected and fully decoded
3. gRPC-Web frame prefix (0x00) → detected as protobuf with gRPC framing
4. MessagePack fixmap first byte → detected via magic bytes
5. CBOR map major type → detected via magic bytes
6. Valid protobuf wire format → structural detection with confidence 0.8
7. Invalid protobuf (bad wire type) → not classified as protobuf
8. MessagePack full decode produces correct JSON
9. CBOR full decode produces correct JSON
10. Protobuf schemaless decode: varint, string, nested message
11. High entropy data → "encrypted or compressed"
12. Gzip magic bytes → "gzip compressed"
13. Short message (<4 bytes) → "too short to identify"
14. Large message (>100KB) → only first 1KB analyzed
15. Format cache hit after 3 consistent messages
16. Format cache invalidated on decode failure
17. Mixed formats on same connection → each detected independently
18. Compressed protobuf → two-layer detection (gzip then protobuf)
19. Decoded content appears in `get_websocket_events` response
20. Decoded content appears in `get_network_bodies` response
21. Schema variants in `get_websocket_status` include binary format names

---

## File Locations

Server implementation: `cmd/browser-agent/binary_detect.go` (detection logic, decoders, cache).

Tests: `cmd/browser-agent/binary_detect_test.go`.

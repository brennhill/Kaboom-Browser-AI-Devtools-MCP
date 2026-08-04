// binary_protobuf_bson_test.go — Tests binary formats, invariants, and fuzz safety.
package util

import (
	"testing"
	"testing/quick"
)

// ---------------------------------------------------------------------------
// isValidVarint — edge cases
// ---------------------------------------------------------------------------

func TestIsValidVarint_Terminated(t *testing.T) {
	t.Parallel()
	data := []byte{0x08, 0x01}
	if !isValidVarint(data) {
		t.Error("isValidVarint(terminated) = false, want true")
	}
}

func TestIsValidVarint_LongVarint(t *testing.T) {
	t.Parallel()
	data := make([]byte, 11)
	data[0] = 0x08
	for i := 1; i < 10; i++ {
		data[i] = 0x80
	}
	data[10] = 0x01
	if isValidVarint(data) {
		t.Error("isValidVarint(unterminated 10-byte) = true, want false")
	}
}

func TestIsValidVarint_ShortUnterminated(t *testing.T) {
	t.Parallel()
	data := []byte{0x08, 0x80}
	if !isValidVarint(data) {
		t.Error("isValidVarint(short unterminated) = false, want true")
	}
}

func TestIsValidVarint_SingleByte(t *testing.T) {
	t.Parallel()
	data := []byte{0x08}
	if !isValidVarint(data) {
		t.Error("isValidVarint(single byte) = false, want true")
	}
}

func TestIsValidVarint_ExactlyAtLimit(t *testing.T) {
	t.Parallel()
	data := make([]byte, 10)
	data[0] = 0x08
	for i := 1; i < 10; i++ {
		data[i] = 0x80
	}
	if isValidVarint(data) {
		t.Error("isValidVarint(exactly 10 bytes unterminated) = true, want false")
	}
}

// ---------------------------------------------------------------------------
// detectProtobufLengthDelimited — all branches
// ---------------------------------------------------------------------------

func TestDetectProtobufLengthDelimited_HighBitContinuation(t *testing.T) {
	t.Parallel()
	data := []byte{0x0a, 0x80, 0x01}
	result := detectProtobufLengthDelimited(data, 1)
	if result == nil {
		t.Fatal("detectProtobufLengthDelimited(continuation) = nil")
	}
	if result.Confidence != 0.6 {
		t.Errorf("Confidence = %f, want 0.6", result.Confidence)
	}
	if result.Name != "protobuf" {
		t.Errorf("Name = %q, want protobuf", result.Name)
	}
}

func TestDetectProtobufLengthDelimited_ValidLength(t *testing.T) {
	t.Parallel()
	data := []byte{0x0a, 0x03, 0x41, 0x42, 0x43}
	result := detectProtobufLengthDelimited(data, 1)
	if result == nil {
		t.Fatal("detectProtobufLengthDelimited(valid length) = nil")
	}
	if result.Confidence != 0.7 {
		t.Errorf("Confidence = %f, want 0.7", result.Confidence)
	}
}

func TestDetectProtobufLengthDelimited_InsufficientData(t *testing.T) {
	t.Parallel()
	data := []byte{0x0a, 0x05, 0x41, 0x42}
	result := detectProtobufLengthDelimited(data, 1)
	if result != nil {
		t.Errorf("detectProtobufLengthDelimited(short data) = %+v, want nil", result)
	}
}

func TestDetectProtobufLengthDelimited_ZeroLength(t *testing.T) {
	t.Parallel()
	data := []byte{0x0a, 0x00}
	result := detectProtobufLengthDelimited(data, 1)
	if result != nil {
		t.Errorf("detectProtobufLengthDelimited(zero length) = %+v, want nil", result)
	}
}

// ---------------------------------------------------------------------------
// detectProtobuf — all wire types and edge cases
// ---------------------------------------------------------------------------

func TestDetectProtobuf_TooShort(t *testing.T) {
	t.Parallel()
	if detectProtobuf(nil) != nil {
		t.Error("detectProtobuf(nil) != nil")
	}
	if detectProtobuf([]byte{0x08}) != nil {
		t.Error("detectProtobuf(single byte) != nil")
	}
}

func TestDetectProtobuf_InvalidWireType(t *testing.T) {
	t.Parallel()
	// Wire types 3, 4, 6, 7 are invalid
	for _, tag := range []byte{0x0b, 0x0c, 0x0e, 0x0f} {
		result := detectProtobuf([]byte{tag, 0x01})
		if result != nil {
			t.Errorf("detectProtobuf(tag 0x%02x) = %+v, want nil", tag, result)
		}
	}
}

func TestDetectProtobuf_FieldNumberZero(t *testing.T) {
	t.Parallel()
	result := detectProtobuf([]byte{0x00, 0x01})
	if result != nil {
		t.Errorf("detectProtobuf(field 0) = %+v, want nil", result)
	}
}

func TestDetectProtobuf_FieldNumberTooHigh(t *testing.T) {
	t.Parallel()
	result := detectProtobuf([]byte{0x80, 0x01})
	if result != nil {
		t.Errorf("detectProtobuf(field 16) = %+v, want nil", result)
	}
}

func TestDetectProtobuf_WireType1_Fixed64(t *testing.T) {
	t.Parallel()
	data := make([]byte, 9)
	data[0] = 0x09 // field 1, wire type 1
	for i := 1; i < 9; i++ {
		data[i] = 0x42
	}
	result := detectProtobuf(data)
	if result == nil {
		t.Fatal("detectProtobuf(fixed64) = nil")
	}
	if result.Name != "protobuf" {
		t.Errorf("Name = %q, want protobuf", result.Name)
	}
	if result.Confidence != 0.65 {
		t.Errorf("Confidence = %f, want 0.65", result.Confidence)
	}
	if result.Details != "field 1, fixed64" {
		t.Errorf("Details = %q, want 'field 1, fixed64'", result.Details)
	}
}

func TestDetectProtobuf_WireType1_TooShort(t *testing.T) {
	t.Parallel()
	data := make([]byte, 8)
	data[0] = 0x09
	result := detectProtobuf(data)
	if result != nil {
		t.Errorf("detectProtobuf(fixed64 short) = %+v, want nil", result)
	}
}

func TestDetectProtobuf_WireType5_Fixed32(t *testing.T) {
	t.Parallel()
	data := []byte{0x0d, 0x42, 0x43, 0x44, 0x45}
	result := detectProtobuf(data)
	if result == nil {
		t.Fatal("detectProtobuf(fixed32) = nil")
	}
	if result.Confidence != 0.65 {
		t.Errorf("Confidence = %f, want 0.65", result.Confidence)
	}
	if result.Details != "field 1, fixed32" {
		t.Errorf("Details = %q, want 'field 1, fixed32'", result.Details)
	}
}

func TestDetectProtobuf_WireType5_TooShort(t *testing.T) {
	t.Parallel()
	data := []byte{0x0d, 0x42, 0x43, 0x44}
	result := detectProtobuf(data)
	if result != nil {
		t.Errorf("detectProtobuf(fixed32 short) = %+v, want nil", result)
	}
}

func TestDetectProtobuf_Varint_InvalidLong(t *testing.T) {
	t.Parallel()
	data := make([]byte, 11)
	data[0] = 0x08
	for i := 1; i < 11; i++ {
		data[i] = 0x80
	}
	result := detectProtobuf(data)
	if result != nil {
		t.Errorf("detectProtobuf(invalid long varint) = %+v, want nil", result)
	}
}

func TestDetectProtobuf_HigherFieldNumbers(t *testing.T) {
	t.Parallel()
	// field 15, wire type 0 = (15 << 3) | 0 = 0x78
	result := detectProtobuf([]byte{0x78, 0x01})
	if result == nil {
		t.Fatal("detectProtobuf(field 15) = nil, want protobuf")
	}
	if result.Name != "protobuf" {
		t.Errorf("Name = %q, want protobuf", result.Name)
	}
}

// ---------------------------------------------------------------------------
// isValidProtobufWireType — exhaustive check
// ---------------------------------------------------------------------------

func TestIsValidProtobufWireType_All(t *testing.T) {
	t.Parallel()
	valid := map[byte]bool{0: true, 1: true, 2: true, 5: true}
	for wt := byte(0); wt < 8; wt++ {
		got := isValidProtobufWireType(wt)
		want := valid[wt]
		if got != want {
			t.Errorf("isValidProtobufWireType(%d) = %v, want %v", wt, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// protobufFieldDetail
// ---------------------------------------------------------------------------

func TestProtobufFieldDetail(t *testing.T) {
	t.Parallel()
	got := protobufFieldDetail(1, "varint")
	if got != "field 1, varint" {
		t.Errorf("protobufFieldDetail(1, varint) = %q, want 'field 1, varint'", got)
	}
	got = protobufFieldDetail(9, "fixed64")
	if got != "field 9, fixed64" {
		t.Errorf("protobufFieldDetail(9, fixed64) = %q, want 'field 9, fixed64'", got)
	}
}

// ---------------------------------------------------------------------------
// detectBSON — all branches
// ---------------------------------------------------------------------------

func TestDetectBSON_TooShort(t *testing.T) {
	t.Parallel()
	if detectBSON(nil) != nil {
		t.Error("detectBSON(nil) != nil")
	}
	if detectBSON([]byte{0x05, 0x00, 0x00, 0x00}) != nil {
		t.Error("detectBSON(4 bytes) != nil")
	}
}

func TestDetectBSON_DocLenTooSmall(t *testing.T) {
	t.Parallel()
	data := []byte{0x04, 0x00, 0x00, 0x00, 0x00}
	if result := detectBSON(data); result != nil {
		t.Errorf("detectBSON(docLen=4) = %+v, want nil", result)
	}
}

func TestDetectBSON_DocLenTooLarge(t *testing.T) {
	t.Parallel()
	// 0x01000001 = 16777217 in little-endian
	data := []byte{0x01, 0x00, 0x00, 0x01, 0x00}
	if result := detectBSON(data); result != nil {
		t.Errorf("detectBSON(docLen>16MB) = %+v, want nil", result)
	}
}

func TestDetectBSON_InvalidNullTerminator(t *testing.T) {
	t.Parallel()
	data := []byte{0x05, 0x00, 0x00, 0x00, 0x01}
	if result := detectBSON(data); result != nil {
		t.Errorf("detectBSON(bad terminator) = %+v, want nil", result)
	}
}

func TestDetectBSON_DocLenSmallerThanData(t *testing.T) {
	t.Parallel()
	data := []byte{0x05, 0x00, 0x00, 0x00, 0x00, 0x01}
	if result := detectBSON(data); result != nil {
		t.Errorf("detectBSON(docLen < len(data)) = %+v, want nil", result)
	}
}

func TestDetectBSON_InvalidElementType(t *testing.T) {
	t.Parallel()
	// docLen=10, data has 6 bytes, element type 0x20 (invalid)
	data := []byte{0x0a, 0x00, 0x00, 0x00, 0x20, 0x61}
	if result := detectBSON(data); result != nil {
		t.Errorf("detectBSON(invalid element type 0x20) = %+v, want nil", result)
	}
}

func TestDetectBSON_PartialDocument(t *testing.T) {
	t.Parallel()
	// docLen=20, 5 bytes of data, element type 0x02 (string) is valid
	data := []byte{0x14, 0x00, 0x00, 0x00, 0x02}
	result := detectBSON(data)
	if result == nil {
		t.Fatal("detectBSON(partial with valid type) = nil, want bson")
	}
	if result.Name != "bson" {
		t.Errorf("Name = %q, want bson", result.Name)
	}
	if result.Confidence != 0.65 {
		t.Errorf("Confidence = %f, want 0.65", result.Confidence)
	}
	if result.Details != "document" {
		t.Errorf("Details = %q, want document", result.Details)
	}
}

func TestDetectBSON_AllValidElementTypes(t *testing.T) {
	t.Parallel()
	validTypes := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09,
		0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13,
		0x7f, 0xff,
	}
	for _, et := range validTypes {
		if !isValidBSONElementType(et) {
			t.Errorf("isValidBSONElementType(0x%02x) = false, want true", et)
		}
	}
	invalidTypes := []byte{0x14, 0x20, 0x50, 0x7e, 0x80, 0xfe}
	for _, et := range invalidTypes {
		if isValidBSONElementType(et) {
			t.Errorf("isValidBSONElementType(0x%02x) = true, want false", et)
		}
	}
}

func TestDetectBSON_ExactDocLen(t *testing.T) {
	t.Parallel()
	data := []byte{0x05, 0x00, 0x00, 0x00, 0x00}
	result := detectBSON(data)
	if result == nil {
		t.Fatal("detectBSON(exact len, valid terminator) = nil, want bson")
	}
	if result.Name != "bson" {
		t.Errorf("Name = %q, want bson", result.Name)
	}
}

func TestDetectBSON_WithDocument(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x0c, 0x00, 0x00, 0x00,
		0x10,
		0x6e, 0x00,
		0x2a, 0x00, 0x00, 0x00,
		0x00,
	}
	result := detectBSON(data)
	if result == nil {
		t.Fatal("detectBSON(valid doc) = nil, want bson")
	}
	if result.Details != "document" {
		t.Errorf("Details = %q, want document", result.Details)
	}
}

// ---------------------------------------------------------------------------
// bsonDocLen
// ---------------------------------------------------------------------------

func TestBsonDocLen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
		want int
	}{
		{"min", []byte{0x05, 0x00, 0x00, 0x00}, 5},
		{"256", []byte{0x00, 0x01, 0x00, 0x00}, 256},
		{"65536", []byte{0x00, 0x00, 0x01, 0x00}, 65536},
		{"big", []byte{0x00, 0x00, 0x00, 0x01}, 16777216},
		{"mixed", []byte{0x0e, 0x00, 0x00, 0x00}, 14},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := bsonDocLen(tt.data)
			if got != tt.want {
				t.Errorf("bsonDocLen(%v) = %d, want %d", tt.data, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DetectBinaryFormat — end-to-end coverage
// ---------------------------------------------------------------------------

func TestDetectBinaryFormat_CBORTaggedViaTopLevel(t *testing.T) {
	t.Parallel()
	result := DetectBinaryFormat([]byte{0xd8, 0x01})
	if result == nil {
		t.Fatal("DetectBinaryFormat(0xd8) = nil")
	}
	if result.Name != "messagepack" {
		t.Errorf("Name = %q (MessagePack takes priority for 0xd8)", result.Name)
	}
}

func TestDetectBinaryFormat_CBORBreakViaTopLevel(t *testing.T) {
	t.Parallel()
	result := DetectBinaryFormat([]byte{0xff})
	if result == nil {
		t.Fatal("DetectBinaryFormat(0xff) = nil, want cbor break")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
}

func TestDetectBinaryFormat_ProtobufFixed64ViaTopLevel(t *testing.T) {
	t.Parallel()
	data := make([]byte, 9)
	data[0] = 0x11 // field 2, wire type 1
	result := DetectBinaryFormat(data)
	if result == nil {
		t.Fatal("DetectBinaryFormat(protobuf fixed64) = nil")
	}
	if result.Name != "protobuf" {
		t.Errorf("Name = %q, want protobuf", result.Name)
	}
}

func TestDetectBinaryFormat_NoMatchReturnsNil(t *testing.T) {
	t.Parallel()
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	result := DetectBinaryFormat(data)
	if result != nil {
		t.Errorf("DetectBinaryFormat(no-match binary) = %+v, want nil", result)
	}
}

// TestPropertyConfidenceBounds verifies that for any []byte, if DetectBinaryFormat
// returns non-nil, Confidence is in [0.0, 1.0].
func TestPropertyConfidenceBounds(t *testing.T) {
	f := func(data []byte) bool {
		result := DetectBinaryFormat(data)

		if result == nil {
			return true // nil result is valid
		}

		// Confidence must be in [0.0, 1.0]
		return result.Confidence >= 0.0 && result.Confidence <= 1.0
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyNameValidity verifies that if non-nil, Name is one of the
// expected format names.
func TestPropertyNameValidity(t *testing.T) {
	validNames := map[string]bool{
		"messagepack": true,
		"protobuf":    true,
		"cbor":        true,
		"bson":        true,
	}

	f := func(data []byte) bool {
		result := DetectBinaryFormat(data)

		if result == nil {
			return true // nil result is valid
		}

		// Name must be one of the valid format names
		return validNames[result.Name]
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyASCIIImmunity verifies that for any string of only printable ASCII
// (0x20-0x7e), if it's long enough (>10 bytes), DetectBinaryFormat returns nil.
func TestPropertyASCIIImmunity(t *testing.T) {
	f := func(data []byte) bool {
		// Filter to only printable ASCII (0x20-0x7e)
		filtered := make([]byte, 0, len(data))
		for _, b := range data {
			if b >= 0x20 && b <= 0x7e {
				filtered = append(filtered, b)
			}
		}

		// If not long enough, skip this test case
		if len(filtered) <= 10 {
			return true // Vacuously true
		}

		// DetectBinaryFormat should return nil for pure ASCII
		result := DetectBinaryFormat(filtered)
		return result == nil
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyEmptyInputReturnsNil verifies that empty input always returns nil.
func TestPropertyEmptyInputReturnsNil(t *testing.T) {
	result := DetectBinaryFormat([]byte{})
	if result != nil {
		t.Errorf("DetectBinaryFormat([]) = %+v, want nil", result)
	}

	result = DetectBinaryFormat(nil)
	if result != nil {
		t.Errorf("DetectBinaryFormat(nil) = %+v, want nil", result)
	}
}

// TestPropertyDeterminism verifies that DetectBinaryFormat always returns
// the same result for the same input.
func TestPropertyDeterminism(t *testing.T) {
	f := func(data []byte) bool {
		first := DetectBinaryFormat(data)
		second := DetectBinaryFormat(data)

		// Both nil or both non-nil with same values
		if first == nil && second == nil {
			return true
		}

		if first == nil || second == nil {
			return false
		}

		return first.Name == second.Name && first.Confidence == second.Confidence
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyConfidenceNonNegative verifies that confidence is never negative.
func TestPropertyConfidenceNonNegative(t *testing.T) {
	f := func(data []byte) bool {
		result := DetectBinaryFormat(data)

		if result == nil {
			return true
		}

		return result.Confidence >= 0.0
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

func FuzzDetectBinaryFormat(f *testing.F) {
	// MessagePack seeds
	f.Add([]byte{0xc0})             // nil
	f.Add([]byte{0x80})             // fixmap
	f.Add([]byte{0x90})             // fixarray
	f.Add([]byte{0xde, 0x00, 0x01}) // map16
	f.Add([]byte{0x91, 0x01})       // array[1]
	f.Add([]byte{0xcc, 0xff})       // uint8

	// CBOR seeds
	f.Add([]byte{0xf4})             // false
	f.Add([]byte{0xf5})             // true
	f.Add([]byte{0xf6})             // null
	f.Add([]byte{0xa1, 0x61, 0x31}) // {"a": "1"}

	// Protobuf seeds
	f.Add([]byte{0x08, 0x01})                               // field 1, varint=1
	f.Add([]byte{0x0a, 0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f}) // field 1, "hello"
	f.Add([]byte{0x10, 0x96, 0x01})                         // field 2, varint=150

	// BSON seeds
	f.Add([]byte{0x05, 0x00, 0x00, 0x00, 0x00})                                                 // minimal empty document
	f.Add([]byte{0x0c, 0x00, 0x00, 0x00, 0x02, 0x61, 0x00, 0x02, 0x00, 0x00, 0x00, 0x62, 0x00}) // {"a":"b"}

	// Non-binary seeds (should return nil)
	f.Add([]byte{0x89, 0x50, 0x4e, 0x47}) // PNG magic
	f.Add([]byte("Hello world"))          // ASCII text
	f.Add([]byte(`{"key":"value"}`))      // JSON
	f.Add([]byte{})                       // empty
	f.Add([]byte("a"))                    // single char
	f.Add([]byte{0x00})                   // null byte
	f.Add([]byte{0xff, 0xff, 0xff, 0xff}) // all high bits

	f.Fuzz(func(t *testing.T, data []byte) {
		// Call the function
		result := DetectBinaryFormat(data)

		// Invariant 1: Empty data → nil result
		if len(data) == 0 {
			if result != nil {
				t.Errorf("Expected nil for empty data, got %+v", result)
			}
			return
		}

		// If result is nil, no further checks needed
		if result == nil {
			return
		}

		// Invariant 2: Confidence must be in [0.0, 1.0]
		if result.Confidence < 0.0 || result.Confidence > 1.0 {
			t.Errorf("Confidence out of range [0.0, 1.0]: %f for data: %v", result.Confidence, data)
		}

		// Invariant 3: Name must be one of the known formats
		validNames := map[string]bool{
			"messagepack": true,
			"protobuf":    true,
			"cbor":        true,
			"bson":        true,
		}
		if !validNames[result.Name] {
			t.Errorf("Invalid format name: %q (data: %v)", result.Name, data)
		}

		// Invariant 4: Pure printable ASCII (>90% threshold) should return nil
		// This matches the isLikelyText logic
		if len(data) >= 10 {
			printableCount := 0
			for _, b := range data {
				if b >= 0x20 && b <= 0x7e {
					printableCount++
				}
			}
			printableRatio := float64(printableCount) / float64(len(data))
			if printableRatio > 0.90 {
				t.Errorf("High printable ASCII ratio (%.2f) should return nil, got %+v (data: %q)",
					printableRatio, result, string(data))
			}
		}

		// Invariant 5: Determinism - calling twice with same input returns same result
		result2 := DetectBinaryFormat(data)
		if result2 == nil {
			t.Errorf("Determinism violation: first call returned %+v, second returned nil", result)
			return
		}

		if result.Name != result2.Name {
			t.Errorf("Determinism violation in Name: %q != %q", result.Name, result2.Name)
		}
		if result.Confidence != result2.Confidence {
			t.Errorf("Determinism violation in Confidence: %f != %f", result.Confidence, result2.Confidence)
		}
		if result.Details != result2.Details {
			t.Errorf("Determinism violation in Details: %q != %q", result.Details, result2.Details)
		}
	})
}

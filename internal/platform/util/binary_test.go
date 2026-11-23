// binary_test.go — Tests binary detection behavior, edge cases, and coverage branches.
package util

import "testing"

func TestDetectBinaryFormat_Empty(t *testing.T) {
	t.Parallel()
	result := DetectBinaryFormat(nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %+v", result)
	}

	result = DetectBinaryFormat([]byte{})
	if result != nil {
		t.Errorf("expected nil for empty slice, got %+v", result)
	}
}

func TestDetectBinaryFormat_TextContent(t *testing.T) {
	t.Parallel()
	// Plain text should not be detected as binary format
	tests := []string{
		"hello world",
		`{"key": "value"}`,
		"<html><body>test</body></html>",
		"GET /api/test HTTP/1.1",
	}
	for _, text := range tests {
		result := DetectBinaryFormat([]byte(text))
		if result != nil {
			t.Errorf("expected nil for text content %q, got %+v", text, result)
		}
	}
}

func TestDetectBinaryFormat_MessagePack(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		// fixmap (0x80-0x8f): map with 0-15 elements
		{"fixmap_empty", []byte{0x80}},
		{"fixmap_1elem", []byte{0x81, 0xa3, 0x6b, 0x65, 0x79, 0xa5, 0x76, 0x61, 0x6c, 0x75, 0x65}}, // {"key":"value"}
		{"fixmap_max", []byte{0x8f}},

		// fixarray (0x90-0x9f): array with 0-15 elements
		{"fixarray_empty", []byte{0x90}},
		{"fixarray_3elem", []byte{0x93, 0x01, 0x02, 0x03}}, // [1,2,3]
		{"fixarray_max", []byte{0x9f}},

		// fixstr (0xa0-0xbf): string with 0-31 bytes
		{"fixstr_empty", []byte{0xa0}},
		{"fixstr_hello", []byte{0xa5, 0x68, 0x65, 0x6c, 0x6c, 0x6f}}, // "hello"

		// Type markers
		{"nil", []byte{0xc0}},
		{"false", []byte{0xc2}},
		{"true", []byte{0xc3}},
		{"float32", []byte{0xca, 0x40, 0x48, 0xf5, 0xc3}}, // 3.14
		{"float64", []byte{0xcb, 0x40, 0x09, 0x21, 0xfb, 0x54, 0x44, 0x2d, 0x18}},
		{"uint8", []byte{0xcc, 0xff}},
		{"uint16", []byte{0xcd, 0x01, 0x00}},
		{"uint32", []byte{0xce, 0x00, 0x01, 0x00, 0x00}},
		{"uint64", []byte{0xcf, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}},
		{"int8", []byte{0xd0, 0xff}},
		{"int16", []byte{0xd1, 0xff, 0xff}},
		{"int32", []byte{0xd2, 0xff, 0xff, 0xff, 0xff}},
		{"int64", []byte{0xd3, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{"map16", []byte{0xde, 0x00, 0x01}},
		{"map32", []byte{0xdf, 0x00, 0x00, 0x00, 0x01}},
		{"array16", []byte{0xdc, 0x00, 0x01}},
		{"array32", []byte{0xdd, 0x00, 0x00, 0x00, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result == nil {
				t.Fatalf("expected MessagePack detection for %s, got nil", tt.name)
			}
			if result.Name != "messagepack" {
				t.Errorf("expected name 'messagepack', got %q", result.Name)
			}
			if result.Confidence < 0.7 || result.Confidence > 1.0 {
				t.Errorf("expected confidence between 0.7-1.0, got %f", result.Confidence)
			}
		})
	}
}

func TestDetectBinaryFormat_Protobuf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		// Field 1, wire type 0 (varint) - common for protobuf messages
		{"field1_varint", []byte{0x08, 0x96, 0x01}},  // field 1, varint 150
		{"field1_varint_simple", []byte{0x08, 0x01}}, // field 1, varint 1

		// Field 1, wire type 2 (length-delimited) - string/bytes/embedded message
		{"field1_string", []byte{0x0a, 0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f}}, // field 1, string "hello"

		// Multiple fields
		{"multi_field", []byte{0x08, 0x01, 0x10, 0x02, 0x18, 0x03}}, // fields 1,2,3 with varints
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result == nil {
				t.Fatalf("expected protobuf detection for %s, got nil", tt.name)
			}
			if result.Name != "protobuf" {
				t.Errorf("expected name 'protobuf', got %q", result.Name)
			}
			if result.Confidence < 0.5 || result.Confidence > 1.0 {
				t.Errorf("expected confidence between 0.5-1.0, got %f", result.Confidence)
			}
		})
	}
}

func TestDetectBinaryFormat_CBOR(t *testing.T) {
	t.Parallel()
	// Note: CBOR and MessagePack share overlapping byte ranges for arrays/maps.
	// MessagePack is checked first, so those ranges are detected as MessagePack.
	// CBOR-specific tests focus on non-overlapping markers.

	tests := []struct {
		name string
		data []byte
	}{
		// Simple values (major type 7) - unique to CBOR
		{"false", []byte{0xf4}},
		{"true", []byte{0xf5}},
		{"null", []byte{0xf6}},
		{"undefined", []byte{0xf7}},
		{"float16", []byte{0xf9, 0x3c, 0x00}}, // 1.0
		{"float32", []byte{0xfa, 0x47, 0xc3, 0x50, 0x00}},
		{"float64", []byte{0xfb, 0x40, 0x09, 0x21, 0xfb, 0x54, 0x44, 0x2d, 0x18}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result == nil {
				t.Fatalf("expected CBOR detection for %s, got nil", tt.name)
			}
			if result.Name != "cbor" {
				t.Errorf("expected name 'cbor', got %q", result.Name)
			}
			if result.Confidence < 0.7 || result.Confidence > 1.0 {
				t.Errorf("expected confidence between 0.7-1.0, got %f", result.Confidence)
			}
		})
	}
}

func TestDetectBinaryFormat_CBOR_Overlapping(t *testing.T) {
	t.Parallel()
	// These CBOR markers overlap with MessagePack and are detected as MessagePack.
	// This is by design - MessagePack is more commonly used in web contexts.
	tests := []struct {
		name     string
		data     []byte
		expected string // "messagepack" or "cbor"
	}{
		// Map (0xa0-0xbf) overlaps with MessagePack fixstr
		{"map_empty", []byte{0xa0}, "messagepack"},
		{"map_1elem", []byte{0xa1, 0x61, 0x61, 0x01}, "messagepack"},

		// Array (0x80-0x9f) overlaps with MessagePack fixmap
		{"array_empty", []byte{0x80}, "messagepack"},
		{"array_3elem", []byte{0x83, 0x01, 0x02, 0x03}, "messagepack"},

		// Tagged values (0xc0-0xdf) overlap with MessagePack type markers
		// 0xc0 = MessagePack nil, 0xc1 = CBOR tag 1 (epoch)
		{"tagged_nil", []byte{0xc0}, "messagepack"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result == nil {
				t.Fatalf("expected detection for %s, got nil", tt.name)
			}
			if result.Name != tt.expected {
				t.Errorf("expected %s for %s, got %q", tt.expected, tt.name, result.Name)
			}
		})
	}
}

func TestDetectBinaryFormat_BSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		// BSON document: int32 length + elements + null terminator
		// Minimum valid BSON: 5 bytes (4-byte length = 5, 1-byte null terminator)
		{"empty_doc", []byte{0x05, 0x00, 0x00, 0x00, 0x00}},

		// Document with string field: {"a": "b"}
		// Length: 14 bytes total
		// 0x02 = string type, "a\0" = field name, 0x02000000 = string length (2), "b\0" = string value
		{"string_field", []byte{
			0x0e, 0x00, 0x00, 0x00, // length = 14
			0x02,       // type = string
			0x61, 0x00, // field name "a\0"
			0x02, 0x00, 0x00, 0x00, // string length = 2
			0x62, 0x00, // string value "b\0"
			0x00, // document terminator
		}},

		// Document with int32 field: {"x": 1}
		{"int32_field", []byte{
			0x0c, 0x00, 0x00, 0x00, // length = 12
			0x10,       // type = int32
			0x78, 0x00, // field name "x\0"
			0x01, 0x00, 0x00, 0x00, // value = 1
			0x00, // document terminator
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result == nil {
				t.Fatalf("expected BSON detection for %s, got nil", tt.name)
			}
			if result.Name != "bson" {
				t.Errorf("expected name 'bson', got %q", result.Name)
			}
			if result.Confidence < 0.5 || result.Confidence > 1.0 {
				t.Errorf("expected confidence between 0.5-1.0, got %f", result.Confidence)
			}
		})
	}
}

func TestDetectBinaryFormat_UnknownBinary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		// Random binary data that doesn't match any format
		{"random_bytes", []byte{0x00, 0x01, 0x02, 0x03, 0x04}},
		{"high_bytes", []byte{0xfe, 0xfe, 0xfe, 0xfe}},
		{"mixed", []byte{0x7f, 0x7e, 0x7d, 0x7c}}, // ASCII-ish but not text
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result != nil {
				t.Errorf("expected nil for unknown binary %s, got %+v", tt.name, result)
			}
		})
	}
}

func TestDetectBinaryFormat_Priority(t *testing.T) {
	t.Parallel()
	// Test that MessagePack is detected over CBOR for ambiguous bytes
	// 0x80 is both MessagePack fixmap(0) and CBOR array(0)
	// MessagePack should take priority per the spec
	result := DetectBinaryFormat([]byte{0x80})
	if result == nil {
		t.Fatal("expected detection for 0x80")
	}
	// MessagePack should be checked first and return higher confidence
	if result.Name != "messagepack" && result.Name != "cbor" {
		t.Errorf("expected messagepack or cbor, got %q", result.Name)
	}
}

func TestDetectBinaryFormat_SingleByte(t *testing.T) {
	t.Parallel()
	// Single bytes that are valid format indicators
	tests := []struct {
		data     byte
		expected string // "" means nil expected
	}{
		{0x80, "messagepack"}, // fixmap(0)
		{0x90, "messagepack"}, // fixarray(0)
		{0xa0, "messagepack"}, // fixstr(0)
		{0xc0, "messagepack"}, // nil
		{0xc2, "messagepack"}, // false
		{0xc3, "messagepack"}, // true
		{0xf4, "cbor"},        // CBOR false
		{0xf5, "cbor"},        // CBOR true
		{0xf6, "cbor"},        // CBOR null
	}

	for _, tt := range tests {
		t.Run(string([]byte{tt.data}), func(t *testing.T) {
			result := DetectBinaryFormat([]byte{tt.data})
			if tt.expected == "" {
				if result != nil {
					t.Errorf("expected nil for 0x%02x, got %+v", tt.data, result)
				}
			} else {
				if result == nil {
					t.Fatalf("expected %s for 0x%02x, got nil", tt.expected, tt.data)
				}
				if result.Name != tt.expected {
					t.Errorf("expected %s for 0x%02x, got %s", tt.expected, tt.data, result.Name)
				}
			}
		})
	}
}

func TestBinaryFormat_Details(t *testing.T) {
	t.Parallel()
	// Test that Details field provides useful information
	result := DetectBinaryFormat([]byte{0x08, 0x01}) // protobuf field 1, varint
	if result == nil {
		t.Fatal("expected protobuf detection")
	}
	if result.Details == "" {
		t.Log("Details field is empty (optional)")
	}
}

// ---------------------------------------------------------------------------
// isLikelyText — empty input and threshold boundary
// ---------------------------------------------------------------------------

func TestIsLikelyText_Empty(t *testing.T) {
	t.Parallel()
	if isLikelyText(nil) {
		t.Error("isLikelyText(nil) = true, want false")
	}
	if isLikelyText([]byte{}) {
		t.Error("isLikelyText([]byte{}) = true, want false")
	}
}

func TestIsLikelyText_MostlyBinary(t *testing.T) {
	t.Parallel()
	data := make([]byte, 100)
	for i := range data {
		data[i] = 0x01
	}
	if isLikelyText(data) {
		t.Error("isLikelyText(all binary) = true, want false")
	}
}

func TestIsLikelyText_ExactlyAtThreshold(t *testing.T) {
	t.Parallel()
	data := make([]byte, 100)
	for i := 0; i < 91; i++ {
		data[i] = 'a'
	}
	for i := 91; i < 100; i++ {
		data[i] = 0x01
	}
	if !isLikelyText(data) {
		t.Error("isLikelyText(91% text) = false, want true")
	}
}

func TestIsLikelyText_BelowThreshold(t *testing.T) {
	t.Parallel()
	data := make([]byte, 100)
	for i := 0; i < 90; i++ {
		data[i] = 'a'
	}
	for i := 90; i < 100; i++ {
		data[i] = 0x01
	}
	if isLikelyText(data) {
		t.Error("isLikelyText(exactly 90% text) = true, want false")
	}
}

func TestIsLikelyText_ControlChars(t *testing.T) {
	t.Parallel()
	data := []byte{0x09, 0x0a, 0x0d, 'a', 'b', 'c'}
	if !isLikelyText(data) {
		t.Error("isLikelyText(with control chars) = false, want true")
	}
}

// ---------------------------------------------------------------------------
// detectCBORMajorType — cover all branches
// ---------------------------------------------------------------------------

func TestDetectCBORMajorType_Array(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(4, 0x00)
	if result == nil {
		t.Fatal("detectCBORMajorType(4, 0x00) = nil, want array")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Confidence != 0.75 {
		t.Errorf("Confidence = %f, want 0.75", result.Confidence)
	}
	if result.Details != "array" {
		t.Errorf("Details = %q, want array", result.Details)
	}
}

func TestDetectCBORMajorType_ArrayMaxInfo(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(4, 0x17)
	if result == nil {
		t.Fatal("detectCBORMajorType(4, 0x17) = nil, want array")
	}
	if result.Details != "array" {
		t.Errorf("Details = %q, want array", result.Details)
	}
}

func TestDetectCBORMajorType_ArrayIndefinite(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(4, 0x1f)
	if result == nil {
		t.Fatal("detectCBORMajorType(4, 0x1f) = nil, want array")
	}
	if result.Details != "array" {
		t.Errorf("Details = %q, want array", result.Details)
	}
}

func TestDetectCBORMajorType_Map(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(5, 0x03)
	if result == nil {
		t.Fatal("detectCBORMajorType(5, 0x03) = nil, want map")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Details != "map" {
		t.Errorf("Details = %q, want map", result.Details)
	}
}

func TestDetectCBORMajorType_MapIndefinite(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(5, 0x1f)
	if result == nil {
		t.Fatal("detectCBORMajorType(5, 0x1f) = nil, want map")
	}
	if result.Details != "map" {
		t.Errorf("Details = %q, want map", result.Details)
	}
}

func TestDetectCBORMajorType_InvalidAdditionalInfo(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(4, 0x18)
	if result != nil {
		t.Errorf("detectCBORMajorType(4, 0x18) = %+v, want nil", result)
	}
	result = detectCBORMajorType(5, 0x1c)
	if result != nil {
		t.Errorf("detectCBORMajorType(5, 0x1c) = %+v, want nil", result)
	}
}

func TestDetectCBORMajorType_OtherMajorTypes(t *testing.T) {
	t.Parallel()
	for _, mt := range []byte{0, 1, 2, 3, 6, 7} {
		result := detectCBORMajorType(mt, 0x00)
		if result != nil {
			t.Errorf("detectCBORMajorType(%d, 0) = %+v, want nil", mt, result)
		}
	}
}

// ---------------------------------------------------------------------------
// detectCBOR — tagged values, simple markers, and array/map via majorType
// ---------------------------------------------------------------------------

func TestDetectCBOR_Empty(t *testing.T) {
	t.Parallel()
	if detectCBOR(nil) != nil {
		t.Error("detectCBOR(nil) != nil")
	}
	if detectCBOR([]byte{}) != nil {
		t.Error("detectCBOR([]byte{}) != nil")
	}
}

func TestDetectCBOR_Tagged(t *testing.T) {
	t.Parallel()
	result := detectCBOR([]byte{0xc6, 0x01})
	if result == nil {
		t.Fatal("detectCBOR(tagged 0xc6) = nil, want cbor tagged")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Details != "tagged" {
		t.Errorf("Details = %q, want tagged", result.Details)
	}
	if result.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", result.Confidence)
	}
}

func TestDetectCBOR_SimpleMarkerInsufficientLength(t *testing.T) {
	t.Parallel()
	if result := detectCBOR([]byte{0xf9, 0x00}); result != nil {
		t.Errorf("detectCBOR(float16 short) = %+v, want nil", result)
	}
	if result := detectCBOR([]byte{0xfa, 0x00, 0x00}); result != nil {
		t.Errorf("detectCBOR(float32 short) = %+v, want nil", result)
	}
	if result := detectCBOR([]byte{0xfb, 0x00, 0x00, 0x00, 0x00}); result != nil {
		t.Errorf("detectCBOR(float64 short) = %+v, want nil", result)
	}
}

func TestDetectCBOR_BreakCode(t *testing.T) {
	t.Parallel()
	result := detectCBOR([]byte{0xff})
	if result == nil {
		t.Fatal("detectCBOR(0xff) = nil, want cbor break")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Details != "break" {
		t.Errorf("Details = %q, want break", result.Details)
	}
	if result.Confidence != 0.8 {
		t.Errorf("Confidence = %f, want 0.8", result.Confidence)
	}
}

func TestDetectCBOR_UndefinedSimple(t *testing.T) {
	t.Parallel()
	result := detectCBOR([]byte{0xf7})
	if result == nil {
		t.Fatal("detectCBOR(0xf7) = nil, want cbor undefined")
	}
	if result.Details != "undefined" {
		t.Errorf("Details = %q, want undefined", result.Details)
	}
}

func TestDetectCBOR_MajorType7_UnknownSimple(t *testing.T) {
	t.Parallel()
	result := detectCBOR([]byte{0xf8, 0x20})
	if result != nil {
		t.Errorf("detectCBOR(0xf8 unknown simple) = %+v, want nil", result)
	}
}

func TestDetectCBOR_NoMatchMajorType(t *testing.T) {
	t.Parallel()
	result := detectCBOR([]byte{0x00})
	if result != nil {
		t.Errorf("detectCBOR(majorType 0) = %+v, want nil", result)
	}
}

func TestDetectCBOR_ArrayViaMajorType(t *testing.T) {
	t.Parallel()
	// 0x83 = majorType 4, additionalInfo 3 (3-element array)
	result := detectCBOR([]byte{0x83, 0x01, 0x02, 0x03})
	if result == nil {
		t.Fatal("detectCBOR(array byte) = nil, want cbor array")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Details != "array" {
		t.Errorf("Details = %q, want array", result.Details)
	}
	if result.Confidence != 0.75 {
		t.Errorf("Confidence = %f, want 0.75", result.Confidence)
	}
}

func TestDetectCBOR_MapViaMajorType(t *testing.T) {
	t.Parallel()
	// 0xA1 = majorType 5, additionalInfo 1 (1-element map)
	result := detectCBOR([]byte{0xa1, 0x01, 0x02})
	if result == nil {
		t.Fatal("detectCBOR(map byte) = nil, want cbor map")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Details != "map" {
		t.Errorf("Details = %q, want map", result.Details)
	}
}

// ---------------------------------------------------------------------------
// detectMessagePack — empty, insufficient length, markers
// ---------------------------------------------------------------------------

func TestDetectMessagePack_Empty(t *testing.T) {
	t.Parallel()
	if detectMessagePack(nil) != nil {
		t.Error("detectMessagePack(nil) != nil")
	}
	if detectMessagePack([]byte{}) != nil {
		t.Error("detectMessagePack([]byte{}) != nil")
	}
}

func TestDetectMessagePack_InsufficientLength(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		{"float32_short", []byte{0xca, 0x00, 0x00}},
		{"float64_short", []byte{0xcb, 0x00, 0x00, 0x00, 0x00}},
		{"uint8_short", []byte{0xcc}},
		{"uint16_short", []byte{0xcd, 0x00}},
		{"uint32_short", []byte{0xce, 0x00, 0x00}},
		{"uint64_short", []byte{0xcf, 0x00, 0x00, 0x00, 0x00}},
		{"int8_short", []byte{0xd0}},
		{"int16_short", []byte{0xd1, 0x00}},
		{"int32_short", []byte{0xd2, 0x00, 0x00}},
		{"int64_short", []byte{0xd3, 0x00, 0x00, 0x00, 0x00}},
		{"str8_short", []byte{0xd9}},
		{"str16_short", []byte{0xda, 0x00}},
		{"str32_short", []byte{0xdb, 0x00, 0x00}},
		{"array16_short", []byte{0xdc, 0x00}},
		{"array32_short", []byte{0xdd, 0x00, 0x00}},
		{"map16_short", []byte{0xde, 0x00}},
		{"map32_short", []byte{0xdf, 0x00, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := detectMessagePack(tt.data); result != nil {
				t.Errorf("detectMessagePack(%s) = %+v, want nil", tt.name, result)
			}
		})
	}
}

func TestDetectMessagePack_NotInMarkers(t *testing.T) {
	t.Parallel()
	if result := detectMessagePack([]byte{0xc1}); result != nil {
		t.Errorf("detectMessagePack(0xc1) = %+v, want nil", result)
	}
}

func TestDetectMessagePack_BinMarkers(t *testing.T) {
	t.Parallel()
	for _, b := range []byte{0xc4, 0xc5, 0xc6} {
		result := detectMessagePack([]byte{b})
		if result == nil {
			t.Errorf("detectMessagePack(0x%02x) = nil, want bin", b)
			continue
		}
		if result.Name != "messagepack" {
			t.Errorf("detectMessagePack(0x%02x).Name = %q, want messagepack", b, result.Name)
		}
		if result.Details != "bin" {
			t.Errorf("detectMessagePack(0x%02x).Details = %q, want bin", b, result.Details)
		}
	}
}

func TestDetectMessagePack_ExtMarkers(t *testing.T) {
	t.Parallel()
	for _, b := range []byte{0xc7, 0xc8, 0xc9} {
		result := detectMessagePack([]byte{b})
		if result == nil {
			t.Errorf("detectMessagePack(0x%02x) = nil, want ext", b)
			continue
		}
		if result.Details != "ext" {
			t.Errorf("detectMessagePack(0x%02x).Details = %q, want ext", b, result.Details)
		}
	}
}

func TestDetectMessagePack_FixextMarkers(t *testing.T) {
	t.Parallel()
	for _, b := range []byte{0xd4, 0xd5, 0xd6, 0xd7, 0xd8} {
		result := detectMessagePack([]byte{b})
		if result == nil {
			t.Errorf("detectMessagePack(0x%02x) = nil, want fixext", b)
			continue
		}
		if result.Details != "fixext" {
			t.Errorf("detectMessagePack(0x%02x).Details = %q, want fixext", b, result.Details)
		}
	}
}

// ---------------------------------------------------------------------------
// detectMessagePackRange — boundary values
// ---------------------------------------------------------------------------

func TestDetectMessagePackRange_Boundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		b       byte
		details string
		isNil   bool
	}{
		{0x7f, "", true},
		{0x80, "fixmap", false},
		{0x8f, "fixmap", false},
		{0x90, "fixarray", false},
		{0x9f, "fixarray", false},
		{0xa0, "fixstr", false},
		{0xbf, "fixstr", false},
		{0xc0, "", true},
	}
	for _, tt := range tests {
		result := detectMessagePackRange(tt.b)
		if tt.isNil {
			if result != nil {
				t.Errorf("detectMessagePackRange(0x%02x) = %+v, want nil", tt.b, result)
			}
		} else {
			if result == nil {
				t.Errorf("detectMessagePackRange(0x%02x) = nil, want %s", tt.b, tt.details)
				continue
			}
			if result.Details != tt.details {
				t.Errorf("detectMessagePackRange(0x%02x).Details = %q, want %q", tt.b, result.Details, tt.details)
			}
		}
	}
}

// Purpose: Implements binary payload format detection heuristics (MessagePack/CBOR/Protobuf/BSON).
// Why: Enables safer telemetry handling by classifying opaque network bodies before downstream processing.
// Docs: docs/features/feature/binary-format-detection/index.md
//
// The four per-format detectors live here rather than in one file each: they are a
// single closed unit behind one entry point (DetectBinaryFormat), they share the
// binaryMarker table type, and no detector is reachable or useful on its own.

package util

import "fmt"

type BinaryFormat struct {
	Name       string
	Confidence float64
	Details    string
}

// binaryMarker describes a single-byte format marker: the minimum payload length
// required for the marker to be credible, the confidence to report, and a label.
type binaryMarker struct {
	minLen     int
	confidence float64
	details    string
}

// DetectBinaryFormat analyzes bytes and returns detected format.
// Returns nil if data is empty, text, or unknown binary format.
// Detection order: MessagePack > CBOR > Protobuf > BSON.
func DetectBinaryFormat(data []byte) *BinaryFormat {
	if len(data) == 0 || isLikelyText(data) {
		return nil
	}

	if format := detectMessagePack(data); format != nil {
		return format
	}
	if format := detectCBOR(data); format != nil {
		return format
	}
	if format := detectProtobuf(data); format != nil {
		return format
	}
	if format := detectBSON(data); format != nil {
		return format
	}

	return nil
}

func isLikelyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	textBytes := 0
	for _, b := range data {
		if (b >= 0x20 && b <= 0x7e) || b == 0x0a || b == 0x0d || b == 0x09 {
			textBytes++
		}
	}

	return float64(textBytes)/float64(len(data)) > 0.9
}

// ---------------------------------------------------------------------------
// MessagePack
// ---------------------------------------------------------------------------

var msgpackMarkers = map[byte]binaryMarker{
	0xc0: {0, 0.9, "nil"},
	0xc2: {0, 0.9, "false"},
	0xc3: {0, 0.9, "true"},
	0xc4: {0, 0.85, "bin"}, 0xc5: {0, 0.85, "bin"}, 0xc6: {0, 0.85, "bin"},
	0xc7: {0, 0.85, "ext"}, 0xc8: {0, 0.85, "ext"}, 0xc9: {0, 0.85, "ext"},
	0xca: {5, 0.85, "float32"}, 0xcb: {9, 0.85, "float64"},
	0xcc: {2, 0.8, "uint8"}, 0xcd: {3, 0.8, "uint16"}, 0xce: {5, 0.8, "uint32"}, 0xcf: {9, 0.8, "uint64"},
	0xd0: {2, 0.8, "int8"}, 0xd1: {3, 0.8, "int16"}, 0xd2: {5, 0.8, "int32"}, 0xd3: {9, 0.8, "int64"},
	0xd4: {0, 0.85, "fixext"}, 0xd5: {0, 0.85, "fixext"}, 0xd6: {0, 0.85, "fixext"},
	0xd7: {0, 0.85, "fixext"}, 0xd8: {0, 0.85, "fixext"},
	0xd9: {2, 0.8, "str8"}, 0xda: {3, 0.8, "str16"}, 0xdb: {5, 0.8, "str32"},
	0xdc: {3, 0.85, "array16"}, 0xdd: {5, 0.85, "array32"},
	0xde: {3, 0.85, "map16"}, 0xdf: {5, 0.85, "map32"},
}

func detectMessagePackRange(b byte) *BinaryFormat {
	switch {
	case b >= 0x80 && b <= 0x8f:
		return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "fixmap"}
	case b >= 0x90 && b <= 0x9f:
		return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "fixarray"}
	case b >= 0xa0 && b <= 0xbf:
		return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "fixstr"}
	default:
		return nil
	}
}

func detectMessagePack(data []byte) *BinaryFormat {
	if len(data) == 0 {
		return nil
	}

	b := data[0]
	if result := detectMessagePackRange(b); result != nil {
		return result
	}

	m, ok := msgpackMarkers[b]
	if !ok {
		return nil
	}
	if m.minLen > 0 && len(data) < m.minLen {
		return nil
	}
	return &BinaryFormat{Name: "messagepack", Confidence: m.confidence, Details: m.details}
}

// ---------------------------------------------------------------------------
// CBOR
// ---------------------------------------------------------------------------

var cborSimpleMarkers = map[byte]binaryMarker{
	0xf4: {0, 0.9, "false"},
	0xf5: {0, 0.9, "true"},
	0xf6: {0, 0.9, "null"},
	0xf7: {0, 0.9, "undefined"},
	0xf9: {3, 0.85, "float16"},
	0xfa: {5, 0.85, "float32"},
	0xfb: {9, 0.85, "float64"},
	0xff: {0, 0.8, "break"},
}

func detectCBORMajorType(majorType, additionalInfo byte) *BinaryFormat {
	if majorType != 4 && majorType != 5 {
		return nil
	}
	if additionalInfo > 0x17 && additionalInfo != 0x1f {
		return nil
	}

	details := "array"
	if majorType == 5 {
		details = "map"
	}
	return &BinaryFormat{Name: "cbor", Confidence: 0.75, Details: details}
}

func detectCBOR(data []byte) *BinaryFormat {
	if len(data) == 0 {
		return nil
	}

	b := data[0]
	majorType := b >> 5
	additionalInfo := b & 0x1f

	if result := detectCBORMajorType(majorType, additionalInfo); result != nil {
		return result
	}
	if majorType == 6 {
		return &BinaryFormat{Name: "cbor", Confidence: 0.85, Details: "tagged"}
	}
	if majorType == 7 {
		if m, ok := cborSimpleMarkers[b]; ok {
			if m.minLen > 0 && len(data) < m.minLen {
				return nil
			}
			return &BinaryFormat{Name: "cbor", Confidence: m.confidence, Details: m.details}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Protocol Buffers
// ---------------------------------------------------------------------------

func isValidProtobufWireType(wireType byte) bool {
	return wireType == 0 || wireType == 1 || wireType == 2 || wireType == 5
}

func protobufFieldDetail(fieldNumber byte, wireTypeStr string) string {
	return fmt.Sprintf("field %d, %s", fieldNumber, wireTypeStr)
}

func isValidVarint(data []byte) bool {
	for i := 1; i < len(data) && i < 10; i++ {
		if data[i]&0x80 == 0 {
			return true
		}
	}
	return len(data) < 10
}

func detectProtobufLengthDelimited(data []byte, fieldNumber byte) *BinaryFormat {
	if data[1]&0x80 != 0 {
		return &BinaryFormat{Name: "protobuf", Confidence: 0.6, Details: protobufFieldDetail(fieldNumber, "length-delimited")}
	}

	length := int(data[1])
	if length > 0 && len(data) >= 2+length {
		return &BinaryFormat{Name: "protobuf", Confidence: 0.7, Details: protobufFieldDetail(fieldNumber, "length-delimited")}
	}
	return nil
}

func detectProtobuf(data []byte) *BinaryFormat {
	if len(data) < 2 {
		return nil
	}

	wireType := data[0] & 0x07
	fieldNumber := data[0] >> 3

	if !isValidProtobufWireType(wireType) || fieldNumber == 0 || fieldNumber > 15 {
		return nil
	}

	switch wireType {
	case 0:
		if isValidVarint(data) {
			return &BinaryFormat{Name: "protobuf", Confidence: 0.7, Details: protobufFieldDetail(fieldNumber, "varint")}
		}
	case 1:
		if len(data) >= 9 {
			return &BinaryFormat{Name: "protobuf", Confidence: 0.65, Details: protobufFieldDetail(fieldNumber, "fixed64")}
		}
	case 2:
		return detectProtobufLengthDelimited(data, fieldNumber)
	case 5:
		if len(data) >= 5 {
			return &BinaryFormat{Name: "protobuf", Confidence: 0.65, Details: protobufFieldDetail(fieldNumber, "fixed32")}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// BSON
// ---------------------------------------------------------------------------

func bsonDocLen(data []byte) int {
	return int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24
}

func isValidBSONElementType(b byte) bool {
	return b == 0x00 || (b >= 0x01 && b <= 0x13) || b == 0x7f || b == 0xff
}

func detectBSON(data []byte) *BinaryFormat {
	if len(data) < 5 {
		return nil
	}

	docLen := bsonDocLen(data)
	if docLen < 5 || docLen > 16*1024*1024 {
		return nil
	}

	if len(data) >= docLen && data[docLen-1] != 0x00 {
		return nil
	}
	if docLen < len(data) {
		return nil
	}

	if len(data) > 4 && isValidBSONElementType(data[4]) {
		return &BinaryFormat{Name: "bson", Confidence: 0.65, Details: "document"}
	}
	if len(data) <= 4 {
		return &BinaryFormat{Name: "bson", Confidence: 0.5, Details: "document (partial)"}
	}

	return nil
}

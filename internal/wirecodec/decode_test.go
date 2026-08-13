// decode_test.go — Pins the wire decode invariants that ordinary Unmarshal cannot express.

package wirecodec

import (
	"errors"
	"testing"
)

type sampleWire struct {
	TabID   int    `json:"tab_id"`
	TraceID string `json:"trace_id"`
	Count   int    `json:"count,omitempty"`
	skipped string
	Ignored string `json:"-"`
}

type embeddedWire struct {
	sampleWire
	Extra string `json:"extra"`
}

type errorCarryingWire struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// The defect this package exists to prevent: the extension reports failure
// in-band as a JSON body with an error key, no transport error and no isError
// flag. encoding/json ignores unknown fields, so that body decodes cleanly into
// a zero value and a dead service worker becomes "nothing matched".
func TestInto_RejectsInBandErrorEnvelope(t *testing.T) {
	var got sampleWire
	err := Into([]byte(`{"error":"computed_styles_failed","message":"no active tab"}`), &got)
	if err == nil {
		t.Fatal("an in-band error envelope decoded as success")
	}
	var remote *RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("err = %v (%T), want *RemoteError", err, err)
	}
	if remote.Code != "computed_styles_failed" {
		t.Errorf("Code = %q, want computed_styles_failed", remote.Code)
	}
	if remote.Message != "no active tab" {
		t.Errorf("Message = %q, want 'no active tab'", remote.Message)
	}
	if got.TabID != 0 || got.TraceID != "" {
		t.Errorf("destination was populated from a failure body: %+v", got)
	}
}

func TestInto_ErrorEnvelopeWithoutMessageStillReportsTheCode(t *testing.T) {
	var got sampleWire
	err := Into([]byte(`{"error":"timeout"}`), &got)
	var remote *RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("err = %v, want *RemoteError", err)
	}
	if remote.Code != "timeout" || remote.Message != "" {
		t.Errorf("remote = %+v, want code timeout and no message", remote)
	}
}

// A wire type that genuinely declares an error field owns that key. Hijacking it
// would make the field undecodable by the only type that documents it.
func TestInto_ErrorFieldBelongingToTheTypeIsDecodedNotHijacked(t *testing.T) {
	var got errorCarryingWire
	if err := Into([]byte(`{"error":"reported","message":"by design"}`), &got); err != nil {
		t.Fatalf("Into() = %v, want the declared error field to decode", err)
	}
	if got.Error != "reported" || got.Message != "by design" {
		t.Errorf("got = %+v, want the body decoded into the declared fields", got)
	}
}

// The general form of the same defect: any body sharing no keys with the target
// yields a zero value that reads as a legitimate empty result.
func TestInto_RejectsPayloadWhereNoKnownFieldWasObserved(t *testing.T) {
	var got sampleWire
	err := Into([]byte(`{"status":"queued","correlation_id":"abc"}`), &got)
	if err == nil {
		t.Fatal("a payload sharing no fields with the target decoded as success")
	}
	var remote *RemoteError
	if errors.As(err, &remote) {
		t.Fatalf("err = %v, want a shape error, not RemoteError", err)
	}
	if !contains(err.Error(), "status") || !contains(err.Error(), "tab_id") {
		t.Errorf("err = %q, want it to name both the keys seen and the keys expected", err)
	}
}

// Version skew is normal and must stay decodable: an extension ahead of the
// daemon sends fields Go has not learned yet. Strict unknown-field rejection
// would turn every forward-compatible rollout into an outage, so the invariant
// is "at least one field was understood", not "every field was understood".
func TestInto_AcceptsUnknownFieldsAlongsideKnownOnes(t *testing.T) {
	var got sampleWire
	if err := Into([]byte(`{"tab_id":7,"future_field":true}`), &got); err != nil {
		t.Fatalf("Into() = %v, want unknown fields tolerated beside a known one", err)
	}
	if got.TabID != 7 {
		t.Errorf("TabID = %d, want 7", got.TabID)
	}
}

// An empty object carries no claim about shape, so there is nothing to reject.
func TestInto_AcceptsEmptyObject(t *testing.T) {
	var got sampleWire
	if err := Into([]byte(`{}`), &got); err != nil {
		t.Fatalf("Into() = %v, want an empty object accepted", err)
	}
}

func TestInto_CountsFieldsPromotedFromEmbeddedStructs(t *testing.T) {
	var got embeddedWire
	if err := Into([]byte(`{"trace_id":"t-1"}`), &got); err != nil {
		t.Fatalf("Into() = %v, want an embedded field to count as recognized", err)
	}
	if got.TraceID != "t-1" {
		t.Errorf("TraceID = %q, want t-1", got.TraceID)
	}
}

// json:"-" opts a field out of the wire contract, so a body naming it has still
// not been understood.
func TestInto_IgnoresFieldsOptedOutOfJSON(t *testing.T) {
	var got sampleWire
	if err := Into([]byte(`{"Ignored":"x"}`), &got); err == nil {
		t.Fatal("a body naming only a json:\"-\" field decoded as success")
	}
}

func TestInto_RejectsTrailingData(t *testing.T) {
	var got sampleWire
	if err := Into([]byte(`{"tab_id":1}{"tab_id":2}`), &got); err == nil {
		t.Fatal("two concatenated JSON values decoded as success")
	}
}

func TestInto_RejectsNonObjectBodyForStructTargets(t *testing.T) {
	var got sampleWire
	if err := Into([]byte(`[{"tab_id":1}]`), &got); err == nil {
		t.Fatal("a JSON array decoded into a struct target as success")
	}
}

func TestInto_RejectsEmptyInput(t *testing.T) {
	var got sampleWire
	if err := Into(nil, &got); err == nil {
		t.Fatal("an empty body decoded as success")
	}
}

func TestInto_RejectsMalformedJSON(t *testing.T) {
	var got sampleWire
	if err := Into([]byte(`{"tab_id":`), &got); err == nil {
		t.Fatal("truncated JSON decoded as success")
	}
}

// Slice targets have no field set to intersect, so they are checked for
// syntax and trailing data only.
func TestInto_AcceptsSliceTargets(t *testing.T) {
	var got []sampleWire
	if err := Into([]byte(`[{"tab_id":3}]`), &got); err != nil {
		t.Fatalf("Into() = %v, want a slice target accepted", err)
	}
	if len(got) != 1 || got[0].TabID != 3 {
		t.Errorf("got = %+v, want one element with TabID 3", got)
	}
}

func TestDecode_ReturnsTheTypedValue(t *testing.T) {
	got, err := Decode[sampleWire]([]byte(`{"tab_id":9,"trace_id":"t"}`))
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	if got.TabID != 9 || got.TraceID != "t" {
		t.Errorf("got = %+v, want TabID 9 and TraceID t", got)
	}
}

func TestDecode_ReturnsTheZeroValueOnFailure(t *testing.T) {
	got, err := Decode[sampleWire]([]byte(`{"error":"boom"}`))
	if err == nil {
		t.Fatal("Decode() accepted an error envelope")
	}
	if got.TabID != 0 || got.TraceID != "" {
		t.Errorf("got = %+v, want the zero value on failure", got)
	}
}

func TestInto_RequiresAPointerDestination(t *testing.T) {
	if err := Into([]byte(`{"tab_id":1}`), sampleWire{}); err == nil {
		t.Fatal("a non-pointer destination was accepted")
	}
}

func TestRemoteError_MessageNamesBothPieces(t *testing.T) {
	err := &RemoteError{Code: "probe_failed", Message: "detached frame"}
	if !contains(err.Error(), "probe_failed") || !contains(err.Error(), "detached frame") {
		t.Errorf("Error() = %q, want both the code and the message", err.Error())
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

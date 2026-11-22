// decode.go — The one way to decode a peer's JSON into a wire type.
//
// PURPOSE: encoding/json ignores fields it does not recognize, so a body that
// shares no keys with the destination decodes without error into a zero value.
// Every caller then reads that zero value as a legitimate empty result. That is
// how a dead service worker was reported as a clean design audit, and how an
// error envelope POSTed to the trace endpoints became a trace for tab 0.
//
// CONTRACT: a decode succeeds only if the peer said something this build
// understands. Unknown fields are tolerated — an extension ahead of the daemon
// is normal and must keep working — but a body where *nothing* was understood
// is a failure, not an empty result. Rule 25: a real failure must not be masked
// as an expected state.

package wirecodec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

// errorKey is how the extension reports an in-band failure: a plain JSON body
// carrying this key, with no transport error and no isError flag.
const errorKey = "error"

// messageKey carries the human-readable detail alongside errorKey.
const messageKey = "message"

// maxReportedKeys bounds the key lists in error messages so a large unexpected
// payload produces a diagnostic, not a dump.
const maxReportedKeys = 6

// RemoteError is a failure the peer reported inside an otherwise well-formed
// JSON body. It is returned instead of a decoded value so the caller cannot
// mistake it for data.
type RemoteError struct {
	Code    string
	Message string
}

func (e *RemoteError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("the peer reported %s", e.Code)
	}
	return fmt.Sprintf("the peer reported %s: %s", e.Code, e.Message)
}

// Decode is Into for callers that want the value back rather than filling one.
// The zero value is returned on any failure, so a caller that ignores the error
// gets an obviously empty struct rather than a half-populated one.
func Decode[T any](raw []byte) (T, error) {
	var value T
	if err := Into(raw, &value); err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}

// Into decodes raw into dst, which must be a non-nil pointer.
func Into(raw []byte, dst any) error {
	if err := requirePointer(dst); err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return errors.New("the payload was empty")
	}

	known := knownKeys(dst)
	// The shape probe runs first so an error envelope is named as such rather
	// than reported as an unrecognized payload.
	if known != nil {
		if err := inspectObject(raw, known); err != nil {
			return err
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("the payload was not the expected shape: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("the payload must contain exactly one JSON value")
	}
	return nil
}

func requirePointer(dst any) error {
	value := reflect.ValueOf(dst)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return fmt.Errorf("decode destination must be a non-nil pointer, got %T", dst)
	}
	return nil
}

// inspectObject enforces the two shape rules that plain Unmarshal cannot: an
// in-band error envelope is a failure, and a body that shares no key with the
// destination was not understood.
func inspectObject(raw []byte, known map[string]bool) error {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		// EXPECTED_ABSENCE: a body that is valid JSON but not an object (an
		// array, a bare string) fails here with an unhelpful type error. The
		// real decode below produces the message that names the target type,
		// so this path defers to it rather than guessing.
		return nil
	}
	if len(body) == 0 {
		return nil
	}

	// A type that declares its own error field owns that key; hijacking it
	// would make the field undecodable by the only type that documents it.
	if !known[errorKey] {
		if remote, reported := remoteFailure(body); reported {
			return remote
		}
	}

	for key := range body {
		if known[key] {
			return nil
		}
	}
	return fmt.Errorf("the payload was not recognized: it carries %s but this build expects %s",
		summarize(keysOf(body)), summarize(keysOf(known)))
}

func remoteFailure(body map[string]json.RawMessage) (*RemoteError, bool) {
	code, present := stringAt(body, errorKey)
	if !present || code == "" {
		// EXPECTED_ABSENCE: a successful payload carries no error key, and an
		// empty or non-string one is not a failure report.
		return nil, false
	}
	message, _ := stringAt(body, messageKey)
	return &RemoteError{Code: code, Message: message}, true
}

func stringAt(body map[string]json.RawMessage, key string) (string, bool) {
	raw, present := body[key]
	if !present {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		// EXPECTED_ABSENCE: a non-string value under this key is not a failure
		// report, and the real decode reports any type mismatch that matters.
		return "", false
	}
	return value, true
}

// knownKeys returns the JSON key set the destination understands, or nil when
// the destination is not a struct and so has no key set to compare against.
func knownKeys(dst any) map[string]bool {
	target := reflect.TypeOf(dst).Elem()
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	if target.Kind() != reflect.Struct {
		return nil
	}
	keys := make(map[string]bool)
	collectKeys(target, keys)
	if len(keys) == 0 {
		return nil
	}
	return keys
}

// collectKeys walks the struct, following anonymous embedded structs because
// encoding/json promotes their fields to the top level.
func collectKeys(target reflect.Type, keys map[string]bool) {
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		tag := field.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")

		if field.Anonymous && name == "" {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				collectKeys(embedded, keys)
				continue
			}
		}
		if !field.IsExported() || name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		keys[name] = true
	}
}

func keysOf[V any](set map[string]V) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// summarize renders a bounded, order-stable key list for an error message.
func summarize(names []string) string {
	if len(names) == 0 {
		return "no fields"
	}
	if len(names) <= maxReportedKeys {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:maxReportedKeys], ", "), len(names)-maxReportedKeys)
}

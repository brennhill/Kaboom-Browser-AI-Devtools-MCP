// fault_test.go — Defines the canonical deterministic persisted-state fault contract.
package statefault

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestKindsAreStableAndExhaustive(t *testing.T) {
	want := []Kind{Read, Write, Sync, Rename, DirectorySync, Quota, Corruption, PartialWrite, Cancellation, Restart}
	if !reflect.DeepEqual(Kinds(), want) {
		t.Fatalf("Kinds() = %#v, want %#v", Kinds(), want)
	}
	for _, kind := range want {
		if !Valid(kind) {
			t.Fatalf("canonical kind %q is not valid", kind)
		}
	}
}

func TestFaultErrorNeverLeaksPrivateSentinel(t *testing.T) {
	const private = "private-persisted-value"
	for _, kind := range Kinds() {
		err := New(kind, private).Error()
		if err == nil || !errors.Is(err, ErrInjected) {
			t.Fatalf("Error(%q) = %v, want injected fault", kind, err)
		}
		if strings.Contains(err.Error(), private) {
			t.Fatalf("Error(%q) leaked private sentinel", kind)
		}
	}
}

func TestPayloadFaultsAreDeterministicAndDoNotAliasInput(t *testing.T) {
	valid := []byte(`{"version":1,"private":"private-persisted-value"}`)
	original := append([]byte(nil), valid...)
	for _, kind := range Kinds() {
		scenario := New(kind, "private-persisted-value")
		first := scenario.Payload(valid)
		second := scenario.Payload(valid)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("Payload(%q) is not deterministic", kind)
		}
		if len(first) > 0 {
			first[0] ^= 0xff
		}
		if !reflect.DeepEqual(valid, original) {
			t.Fatalf("Payload(%q) aliases its input", kind)
		}
	}
}

func TestCancellationAndRestartAreExplicitTransitions(t *testing.T) {
	ctx := New(Cancellation, "private").Context(context.Background())
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("cancellation context error = %v", ctx.Err())
	}
	if got := New(Restart, "private").NextGeneration(41); got != 42 {
		t.Fatalf("restart generation = %d, want 42", got)
	}
	if got := New(Read, "private").NextGeneration(41); got != 41 {
		t.Fatalf("non-restart generation = %d, want 41", got)
	}
}

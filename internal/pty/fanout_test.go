// fanout_test.go — Tests for multi-subscriber fan-out.

package pty

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestFanout_SubscribeAndBroadcast(t *testing.T) {
	f := NewFanout()
	defer f.Close()

	ch1, err := f.Subscribe("sub-1")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	ch2, err := f.Subscribe("sub-2")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	f.Broadcast([]byte("hello"))

	for _, tc := range []struct {
		name string
		ch   <-chan []byte
	}{{"sub-1", ch1}, {"sub-2", ch2}} {
		select {
		case msg := <-tc.ch:
			if string(msg) != "hello" {
				t.Fatalf("%s: expected %q, got %q", tc.name, "hello", string(msg))
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout on %s", tc.name)
		}
	}
}

// Subscribe used to overwrite a duplicate id's map entry WITHOUT closing the old
// channel: the first subscriber's pump then blocked forever on a channel nothing
// would ever write to or close, and the first Unsubscribe closed the SECOND
// subscriber's channel out from under it. The guard lived in one caller
// (WaitForPromptViaRelay's unique ids); per rule 19 the invariant belongs in the
// primitive, where no caller can forget it (finding S7).
func TestFanout_SubscribeRejectsDuplicateID(t *testing.T) {
	f := NewFanout()
	defer f.Close()

	first, err := f.Subscribe("dup")
	if err != nil {
		t.Fatalf("first subscribe: %v", err)
	}

	second, err := f.Subscribe("dup")
	if !errors.Is(err, ErrDuplicateSubscriber) {
		t.Fatalf("duplicate subscribe: got %v, want ErrDuplicateSubscriber", err)
	}
	if second != nil {
		t.Fatal("a rejected subscribe must not hand back a channel")
	}
	if f.Count() != 1 {
		t.Fatalf("Count()=%d, want 1 — the duplicate must not replace the incumbent", f.Count())
	}

	// The incumbent is untouched: still registered, still receiving.
	f.Broadcast([]byte("hello"))
	select {
	case msg := <-first:
		if string(msg) != "hello" {
			t.Fatalf("incumbent got %q, want %q", msg, "hello")
		}
	case <-time.After(time.Second):
		t.Fatal("the incumbent subscriber was orphaned by the duplicate Subscribe")
	}
}

func TestFanout_Unsubscribe(t *testing.T) {
	f := NewFanout()
	defer f.Close()

	ch, err := f.Subscribe("sub-1")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	f.Unsubscribe("sub-1")

	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed after unsubscribe")
	}
	if f.Count() != 0 {
		t.Fatalf("expected 0 subscribers, got %d", f.Count())
	}
}

func TestFanout_SlowSubscriberDropped(t *testing.T) {
	f := NewFanout()
	defer f.Close()

	ch, err := f.Subscribe("slow")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Fill the subscriber's channel buffer.
	for i := 0; i < subscriberBufSize; i++ {
		f.Broadcast([]byte("msg"))
	}

	// Next broadcast should drop the slow subscriber.
	f.Broadcast([]byte("overflow"))

	if f.Count() != 0 {
		t.Fatalf("expected slow subscriber to be dropped, got count=%d", f.Count())
	}

	// Drain and verify channel is closed.
	for range ch {
	}
}

func TestFanout_Close(t *testing.T) {
	f := NewFanout()
	ch, err := f.Subscribe("sub-1")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	f.Close()

	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed after fanout close")
	}

	_, err = f.Subscribe("sub-2")
	if err != ErrFanoutClosed {
		t.Fatalf("expected ErrFanoutClosed, got: %v", err)
	}
}

func TestFanout_MaxSubscribers(t *testing.T) {
	f := NewFanout()
	defer f.Close()

	for i := 0; i < maxSubscribers; i++ {
		_, err := f.Subscribe(fmt.Sprintf("sub-%d", i))
		if err != nil {
			t.Fatalf("subscribe %d: %v", i, err)
		}
	}

	_, err := f.Subscribe("overflow")
	if err != ErrFanoutFull {
		t.Fatalf("expected ErrFanoutFull, got: %v", err)
	}
}

func TestFanout_ConcurrentBroadcast(t *testing.T) {
	f := NewFanout()
	defer f.Close()

	ch, err := f.Subscribe("sub-1")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	var wg sync.WaitGroup
	const broadcasts = 100
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < broadcasts; i++ {
			f.Broadcast([]byte("data"))
		}
	}()

	received := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
			received++
		}
	}()

	wg.Wait()
	f.Close()
	<-done

	if received == 0 {
		t.Fatal("expected to receive at least some messages")
	}
}

func TestFanout_DoubleClose(t *testing.T) {
	f := NewFanout()
	f.Close()
	f.Close() // should not panic
}

func TestFanout_UnsubscribeNonexistent(t *testing.T) {
	f := NewFanout()
	defer f.Close()
	f.Unsubscribe("nonexistent") // should not panic
}

func TestFanout_BroadcastDataIsolation(t *testing.T) {
	f := NewFanout()
	defer f.Close()

	ch, err := f.Subscribe("sub-1")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	original := []byte("hello")
	f.Broadcast(original)

	// Mutate original — subscriber's copy should be unaffected.
	original[0] = 'X'

	msg := <-ch
	if string(msg) != "hello" {
		t.Fatalf("expected %q, got %q (data not isolated)", "hello", string(msg))
	}
}

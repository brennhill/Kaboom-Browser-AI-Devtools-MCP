// clock.go — time abstraction for the PTY session.
// Why: idle detection uses time.Now() + time.AfterFunc; a seam lets tests drive
// idle firing and last-output/-input timestamps deterministically instead of
// with real sleeps (repo rule 9: deterministic over sleep-based timing).

package pty

import "time"

// clock is the time surface the session depends on. Production uses realClock;
// tests inject a fake that fires timers on demand.
type clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) stoppableTimer
}

// stoppableTimer is the subset of *time.Timer the session drives.
type stoppableTimer interface {
	Stop() bool
	Reset(d time.Duration) bool
}

// realClock is the production clock backed by the time package. *time.Timer
// already satisfies stoppableTimer.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) AfterFunc(d time.Duration, f func()) stoppableTimer {
	return time.AfterFunc(d, f)
}

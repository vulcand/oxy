package clock

import "time"

// Timer see time.Timer.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

// Ticker see time.Ticker.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// NewStoppedTimer returns a stopped timer. Call Reset to get it ticking.
func NewStoppedTimer() Timer {
	t := NewTimer(42 * time.Hour)
	t.Stop()
	return t
}

// Clock is an interface that mimics the one of the SDK time package.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
	After(d time.Duration) <-chan time.Time
	NewTimer(d time.Duration) Timer
	AfterFunc(d time.Duration, f func()) Timer
	NewTicker(d time.Duration) Ticker
	Tick(d time.Duration) <-chan time.Time
	Wait4Scheduled(n int, timeout time.Duration) bool
}

package clock

import (
	"errors"
	"sync"
	"time"
)

type frozenTime struct {
	mu     sync.Mutex
	now    time.Time
	timers []*frozenTimer
	waiter *waiter
}

type waiter struct {
	count    int
	signalCh chan struct{}
}

func (ft *frozenTime) Now() time.Time {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.now
}

func (ft *frozenTime) Sleep(d time.Duration) {
	<-ft.NewTimer(d).C()
}

func (ft *frozenTime) After(d time.Duration) <-chan time.Time {
	return ft.NewTimer(d).C()
}

func (ft *frozenTime) NewTimer(d time.Duration) Timer {
	return ft.AfterFunc(d, nil)
}

func (ft *frozenTime) AfterFunc(d time.Duration, f func()) Timer {
	t := &frozenTimer{
		ft:   ft,
		when: ft.Now().Add(d),
		f:    f,
	}
	if f == nil {
		t.c = make(chan time.Time, 1)
	}
	ft.startTimer(t)
	return t
}

func (ft *frozenTime) advance(d time.Duration) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	ft.now = ft.now.Add(d)
	for t := ft.nextExpired(); t != nil; t = ft.nextExpired() {
		// Send the timer expiration time to the timer channel if it is
		// defined. But make sure not to block on the send if the channel is
		// full. This behavior will make a ticker skip beats if it readers are
		// not fast enough.
		if t.c != nil {
			select {
			case t.c <- t.when:
			default:
			}
		}
		// If it is a ticking timer then schedule next tick, otherwise mark it
		// as stopped.
		if t.interval != 0 {
			t.when = t.when.Add(t.interval)
			t.stopped = false
			ft.unlockedStartTimer(t)
		}
		// If a function is associated with the timer then call it, but make
		// sure to release the lock for the time of call it is necessary
		// because the lock is not re-entrant but the function may need to
		// start another timer or ticker.
		if t.f != nil {
			func() {
				ft.mu.Unlock()
				defer ft.mu.Lock()
				t.f()
			}()
		}
	}
}

func (ft *frozenTime) stopTimer(t *frozenTimer) bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	if t.stopped {
		return false
	}
	for i, curr := range ft.timers {
		if curr == t {
			t.stopped = true
			copy(ft.timers[i:], ft.timers[i+1:])
			lastIdx := len(ft.timers) - 1
			ft.timers[lastIdx] = nil
			ft.timers = ft.timers[:lastIdx]
			return true
		}
	}
	return false
}

func (ft *frozenTime) nextExpired() *frozenTimer {
	if len(ft.timers) == 0 {
		return nil
	}
	t := ft.timers[0]
	if ft.now.Before(t.when) {
		return nil
	}
	copy(ft.timers, ft.timers[1:])
	lastIdx := len(ft.timers) - 1
	ft.timers[lastIdx] = nil
	ft.timers = ft.timers[:lastIdx]
	t.stopped = true
	return t
}

func (ft *frozenTime) startTimer(t *frozenTimer) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	ft.unlockedStartTimer(t)

	if ft.waiter == nil {
		return
	}
	if len(ft.timers) >= ft.waiter.count {
		close(ft.waiter.signalCh)
	}
}

func (ft *frozenTime) unlockedStartTimer(t *frozenTimer) {
	pos := 0
	for _, curr := range ft.timers {
		if t.when.Before(curr.when) {
			break
		}
		pos++
	}
	ft.timers = append(ft.timers, nil)
	copy(ft.timers[pos+1:], ft.timers[pos:])
	ft.timers[pos] = t
}

type frozenTimer struct {
	ft       *frozenTime
	when     time.Time
	interval time.Duration
	stopped  bool
	c        chan time.Time
	f        func()
}

func (t *frozenTimer) C() <-chan time.Time {
	return t.c
}

func (t *frozenTimer) Stop() bool {
	return t.ft.stopTimer(t)
}

func (t *frozenTimer) Reset(d time.Duration) bool {
	active := t.ft.stopTimer(t)
	t.when = t.ft.Now().Add(d)
	t.ft.startTimer(t)
	return active
}

type frozenTicker struct {
	t *frozenTimer
}

func (t *frozenTicker) C() <-chan time.Time {
	return t.t.C()
}

func (t *frozenTicker) Stop() {
	t.t.Stop()
}

func (ft *frozenTime) NewTicker(d time.Duration) Ticker {
	if d <= 0 {
		panic(errors.New("non-positive interval for NewTicker"))
	}
	t := &frozenTimer{
		ft:       ft,
		when:     ft.Now().Add(d),
		interval: d,
		c:        make(chan time.Time, 1),
	}
	ft.startTimer(t)
	return &frozenTicker{t}
}

func (ft *frozenTime) Tick(d time.Duration) <-chan time.Time {
	if d <= 0 {
		return nil
	}
	return ft.NewTicker(d).C()
}

func (ft *frozenTime) Wait4Scheduled(count int, timeout time.Duration) bool {
	ft.mu.Lock()
	if len(ft.timers) >= count {
		ft.mu.Unlock()
		return true
	}
	if ft.waiter != nil {
		panic("Concurrent call")
	}
	ft.waiter = &waiter{count, make(chan struct{})}
	ft.mu.Unlock()

	success := false
	select {
	case <-ft.waiter.signalCh:
		success = true
	case <-time.After(timeout):
	}
	ft.mu.Lock()
	ft.waiter = nil
	ft.mu.Unlock()
	return success
}

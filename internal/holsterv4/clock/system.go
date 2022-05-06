package clock

import "time"

type systemTime struct{}

func (st *systemTime) Now() time.Time {
	return time.Now()
}

func (st *systemTime) Sleep(d time.Duration) {
	time.Sleep(d)
}

func (st *systemTime) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

type systemTimer struct {
	t *time.Timer
}

func (st *systemTime) NewTimer(d time.Duration) Timer {
	t := time.NewTimer(d)
	return &systemTimer{t}
}

func (st *systemTime) AfterFunc(d time.Duration, f func()) Timer {
	t := time.AfterFunc(d, f)
	return &systemTimer{t}
}

func (t *systemTimer) C() <-chan time.Time {
	return t.t.C
}

func (t *systemTimer) Stop() bool {
	return t.t.Stop()
}

func (t *systemTimer) Reset(d time.Duration) bool {
	return t.t.Reset(d)
}

type systemTicker struct {
	t *time.Ticker
}

func (t *systemTicker) C() <-chan time.Time {
	return t.t.C
}

func (t *systemTicker) Stop() {
	t.t.Stop()
}

func (st *systemTime) NewTicker(d time.Duration) Ticker {
	t := time.NewTicker(d)
	return &systemTicker{t}
}

func (st *systemTime) Tick(d time.Duration) <-chan time.Time {
	return time.Tick(d)
}

func (st *systemTime) Wait4Scheduled(count int, timeout time.Duration) bool {
	panic("Not supported")
}

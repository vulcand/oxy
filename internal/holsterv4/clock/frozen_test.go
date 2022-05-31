package clock

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

func TestFreezeUnfreeze(t *testing.T) {
	defer Freeze(Now()).Unfreeze()
}

type FrozenSuite struct {
	suite.Suite
	epoch time.Time
}

func TestFrozenSuite(t *testing.T) {
	suite.Run(t, new(FrozenSuite))
}

func (s *FrozenSuite) SetupSuite() {
	var err error
	s.epoch, err = time.Parse(time.RFC3339, "2009-02-19T00:00:00Z")
	s.Require().NoError(err)
}

func (s *FrozenSuite) SetupTest() {
	Freeze(s.epoch)
}

func (s *FrozenSuite) TearDownTest() {
	Unfreeze()
}

func (s *FrozenSuite) TestAdvanceNow() {
	s.Require().Equal(s.epoch, Now())
	s.Require().Equal(42*time.Millisecond, Advance(42*time.Millisecond))
	s.Require().Equal(s.epoch.Add(42*time.Millisecond), Now())
	s.Require().Equal(55*time.Millisecond, Advance(13*time.Millisecond))
	s.Require().Equal(74*time.Millisecond, Advance(19*time.Millisecond))
	s.Require().Equal(s.epoch.Add(74*time.Millisecond), Now())
}

func (s *FrozenSuite) TestSleep() {
	hits := make(chan int, 100)

	delays := []int{60, 100, 90, 131, 999, 5}
	for i, tc := range []struct {
		desc string
		fn   func(delayMs int)
	}{{
		desc: "Sleep",
		fn: func(delay int) {
			Sleep(time.Duration(delay) * time.Millisecond)
			hits <- delay
		},
	}, {
		desc: "After",
		fn: func(delay int) {
			<-After(time.Duration(delay) * time.Millisecond)
			hits <- delay
		},
	}, {
		desc: "AfterFunc",
		fn: func(delay int) {
			AfterFunc(time.Duration(delay)*time.Millisecond,
				func() {
					hits <- delay
				})
		},
	}, {
		desc: "NewTimer",
		fn: func(delay int) {
			t := NewTimer(time.Duration(delay) * time.Millisecond)
			<-t.C()
			hits <- delay
		},
	}} {
		fmt.Printf("Test case #%d: %s", i, tc.desc)
		for _, delay := range delays {
			go tc.fn(delay)
		}
		// Spin-wait for all goroutines to fall asleep.
		ft := provider.(*frozenTime)
		for {
			var brk bool
			ft.mu.Lock()
			if len(ft.timers) == len(delays) {
				brk = true
			}
			ft.mu.Unlock()
			if brk {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		runningMs := 0
		for i, delayMs := range []int{5, 60, 90, 100, 131, 999} {
			fmt.Printf("Checking timer #%d, delay=%d\n", i, delayMs)
			delta := delayMs - runningMs - 1
			Advance(time.Duration(delta) * time.Millisecond)
			// Check before each timer deadline that it is not triggered yet.
			s.assertHits(hits, []int{})

			// When
			Advance(1 * time.Millisecond)

			// Then
			s.assertHits(hits, []int{delayMs})

			runningMs += delta + 1
		}

		Advance(1000 * time.Millisecond)
		s.assertHits(hits, []int{})
	}
}

// Timers scheduled to trigger at the same time do that in the order they were
// created.
func (s *FrozenSuite) TestSameTime() {
	var hits []int

	AfterFunc(100, func() { hits = append(hits, 3) })
	AfterFunc(100, func() { hits = append(hits, 1) })
	AfterFunc(99, func() { hits = append(hits, 2) })
	AfterFunc(100, func() { hits = append(hits, 5) })
	AfterFunc(101, func() { hits = append(hits, 4) })
	AfterFunc(101, func() { hits = append(hits, 6) })

	// When
	Advance(100)

	// Then
	s.Require().Equal([]int{2, 3, 1, 5}, hits)
}

func (s *FrozenSuite) TestTimerStop() {
	hits := []int{}

	AfterFunc(100, func() { hits = append(hits, 1) })
	t := AfterFunc(100, func() { hits = append(hits, 2) })
	AfterFunc(100, func() { hits = append(hits, 3) })
	Advance(99)
	s.Require().Equal([]int{}, hits)

	// When
	active1 := t.Stop()
	active2 := t.Stop()

	// Then
	s.Require().Equal(true, active1)
	s.Require().Equal(false, active2)
	Advance(1)
	s.Require().Equal([]int{1, 3}, hits)
}

func (s *FrozenSuite) TestReset() {
	hits := []int{}

	t1 := AfterFunc(100, func() { hits = append(hits, 1) })
	t2 := AfterFunc(100, func() { hits = append(hits, 2) })
	AfterFunc(100, func() { hits = append(hits, 3) })
	Advance(99)
	s.Require().Equal([]int{}, hits)

	// When
	active1 := t1.Reset(1) // Reset to the same time
	active2 := t2.Reset(7)

	// Then
	s.Require().Equal(true, active1)
	s.Require().Equal(true, active2)

	Advance(1)
	s.Require().Equal([]int{3, 1}, hits)
	Advance(5)
	s.Require().Equal([]int{3, 1}, hits)
	Advance(1)
	s.Require().Equal([]int{3, 1, 2}, hits)
}

// Reset to the same time just puts the timer at the end of the trigger list
// for the date.
func (s *FrozenSuite) TestResetSame() {
	hits := []int{}

	t := AfterFunc(100, func() { hits = append(hits, 1) })
	AfterFunc(100, func() { hits = append(hits, 2) })
	AfterFunc(100, func() { hits = append(hits, 3) })
	AfterFunc(101, func() { hits = append(hits, 4) })
	Advance(9)

	// When
	active := t.Reset(91)

	// Then
	s.Require().Equal(true, active)

	Advance(90)
	s.Require().Equal([]int{}, hits)
	Advance(1)
	s.Require().Equal([]int{2, 3, 1}, hits)
}

func (s *FrozenSuite) TestTicker() {
	t := NewTicker(100)

	Advance(99)
	s.assertNotFired(t.C())
	Advance(1)
	s.Require().Equal(<-t.C(), s.epoch.Add(100))
	Advance(750)
	s.Require().Equal(<-t.C(), s.epoch.Add(200))
	Advance(49)
	s.assertNotFired(t.C())
	Advance(1)
	s.Require().Equal(<-t.C(), s.epoch.Add(900))

	t.Stop()
	Advance(300)
	s.assertNotFired(t.C())
}

func (s *FrozenSuite) TestTickerZero() {
	defer func() {
		recover()
	}()

	NewTicker(0)
	s.Fail("Should panic")
}

func (s *FrozenSuite) TestTick() {
	ch := Tick(100)

	Advance(99)
	s.assertNotFired(ch)
	Advance(1)
	s.Require().Equal(<-ch, s.epoch.Add(100))
	Advance(750)
	s.Require().Equal(<-ch, s.epoch.Add(200))
	Advance(49)
	s.assertNotFired(ch)
	Advance(1)
	s.Require().Equal(<-ch, s.epoch.Add(900))
}

func (s *FrozenSuite) TestTickZero() {
	ch := Tick(0)
	s.Require().Nil(ch)
}

func (s *FrozenSuite) TestNewStoppedTimer() {
	t := NewStoppedTimer()

	// When/Then
	select {
	case <-t.C():
		s.Fail("Timer should not have fired")
	default:
	}
	s.Require().Equal(false, t.Stop())
}

func (s *FrozenSuite) TestWait4Scheduled() {
	After(100 * Millisecond)
	After(100 * Millisecond)
	s.Require().Equal(false, Wait4Scheduled(3, 0))

	startedCh := make(chan struct{})
	resultCh := make(chan bool)
	go func() {
		close(startedCh)
		resultCh <- Wait4Scheduled(3, 5*Second)
	}()
	// Allow some time for waiter to be set and start waiting for a signal.
	<-startedCh
	time.Sleep(50 * Millisecond)

	// When
	After(100 * Millisecond)

	// Then
	s.Require().Equal(true, <-resultCh)
}

// If there is enough timers scheduled already, then a shortcut execution path
// is taken and Wait4Scheduled returns immediately.
func (s *FrozenSuite) TestWait4ScheduledImmediate() {
	After(100 * Millisecond)
	After(100 * Millisecond)
	// When/Then
	s.Require().Equal(true, Wait4Scheduled(2, 0))
}

func (s *FrozenSuite) TestSince() {
	s.Require().Equal(Duration(0), Since(Now()))
	s.Require().Equal(-Millisecond, Since(Now().Add(Millisecond)))
	s.Require().Equal(Millisecond, Since(Now().Add(-Millisecond)))
}

func (s *FrozenSuite) TestUntil() {
	s.Require().Equal(Duration(0), Until(Now()))
	s.Require().Equal(Millisecond, Until(Now().Add(Millisecond)))
	s.Require().Equal(-Millisecond, Until(Now().Add(-Millisecond)))
}

func (s *FrozenSuite) assertHits(got <-chan int, want []int) {
	for i, w := range want {
		var g int
		select {
		case g = <-got:
		case <-time.After(100 * time.Millisecond):
			s.Failf("Missing hit", "want=%v", w)
			return
		}
		s.Require().Equal(w, g, "Hit #%d", i)
	}
	for {
		select {
		case g := <-got:
			s.Failf("Unexpected hit", "got=%v", g)
		default:
			return
		}
	}
}

func (s *FrozenSuite) assertNotFired(ch <-chan time.Time) {
	select {
	case <-ch:
		s.Fail("Premature fire")
	default:
	}
}

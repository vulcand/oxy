package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSleep(t *testing.T) {
	start := Now()

	// When
	Sleep(100 * time.Millisecond)

	// Then
	if Now().Sub(start) < 100*time.Millisecond {
		assert.Fail(t, "Sleep did not last long enough")
	}
}

func TestAfter(t *testing.T) {
	start := Now()

	// When
	end := <-After(100 * time.Millisecond)

	// Then
	if end.Sub(start) < 100*time.Millisecond {
		assert.Fail(t, "Sleep did not last long enough")
	}
}

func TestAfterFunc(t *testing.T) {
	start := Now()
	endCh := make(chan time.Time, 1)

	// When
	AfterFunc(100*time.Millisecond, func() { endCh <- time.Now() })

	// Then
	end := <-endCh
	if end.Sub(start) < 100*time.Millisecond {
		assert.Fail(t, "Sleep did not last long enough")
	}
}

func TestNewTimer(t *testing.T) {
	start := Now()

	// When
	timer := NewTimer(100 * time.Millisecond)

	// Then
	end := <-timer.C()
	if end.Sub(start) < 100*time.Millisecond {
		assert.Fail(t, "Sleep did not last long enough")
	}
}

func TestTimerStop(t *testing.T) {
	timer := NewTimer(50 * time.Millisecond)

	// When
	active := timer.Stop()

	// Then
	assert.Equal(t, true, active)
	time.Sleep(100)
	select {
	case <-timer.C():
		assert.Fail(t, "Timer should not have fired")
	default:
	}
}

func TestTimerReset(t *testing.T) {
	start := time.Now()
	timer := NewTimer(300 * time.Millisecond)

	// When
	timer.Reset(100 * time.Millisecond)

	// Then
	end := <-timer.C()
	if end.Sub(start) > 150*time.Millisecond {
		assert.Fail(t, "Waited too long")
	}
}

func TestNewTicker(t *testing.T) {
	start := Now()

	// When
	timer := NewTicker(100 * time.Millisecond)

	// Then
	end := <-timer.C()
	if end.Sub(start) < 100*time.Millisecond {
		assert.Fail(t, "Sleep did not last long enough")
	}
	end = <-timer.C()
	if end.Sub(start) < 200*time.Millisecond {
		assert.Fail(t, "Sleep did not last long enough")
	}

	timer.Stop()
	time.Sleep(150)
	select {
	case <-timer.C():
		assert.Fail(t, "Ticker should not have fired")
	default:
	}
}

func TestTick(t *testing.T) {
	start := Now()

	// When
	ch := Tick(100 * time.Millisecond)

	// Then
	end := <-ch
	if end.Sub(start) < 100*time.Millisecond {
		assert.Fail(t, "Sleep did not last long enough")
	}
	end = <-ch
	if end.Sub(start) < 200*time.Millisecond {
		assert.Fail(t, "Sleep did not last long enough")
	}
}

func TestNewStoppedTimer(t *testing.T) {
	timer := NewStoppedTimer()

	// When/Then
	select {
	case <-timer.C():
		assert.Fail(t, "Timer should not have fired")
	default:
	}
	assert.Equal(t, false, timer.Stop())
}

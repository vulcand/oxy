# Clock

A drop in (almost) replacement for the system `time` package. It provides a way
to make scheduled calls, timers and tickers deterministic in tests. By default
it forwards all calls to the system `time` package. In test, however, it is
possible to enable the frozen clock mode, and advance time manually to make
scheduled even trigger at certain moments.

# Usage

```go
package foo

import (
    "testing"

    "github.com/vulcand/oxy/internal/holsterv4/clock"
	"github.com/stretchr/testify/assert"
)

func TestSleep(t *testing.T) {
    // Freeze switches the clock package to the frozen clock mode. You need to
    // advance time manually from now on. Note that all scheduled events, timers
    // and ticker created before this call keep operating in real time.
    //
    // The initial time is set to now here, but you can set any datetime.
    clock.Freeze(clock.Now())
    // Do not forget to revert the effect of Freeze at the end of the test.
    defer clock.Unfreeze()

    var fired bool

    clock.AfterFunc(100*clock.Millisecond, func() {
        fired = true
    })
    clock.Advance(93*clock.Millisecond)

    // Advance will make all fire all events, timers, tickers that are
    // scheduled for the passed period of time. Note that scheduled functions
    // are called from within Advanced unlike system time package that calls
    // them in their own goroutine.
    assert.Equal(t, 97*clock.Millisecond, clock.Advance(6*clock.Millisecond))
    assert.True(t, fired)
    assert.Equal(t, 100*clock.Millisecond, clock.Advance(1*clock.Millisecond))
    assert.True(t, fired)
}
```

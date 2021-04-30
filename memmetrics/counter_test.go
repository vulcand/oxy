package memmetrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/mailgun/timetools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneExpired(t *testing.T) {
	clockTest := &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}

	cnt, err := NewCounter(3, time.Second, CounterClock(clockTest))
	require.NoError(t, err)

	cnt.Inc(1)

	clockTest.Sleep(time.Second)
	cnt.Inc(1)

	clockTest.Sleep(time.Second)
	cnt.Inc(1)

	clockTest.Sleep(time.Second)
	out := cnt.Clone()

	assert.EqualValues(t, 2, out.Count())
}

func BenchmarkCounterIncOnly(b *testing.B) {
	benchmarkCounter(
		b,
		func(clock *timetools.FreezedTime, ctr *RollingCounter) {
			clock.Sleep(time.Second)
			ctr.Inc(1)
		},
	)
}

func BenchmarkCounterIncCountContigu(b *testing.B) {
	benchmarkCounter(
		b,
		func(clock *timetools.FreezedTime, ctr *RollingCounter) {
			clock.Sleep(time.Second)
			ctr.Inc(1)
			ctr.Count()
		},
	)
}

func BenchmarkCounterIncCountSparse(b *testing.B) {
	benchmarkCounter(
		b,
		func(clock *timetools.FreezedTime, ctr *RollingCounter) {
			clock.Sleep(5 * time.Second)
			ctr.Inc(1)
			ctr.Count()
		},
	)
}

func benchmarkCounter(b *testing.B, fn func(*timetools.FreezedTime, *RollingCounter)) {
	clockTest := &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}

	for _, size := range []int{3, 5, 10, 20, 50, 100} {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			cnt, _ := NewCounter(size, time.Second, CounterClock(clockTest))

			for i := 0; i < b.N; i++ {
				fn(clockTest, cnt)
			}
		})
	}
}

package memmetrics

import (
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

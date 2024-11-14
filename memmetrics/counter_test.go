package memmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
)

func TestCloneExpired(t *testing.T) {
	clock.Freeze(clock.Date(2012, 3, 4, 5, 6, 7, 0, clock.UTC))

	cnt, err := NewCounter(3, clock.Second)
	require.NoError(t, err)

	cnt.Inc(1)

	clock.Advance(clock.Second)
	cnt.Inc(1)

	clock.Advance(clock.Second)
	cnt.Inc(1)

	clock.Advance(clock.Second)
	out := cnt.Clone()

	assert.EqualValues(t, 2, out.Count())
}

func TestCleanup(t *testing.T) {
	clock.Freeze(clock.Date(2012, 3, 4, 5, 6, 7, 0, clock.UTC))

	cnt, err := NewCounter(10, clock.Second)
	require.NoError(t, err)

	cnt.Inc(1)
	for i := 0; i < 9; i++ {
		clock.Advance(clock.Second)
		cnt.Inc(1)
	}
	// cnt will be  [1 1 1 1 1 1 1 1 1 1]

	clock.Advance(9 * clock.Second)
	assert.EqualValues(t, 1, cnt.Count())
	// cnt will be  [0 0 0 0 0 0 1 0 0 0]
	// old behavior [1 1 0 1 0 0 1 1 1 0]
}

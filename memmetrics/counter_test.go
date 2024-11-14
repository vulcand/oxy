package memmetrics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
	"github.com/vulcand/oxy/v2/testutils"
)

func TestRollingCounter_Clone_expired(t *testing.T) {
	testutils.FreezeTime(t)

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

func TestRollingCounter_cleanup(t *testing.T) {
	testutils.FreezeTime(t)

	cnt, err := NewCounter(10, clock.Second)
	require.NoError(t, err)

	cnt.Inc(1)

	for i := 0; i < 9; i++ {
		clock.Advance(clock.Second)
		cnt.Inc(int(math.Pow10(i + 1)))
	}

	assert.EqualValues(t, 1111111111, cnt.Count())
	assert.Equal(t, []int{1000, 10000, 100000, 1000000, 10000000, 100000000, 1000000000, 1, 10, 100}, cnt.values)

	clock.Advance(9 * clock.Second)

	assert.EqualValues(t, 1000000000, cnt.Count())
	assert.Equal(t, []int{0, 0, 0, 0, 0, 0, 1000000000, 0, 0, 0}, cnt.values)
}

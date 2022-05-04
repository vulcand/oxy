package memmetrics

import (
	"testing"

	"github.com/mailgun/holster/v4/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

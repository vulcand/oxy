package memmetrics

import (
	"testing"
	"time"

	"github.com/mailgun/holster/v3/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneExpired(t *testing.T) {
	defer clock.Freeze(time.Now()).Unfreeze()

	cnt, err := NewCounter(3, time.Second)
	require.NoError(t, err)

	cnt.Inc(1)

	clock.Advance(time.Second)
	cnt.Inc(1)

	clock.Advance(time.Second)
	cnt.Inc(1)

	clock.Advance(time.Second)
	out := cnt.Clone()

	assert.EqualValues(t, 2, out.Count())
}

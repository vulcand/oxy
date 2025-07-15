package cbreaker

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
	"github.com/vulcand/oxy/v2/testutils"
	"github.com/vulcand/oxy/v2/utils"
)

func Test_ratioController_rampUp(t *testing.T) {
	testutils.FreezeTime(t)

	duration := 10 * clock.Second
	rc := newRatioController(duration, &utils.NoopLogger{})

	allowed, denied := 0, 0
	for range duration / clock.Millisecond {
		ratio := sendRequest(&allowed, &denied, rc)
		expected := rc.targetRatio()
		diff := math.Abs(expected - ratio)
		t.Log("Ratio", ratio)
		t.Log("Expected", expected)
		t.Log("Diff", diff)
		assert.EqualValues(t, 0, round(diff, 0.5, 1)) //nolint:testifylint // the rounding is already handled.
		clock.Advance(clock.Millisecond)
	}
}

func sendRequest(allowed, denied *int, rc *ratioController) float64 {
	if rc.allowRequest() {
		*allowed++
	} else {
		*denied++
	}

	if *allowed+*denied == 0 {
		return 0
	}

	return float64(*allowed) / float64(*allowed+*denied)
}

func round(val float64, roundOn float64, places int) float64 {
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)

	var round float64
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}

	return round / pow
}

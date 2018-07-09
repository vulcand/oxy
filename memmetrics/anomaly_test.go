package memmetrics

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMedian(t *testing.T) {
	testCases := []struct {
		desc     string
		values   []float64
		expected float64
	}{
		{
			desc:     "2 values",
			values:   []float64{0.1, 0.2},
			expected: (float64(0.1) + float64(0.2)) / 2.0,
		},
		{
			desc:     "3 values",
			values:   []float64{0.3, 0.2, 0.5},
			expected: 0.3,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			actual := median(test.values)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestSplitRatios(t *testing.T) {
	testCases := []struct {
		values []float64
		good   []float64
		bad    []float64
	}{
		{
			values: []float64{0, 0},
			good:   []float64{0},
			bad:    []float64{},
		},

		{
			values: []float64{0, 1},
			good:   []float64{0},
			bad:    []float64{1},
		},
		{
			values: []float64{0.1, 0.1},
			good:   []float64{0.1},
			bad:    []float64{},
		},

		{
			values: []float64{0.15, 0.1},
			good:   []float64{0.15, 0.1},
			bad:    []float64{},
		},
		{
			values: []float64{0.01, 0.01},
			good:   []float64{0.01},
			bad:    []float64{},
		},
		{
			values: []float64{0.012, 0.01, 1},
			good:   []float64{0.012, 0.01},
			bad:    []float64{1},
		},
		{
			values: []float64{0, 0, 1, 1},
			good:   []float64{0},
			bad:    []float64{1},
		},
		{
			values: []float64{0, 0.1, 0.1, 0},
			good:   []float64{0},
			bad:    []float64{0.1},
		},
		{
			values: []float64{0, 0.01, 0.1, 0},
			good:   []float64{0},
			bad:    []float64{0.01, 0.1},
		},
		{
			values: []float64{0, 0.01, 0.02, 1},
			good:   []float64{0, 0.01, 0.02},
			bad:    []float64{1},
		},
		{
			values: []float64{0, 0, 0, 0, 0, 0.01, 0.02, 1},
			good:   []float64{0},
			bad:    []float64{0.01, 0.02, 1},
		},
	}

	for ind, test := range testCases {
		test := test
		t.Run(strconv.Itoa(ind), func(t *testing.T) {
			t.Parallel()

			good, bad := SplitRatios(test.values)

			vgood := make(map[float64]bool, len(test.good))
			for _, v := range test.good {
				vgood[v] = true
			}

			vbad := make(map[float64]bool, len(test.bad))
			for _, v := range test.bad {
				vbad[v] = true
			}

			assert.Equal(t, vgood, good)
			assert.Equal(t, vbad, bad)
		})
	}
}

func TestSplitLatencies(t *testing.T) {
	testCases := []struct {
		values []int
		good   []int
		bad    []int
	}{
		{
			values: []int{0, 0},
			good:   []int{0},
			bad:    []int{},
		},
		{
			values: []int{1, 2},
			good:   []int{1, 2},
			bad:    []int{},
		},
		{
			values: []int{1, 2, 4},
			good:   []int{1, 2, 4},
			bad:    []int{},
		},
		{
			values: []int{8, 8, 18},
			good:   []int{8},
			bad:    []int{18},
		},
		{
			values: []int{32, 28, 11, 26, 19, 51, 25, 39, 28, 26, 8, 97},
			good:   []int{32, 28, 11, 26, 19, 51, 25, 39, 28, 26, 8},
			bad:    []int{97},
		},
		{
			values: []int{1, 2, 4, 40},
			good:   []int{1, 2, 4},
			bad:    []int{40},
		},
		{
			values: []int{40, 60, 1000},
			good:   []int{40, 60},
			bad:    []int{1000},
		},
	}

	for ind, test := range testCases {
		test := test
		t.Run(strconv.Itoa(ind), func(t *testing.T) {
			t.Parallel()

			values := make([]time.Duration, len(test.values))
			for i, d := range test.values {
				values[i] = time.Millisecond * time.Duration(d)
			}

			good, bad := SplitLatencies(values, time.Millisecond)

			vgood := make(map[time.Duration]bool, len(test.good))
			for _, v := range test.good {
				vgood[time.Duration(v)*time.Millisecond] = true
			}
			assert.Equal(t, vgood, good)

			vbad := make(map[time.Duration]bool, len(test.bad))
			for _, v := range test.bad {
				vbad[time.Duration(v)*time.Millisecond] = true
			}
			assert.Equal(t, vbad, bad)
		})
	}
}

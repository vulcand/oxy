package memmetrics

import (
	"fmt"
	"time"

	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
)

type rcOption func(*RollingCounter) error

// RollingCounter Calculates in memory failure rate of an endpoint using rolling window of a predefined size.
type RollingCounter struct {
	resolution     time.Duration
	values         []int
	countedBuckets int // how many samples in different buckets have we collected so far
	lastBucket     int // last recorded bucket
	lastUpdated    clock.Time
}

// NewCounter creates a counter with fixed amount of buckets that are rotated every resolution period.
// E.g. 10 buckets with 1 second means that every new second the bucket is refreshed, so it maintains 10 seconds rolling window.
// By default, creates a bucket with 10 buckets and 1 second resolution.
func NewCounter(buckets int, resolution time.Duration, options ...rcOption) (*RollingCounter, error) {
	if buckets <= 0 {
		return nil, fmt.Errorf("buckets should be >= 0")
	}
	if resolution < clock.Second {
		return nil, fmt.Errorf("resolution should be larger than a second")
	}

	rc := &RollingCounter{
		lastBucket: -1,
		resolution: resolution,

		values: make([]int, buckets),
	}

	for _, o := range options {
		if err := o(rc); err != nil {
			return nil, err
		}
	}

	return rc, nil
}

// Append appends a counter.
func (c *RollingCounter) Append(o *RollingCounter) error {
	c.Inc(int(o.Count()))
	return nil
}

// Clone clones a counter.
func (c *RollingCounter) Clone() *RollingCounter {
	c.cleanup()
	other := &RollingCounter{
		resolution:  c.resolution,
		values:      make([]int, len(c.values)),
		lastBucket:  c.lastBucket,
		lastUpdated: c.lastUpdated,
	}
	copy(other.values, c.values)
	return other
}

// Reset resets a counter.
func (c *RollingCounter) Reset() {
	c.lastBucket = -1
	c.countedBuckets = 0
	c.lastUpdated = clock.Time{}
	for i := range c.values {
		c.values[i] = 0
	}
}

// CountedBuckets gets counted buckets.
func (c *RollingCounter) CountedBuckets() int {
	return c.countedBuckets
}

// Count counts.
func (c *RollingCounter) Count() int64 {
	c.cleanup()
	return c.sum()
}

// Resolution gets resolution.
func (c *RollingCounter) Resolution() time.Duration {
	return c.resolution
}

// Buckets gets buckets.
func (c *RollingCounter) Buckets() int {
	return len(c.values)
}

// WindowSize gets windows size.
func (c *RollingCounter) WindowSize() time.Duration {
	return time.Duration(len(c.values)) * c.resolution
}

// Inc increments counter.
func (c *RollingCounter) Inc(v int) {
	c.cleanup()
	c.incBucketValue(v)
}

func (c *RollingCounter) incBucketValue(v int) {
	now := clock.Now().UTC()
	bucket := c.getBucket(now)
	c.values[bucket] += v
	c.lastUpdated = now
	// Update usage stats if we haven't collected enough data
	if c.countedBuckets < len(c.values) {
		// Only update if we have advanced to the next bucket and not incremented the value
		// in the current bucket.
		if c.lastBucket != bucket {
			c.lastBucket = bucket
			c.countedBuckets++
		}
	}
}

// Returns the number in the moving window bucket that this slot occupies.
func (c *RollingCounter) getBucket(t time.Time) int {
	return int(t.Truncate(c.resolution).Unix() % int64(len(c.values)))
}

// Reset buckets that were not updated.
func (c *RollingCounter) cleanup() {
	now := clock.Now().UTC()
	for i := 0; i < len(c.values); i++ {
		now = now.Add(time.Duration(-1*i) * c.resolution)
		if now.Truncate(c.resolution).After(c.lastUpdated.Truncate(c.resolution)) {
			c.values[c.getBucket(now)] = 0
		} else {
			break
		}
	}
}

func (c *RollingCounter) sum() int64 {
	out := int64(0)
	for _, v := range c.values {
		out += int64(v)
	}
	return out
}

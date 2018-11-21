package memmetrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/mailgun/timetools"
)

type rcOptSetter func(*RollingCounter) error

// CounterClock defines a counter clock
func CounterClock(c timetools.TimeProvider) rcOptSetter {
	return func(r *RollingCounter) error {
		r.clock = c
		return nil
	}
}

type bucket struct {
	value  int64
	bucket int64
}

// RollingCounter Calculates in memory failure rate of an endpoint using rolling window of a predefined size
type RollingCounter struct {
	clock      timetools.TimeProvider
	resolution int64

	values     []bucket
	pos        int
	mu         sync.RWMutex
	incCounter int
}

// NewCounter creates a counter with fixed amount of buckets that are rotated every resolution period.
// E.g. 10 buckets with 1 second means that every new second the bucket is refreshed, so it maintains 10 second rolling window.
// By default creates a bucket with 10 buckets and 1 second resolution
func NewCounter(buckets int, resolution time.Duration, options ...rcOptSetter) (*RollingCounter, error) {
	if buckets <= 0 {
		return nil, fmt.Errorf("Buckets should be >= 0")
	}
	if resolution < time.Second {
		return nil, fmt.Errorf("Resolution should be larger than a second")
	}

	rc := &RollingCounter{
		values:     make([]bucket, buckets),
		pos:        buckets - 1,
		resolution: int64(resolution),
	}

	for _, o := range options {
		if err := o(rc); err != nil {
			return nil, err
		}
	}

	if rc.clock == nil {
		rc.clock = &timetools.RealTime{}
	}

	return rc, nil
}

// Append append a counter
func (c *RollingCounter) Append(o *RollingCounter) error {
	c.Inc(int(o.Count()))
	return nil
}

// Clone clone a counter
func (c *RollingCounter) Clone() *RollingCounter {
	other := &RollingCounter{
		resolution: c.resolution,
		values:     make([]bucket, len(c.values)),
		clock:      c.clock,
		pos:        c.pos,
		incCounter: c.incCounter,
	}
	copy(other.values, c.values)
	return other
}

// Reset reset a counter
func (c *RollingCounter) Reset() {
	c.pos = len(c.values) - 1
	c.incCounter = 0
	for i := range c.values {
		c.values[i].value = 0
	}
}

// CountedBuckets gets counted buckets
func (c *RollingCounter) CountedBuckets() int {
	if c.incCounter < len(c.values) {
		return c.incCounter
	}

	return len(c.values)
}

// Count counts
func (c *RollingCounter) Count() int64 {
	return c.sum()
}

// Resolution gets resolution
func (c *RollingCounter) Resolution() time.Duration {
	return time.Duration(c.resolution)
}

// Buckets gets buckets
func (c *RollingCounter) Buckets() int {
	return len(c.values)
}

// WindowSize gets windows size
func (c *RollingCounter) WindowSize() time.Duration {
	return time.Duration(int64(len(c.values)) * c.resolution)
}

// Inc increment counter
func (c *RollingCounter) Inc(v int) {
	c.incBucketValue(v)
}

func (c *RollingCounter) incBucketValue(v int) {
	now := c.clock.UtcNow()
	bucket := c.getBucket(now)

	c.mu.Lock()

	if c.values[c.pos].bucket != bucket {
		c.pos = (c.pos + 1) % len(c.values)
	}

	c.values[c.pos].bucket = bucket
	c.values[c.pos].value += int64(v)
	c.incCounter++

	c.mu.Unlock()
}

// Returns the number in the moving window bucket that this slot occupies
func (c *RollingCounter) getBucket(t time.Time) int64 {
	return t.UnixNano() / int64(c.resolution)
}

func (c *RollingCounter) sum() int64 {
	var (
		out int64

		vs        = len(c.values)
		minBucket = c.getBucket(c.clock.UtcNow()) - int64(vs) + 1
	)

	c.mu.RLock()

	for i := 0; i < len(c.values); i++ {
		val := c.values[(vs+c.pos-i)%vs]

		if val.bucket < minBucket {
			break
		}

		out += val.value
	}

	c.mu.RUnlock()

	return out
}

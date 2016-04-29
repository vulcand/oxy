package memmetrics

import (
	"time"

	"github.com/codahale/hdrhistogram"
	"github.com/mailgun/timetools"
	. "gopkg.in/check.v1"
)

type HistogramSuite struct {
	tm *timetools.FreezedTime
}

var _ = Suite(&HistogramSuite{})

func (s *HistogramSuite) SetUpSuite(c *C) {
	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func (s *HistogramSuite) TestMerge(c *C) {
	a, err := NewHDRHistogram(1, 3600000, 2)
	c.Assert(err, IsNil)

	a.RecordValues(1, 2)

	b, err := NewHDRHistogram(1, 3600000, 2)
	c.Assert(err, IsNil)

	b.RecordValues(2, 1)

	c.Assert(a.Merge(b), IsNil)

	c.Assert(a.ValueAtQuantile(50), Equals, int64(1))
	c.Assert(a.ValueAtQuantile(100), Equals, int64(2))
}

func (s *HistogramSuite) TestInvalidParams(c *C) {
	_, err := NewHDRHistogram(1, 3600000, 0)
	c.Assert(err, NotNil)
}

func (s *HistogramSuite) TestMergeNil(c *C) {
	a, err := NewHDRHistogram(1, 3600000, 1)
	c.Assert(err, IsNil)

	c.Assert(a.Merge(nil), NotNil)
}

func (s *HistogramSuite) TestRotation(c *C) {
	h, err := NewRollingHDRHistogram(
		1,           // min value
		3600000,     // max value
		3,           // significant figurwes
		time.Second, // 1 second is a rolling period
		2,           // 2 histograms in a window
		RollingClock(s.tm))

	c.Assert(err, IsNil)
	c.Assert(h, NotNil)

	h.RecordValues(5, 1)

	m, err := h.Merged()
	c.Assert(err, IsNil)
	c.Assert(m.ValueAtQuantile(100), Equals, int64(5))

	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	h.RecordValues(2, 1)
	h.RecordValues(1, 1)

	m, err = h.Merged()
	c.Assert(err, IsNil)
	c.Assert(m.ValueAtQuantile(100), Equals, int64(5))

	// rotate, this means that the old value would evaporate
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	h.RecordValues(1, 1)
	m, err = h.Merged()
	c.Assert(err, IsNil)
	c.Assert(m.ValueAtQuantile(100), Equals, int64(2))
}

func (s *HistogramSuite) TestReset(c *C) {
	h, err := NewRollingHDRHistogram(
		1,           // min value
		3600000,     // max value
		3,           // significant figurwes
		time.Second, // 1 second is a rolling period
		2,           // 2 histograms in a window
		RollingClock(s.tm))

	c.Assert(err, IsNil)
	c.Assert(h, NotNil)

	h.RecordValues(5, 1)

	m, err := h.Merged()
	c.Assert(err, IsNil)
	c.Assert(m.ValueAtQuantile(100), Equals, int64(5))

	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	h.RecordValues(2, 1)
	h.RecordValues(1, 1)

	m, err = h.Merged()
	c.Assert(err, IsNil)
	c.Assert(m.ValueAtQuantile(100), Equals, int64(5))

	h.Reset()

	h.RecordValues(5, 1)

	m, err = h.Merged()
	c.Assert(err, IsNil)
	c.Assert(m.ValueAtQuantile(100), Equals, int64(5))

	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	h.RecordValues(2, 1)
	h.RecordValues(1, 1)

	m, err = h.Merged()
	c.Assert(err, IsNil)
	c.Assert(m.ValueAtQuantile(100), Equals, int64(5))

}

func (s *HistogramSuite) TestHDRHistogramExportReturnsNewCopy(c *C) {
	// Create HDRHistogram instance
	a := HDRHistogram{}
	a.low = 1
	a.high = 2
	a.sigfigs = 3
	a.h = hdrhistogram.New(0, 1, 2)

	// Get a copy and modify the original
	b := a.Export()
	a.low = 11
	a.high = 12
	a.sigfigs = 4
	a.h = nil

	// Assert the copy has not been modified
	c.Assert(b.low, Equals, int64(1))
	c.Assert(b.high, Equals, int64(2))
	c.Assert(b.sigfigs, Equals, 3)
	c.Assert(b.h, NotNil)
}

func (s *HistogramSuite) TestRollingHDRHistogramExportReturnsNewCopy(c *C) {
	a := RollingHDRHistogram{}
	a.idx = 1
	origTime := time.Now()
	a.lastRoll = origTime
	a.period = 2 * time.Second
	a.bucketCount = 3
	a.low = 4
	a.high = 5
	a.sigfigs = 1
	a.buckets = []*HDRHistogram{}
	a.clock = s.tm

	b := a.Export()
	a.idx = 11
	a.lastRoll = time.Now().Add(1 * time.Minute)
	a.period = 12 * time.Second
	a.bucketCount = 13
	a.low = 14
	a.high = 15
	a.sigfigs = 1
	a.buckets = nil
	a.clock = nil

	c.Assert(b.idx, Equals, 1)
	c.Assert(b.lastRoll, Equals, origTime)
	c.Assert(b.period, Equals, 2*time.Second)
	c.Assert(b.bucketCount, Equals, 3)
	c.Assert(b.low, Equals, int64(4))
	c.Assert(b.high, Equals, int64(5))
	c.Assert(b.buckets, NotNil)
	c.Assert(b.clock, NotNil)
}

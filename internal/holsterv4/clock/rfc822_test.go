package clock

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	Time RFC822Time `json:"ts"`
}

func TestRFC822New(t *testing.T) {
	stdTime, err := Parse(RFC3339, "2019-08-29T11:20:07.123456+03:00")
	assert.NoError(t, err)

	rfc822TimeFromTime := NewRFC822Time(stdTime)
	rfc822TimeFromUnix := NewRFC822TimeFromUnix(stdTime.Unix())
	assert.True(t, rfc822TimeFromTime.Equal(rfc822TimeFromUnix.Time),
		"want=%s, got=%s", rfc822TimeFromTime.Time, rfc822TimeFromUnix.Time)

	// Parsing from numerical offset to abbreviated offset is not always reliable. In this
	// context Go will fallback to the known numerical offset.
	assert.Equal(t, "Thu, 29 Aug 2019 11:20:07 +0300", rfc822TimeFromTime.String())
	assert.Equal(t, "Thu, 29 Aug 2019 08:20:07 UTC", rfc822TimeFromUnix.String())
}

// NewRFC822Time truncates to second precision.
func TestRFC822SecondPrecision(t *testing.T) {
	stdTime1, err := Parse(RFC3339, "2019-08-29T11:20:07.111111+03:00")
	assert.NoError(t, err)
	stdTime2, err := Parse(RFC3339, "2019-08-29T11:20:07.999999+03:00")
	assert.NoError(t, err)
	assert.False(t, stdTime1.Equal(stdTime2))

	rfc822Time1 := NewRFC822Time(stdTime1)
	rfc822Time2 := NewRFC822Time(stdTime2)
	assert.True(t, rfc822Time1.Equal(rfc822Time2.Time),
		"want=%s, got=%s", rfc822Time1.Time, rfc822Time2.Time)
}

// Marshaled representation is truncated down to second precision.
func TestRFC822Marshaling(t *testing.T) {
	stdTime, err := Parse(RFC3339Nano, "2019-08-29T11:20:07.123456789+03:30")
	assert.NoError(t, err)

	ts := testStruct{Time: NewRFC822Time(stdTime)}
	encoded, err := json.Marshal(&ts)
	assert.NoError(t, err)
	assert.Equal(t, `{"ts":"Thu, 29 Aug 2019 11:20:07 +0330"}`, string(encoded))
}

func TestRFC822Unmarshaling(t *testing.T) {
	for i, tc := range []struct {
		inRFC822   string
		outRFC3339 string
		outRFC822  string
	}{{
		inRFC822:   "Thu, 29 Aug 2019 11:20:07 GMT",
		outRFC3339: "2019-08-29T11:20:07Z",
		outRFC822:  "Thu, 29 Aug 2019 11:20:07 GMT",
	}, {
		inRFC822: "Thu, 29 Aug 2019 11:20:07 MSK",
		// Extrapolating the numerical offset from an abbreviated offset is unreliable. In
		// this test case the RFC3339 will have the incorrect result due to limitation in
		// Go's time parser.
		outRFC3339: "2019-08-29T11:20:07Z",
		outRFC822:  "Thu, 29 Aug 2019 11:20:07 MSK",
	}, {
		inRFC822:   "Thu, 29 Aug 2019 11:20:07 -0000",
		outRFC3339: "2019-08-29T11:20:07Z",
		outRFC822:  "Thu, 29 Aug 2019 11:20:07 -0000",
	}, {
		inRFC822:   "Thu, 29 Aug 2019 11:20:07 +0000",
		outRFC3339: "2019-08-29T11:20:07Z",
		outRFC822:  "Thu, 29 Aug 2019 11:20:07 +0000",
	}, {
		inRFC822:   "Thu, 29 Aug 2019 11:20:07 +0300",
		outRFC3339: "2019-08-29T11:20:07+03:00",
		outRFC822:  "Thu, 29 Aug 2019 11:20:07 +0300",
	}, {
		inRFC822:   "Thu, 29 Aug 2019 11:20:07 +0330",
		outRFC3339: "2019-08-29T11:20:07+03:30",
		outRFC822:  "Thu, 29 Aug 2019 11:20:07 +0330",
	}, {
		inRFC822:   "Sun, 01 Sep 2019 11:20:07 +0300",
		outRFC3339: "2019-09-01T11:20:07+03:00",
		outRFC822:  "Sun, 01 Sep 2019 11:20:07 +0300",
	}, {
		inRFC822:   "Sun,  1 Sep 2019 11:20:07 +0300",
		outRFC3339: "2019-09-01T11:20:07+03:00",
		outRFC822:  "Sun, 01 Sep 2019 11:20:07 +0300",
	}, {
		inRFC822:   "Sun, 1 Sep 2019 11:20:07 +0300",
		outRFC3339: "2019-09-01T11:20:07+03:00",
		outRFC822:  "Sun, 01 Sep 2019 11:20:07 +0300",
	}, {
		inRFC822:   "Sun, 1 Sep 2019 11:20:07 UTC",
		outRFC3339: "2019-09-01T11:20:07Z",
		outRFC822:  "Sun, 01 Sep 2019 11:20:07 UTC",
	}, {
		inRFC822:   "Sun, 1 Sep 2019 11:20:07 UTC",
		outRFC3339: "2019-09-01T11:20:07Z",
		outRFC822:  "Sun, 01 Sep 2019 11:20:07 UTC",
	}, {
		inRFC822:   "Sun, 1 Sep 2019 11:20:07 GMT",
		outRFC3339: "2019-09-01T11:20:07Z",
		outRFC822:  "Sun, 01 Sep 2019 11:20:07 GMT",
	}, {
		inRFC822:   "Fri, 21 Nov 1997 09:55:06 -0600 (MDT)",
		outRFC3339: "1997-11-21T09:55:06-06:00",
		outRFC822:  "Fri, 21 Nov 1997 09:55:06 MDT",
	}} {
		t.Run(tc.inRFC822, func(t *testing.T) {
			tcDesc := fmt.Sprintf("Test case #%d: %v", i, tc)
			var ts testStruct

			inEncoded := []byte(fmt.Sprintf(`{"ts":"%s"}`, tc.inRFC822))
			err := json.Unmarshal(inEncoded, &ts)
			assert.NoError(t, err, tcDesc)
			assert.Equal(t, tc.outRFC3339, ts.Time.Format(RFC3339), tcDesc)

			actualEncoded, err := json.Marshal(&ts)
			assert.NoError(t, err, tcDesc)
			outEncoded := fmt.Sprintf(`{"ts":"%s"}`, tc.outRFC822)
			assert.Equal(t, outEncoded, string(actualEncoded), tcDesc)
		})
	}
}

func TestRFC822UnmarshalingError(t *testing.T) {
	for _, tc := range []struct {
		inEncoded string
		outError  string
	}{{
		inEncoded: `{"ts": "Thu, 29 Aug 2019 11:20:07"}`,
		outError:  `parsing time "Thu, 29 Aug 2019 11:20:07" as "January 2 2006 15:04 -0700 (MST)": cannot parse "Thu, 29 Aug 2019 11:20:07" as "January"`,
	}, {
		inEncoded: `{"ts": "foo"}`,
		outError:  `parsing time "foo" as "January 2 2006 15:04 -0700 (MST)": cannot parse "foo" as "January"`,
	}, {
		inEncoded: `{"ts": 42}`,
		outError:  "invalid syntax",
	}} {
		t.Run(tc.inEncoded, func(t *testing.T) {
			var ts testStruct
			err := json.Unmarshal([]byte(tc.inEncoded), &ts)
			assert.EqualError(t, err, tc.outError)
		})
	}
}

func TestParseRFC822Time(t *testing.T) {
	for _, tt := range []struct {
		rfc822Time string
	}{
		{"Thu, 3 Jun 2021 12:01:05 MST"},
		{"Thu, 3 Jun 2021 12:01:05 -0700"},
		{"Thu, 3 Jun 2021 12:01:05 -0700 (MST)"},
		{"2 Jun 2021 17:06:41 GMT"},
		{"2 Jun 2021 17:06:41 -0700"},
		{"2 Jun 2021 17:06:41 -0700 (MST)"},
		{"Mon, 30 August 2021 11:05:00 -0400"},
		{"Thu, 3 June 2021 12:01:05 MST"},
		{"Thu, 3 June 2021 12:01:05 -0700"},
		{"Thu, 3 June 2021 12:01:05 -0700 (MST)"},
		{"2 June 2021 17:06:41 GMT"},
		{"2 June 2021 17:06:41 -0700"},
		{"2 June 2021 17:06:41 -0700 (MST)"},
		{"Wed, Nov 03 2021 17:48:06 CST"},
		{"Wed, November 03 2021 17:48:06 CST"},

		// Timestamps without seconds.
		{"Sun, 31 Oct 2021 12:10 -5000"},
		{"Thu, 3 Jun 2021 12:01 MST"},
		{"Thu, 3 Jun 2021 12:01 -0700"},
		{"Thu, 3 Jun 2021 12:01 -0700 (MST)"},
		{"2 Jun 2021 17:06 GMT"},
		{"2 Jun 2021 17:06 -0700"},
		{"2 Jun 2021 17:06 -0700 (MST)"},
		{"Mon, 30 August 2021 11:05 -0400"},
		{"Thu, 3 June 2021 12:01 MST"},
		{"Thu, 3 June 2021 12:01 -0700"},
		{"Thu, 3 June 2021 12:01 -0700 (MST)"},
		{"2 June 2021 17:06 GMT"},
		{"2 June 2021 17:06 -0700"},
		{"2 June 2021 17:06 -0700 (MST)"},
		{"Wed, Nov 03 2021 17:48 CST"},
		{"Wed, November 03 2021 17:48 CST"},
	} {
		t.Run(tt.rfc822Time, func(t *testing.T) {
			_, err := ParseRFC822Time(tt.rfc822Time)
			assert.NoError(t, err)
		})
	}
}

func TestStringWithOffset(t *testing.T) {
	now := time.Now().UTC()
	r := NewRFC822Time(now)
	assert.Equal(t, now.Format(time.RFC1123Z), r.StringWithOffset())
}

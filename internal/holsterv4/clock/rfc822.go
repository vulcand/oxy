package clock

import (
	"strconv"
	"time"
)

var datetimeLayouts = [48]string{
	// Day first month 2nd abbreviated.
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 -0700 (MST)",
	"2 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 -0700 (MST)",
	"Mon, 2 Jan 2006 15:04 MST",
	"Mon, 2 Jan 2006 15:04 -0700",
	"Mon, 2 Jan 2006 15:04 -0700 (MST)",
	"2 Jan 2006 15:04 MST",
	"2 Jan 2006 15:04 -0700",
	"2 Jan 2006 15:04 -0700 (MST)",

	// Month first day 2nd abbreviated.
	"Mon, Jan 2 2006 15:04:05 MST",
	"Mon, Jan 2 2006 15:04:05 -0700",
	"Mon, Jan 2 2006 15:04:05 -0700 (MST)",
	"Jan 2 2006 15:04:05 MST",
	"Jan 2 2006 15:04:05 -0700",
	"Jan 2 2006 15:04:05 -0700 (MST)",
	"Mon, Jan 2 2006 15:04 MST",
	"Mon, Jan 2 2006 15:04 -0700",
	"Mon, Jan 2 2006 15:04 -0700 (MST)",
	"Jan 2 2006 15:04 MST",
	"Jan 2 2006 15:04 -0700",
	"Jan 2 2006 15:04 -0700 (MST)",

	// Day first month 2nd not abbreviated.
	"Mon, 2 January 2006 15:04:05 MST",
	"Mon, 2 January 2006 15:04:05 -0700",
	"Mon, 2 January 2006 15:04:05 -0700 (MST)",
	"2 January 2006 15:04:05 MST",
	"2 January 2006 15:04:05 -0700",
	"2 January 2006 15:04:05 -0700 (MST)",
	"Mon, 2 January 2006 15:04 MST",
	"Mon, 2 January 2006 15:04 -0700",
	"Mon, 2 January 2006 15:04 -0700 (MST)",
	"2 January 2006 15:04 MST",
	"2 January 2006 15:04 -0700",
	"2 January 2006 15:04 -0700 (MST)",

	// Month first day 2nd not abbreviated.
	"Mon, January 2 2006 15:04:05 MST",
	"Mon, January 2 2006 15:04:05 -0700",
	"Mon, January 2 2006 15:04:05 -0700 (MST)",
	"January 2 2006 15:04:05 MST",
	"January 2 2006 15:04:05 -0700",
	"January 2 2006 15:04:05 -0700 (MST)",
	"Mon, January 2 2006 15:04 MST",
	"Mon, January 2 2006 15:04 -0700",
	"Mon, January 2 2006 15:04 -0700 (MST)",
	"January 2 2006 15:04 MST",
	"January 2 2006 15:04 -0700",
	"January 2 2006 15:04 -0700 (MST)",
}

// Allows seamless JSON encoding/decoding of rfc822 formatted timestamps.
// https://www.ietf.org/rfc/rfc822.txt section 5.
type RFC822Time struct {
	Time
}

// NewRFC822Time creates RFC822Time from a standard Time. The created value is
// truncated down to second precision because RFC822 does not allow for better.
func NewRFC822Time(t Time) RFC822Time {
	return RFC822Time{Time: t.Truncate(Second)}
}

// ParseRFC822Time parses an RFC822 time string.
func ParseRFC822Time(s string) (Time, error) {
	var t time.Time
	var err error
	for _, layout := range datetimeLayouts {
		t, err = Parse(layout, s)
		if err == nil {
			return t, err
		}
	}
	return t, err
}

// NewRFC822Time creates RFC822Time from a Unix timestamp (seconds from Epoch).
func NewRFC822TimeFromUnix(timestamp int64) RFC822Time {
	return RFC822Time{Time: Unix(timestamp, 0).UTC()}
}

func (t RFC822Time) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(t.Format(RFC1123))), nil
}

func (t *RFC822Time) UnmarshalJSON(s []byte) error {
	q, err := strconv.Unquote(string(s))
	if err != nil {
		return err
	}
	parsed, err := ParseRFC822Time(q)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

func (t RFC822Time) String() string {
	return t.Format(RFC1123)
}

func (t RFC822Time) StringWithOffset() string {
	return t.Format(RFC1123Z)
}

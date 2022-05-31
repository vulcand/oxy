// +build go1.9

// This file introduces aliases to allow using of the clock package as a
// drop-in replacement of the standard time package.

package clock

import "time"

type (
	Time     = time.Time
	Duration = time.Duration
	Location = time.Location

	Weekday = time.Weekday
	Month   = time.Month

	ParseError = time.ParseError
)

const (
	Nanosecond  = time.Nanosecond
	Microsecond = time.Microsecond
	Millisecond = time.Millisecond
	Second      = time.Second
	Minute      = time.Minute
	Hour        = time.Hour

	Sunday    = time.Sunday
	Monday    = time.Monday
	Tuesday   = time.Tuesday
	Wednesday = time.Wednesday
	Thursday  = time.Thursday
	Friday    = time.Friday
	Saturday  = time.Saturday

	January   = time.January
	February  = time.February
	March     = time.March
	April     = time.April
	May       = time.May
	June      = time.June
	July      = time.July
	August    = time.August
	September = time.September
	October   = time.October
	November  = time.November
	December  = time.December

	ANSIC       = time.ANSIC
	UnixDate    = time.UnixDate
	RubyDate    = time.RubyDate
	RFC822      = time.RFC822
	RFC822Z     = time.RFC822Z
	RFC850      = time.RFC850
	RFC1123     = time.RFC1123
	RFC1123Z    = time.RFC1123Z
	RFC3339     = time.RFC3339
	RFC3339Nano = time.RFC3339Nano
	Kitchen     = time.Kitchen
	Stamp       = time.Stamp
	StampMilli  = time.StampMilli
	StampMicro  = time.StampMicro
	StampNano   = time.StampNano
)

var (
	UTC   = time.UTC
	Local = time.Local
)

func Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location) Time {
	return time.Date(year, month, day, hour, min, sec, nsec, loc)
}

func FixedZone(name string, offset int) *Location {
	return time.FixedZone(name, offset)
}

func LoadLocation(name string) (*Location, error) {
	return time.LoadLocation(name)
}

func Parse(layout, value string) (Time, error) {
	return time.Parse(layout, value)
}

func ParseDuration(s string) (Duration, error) {
	return time.ParseDuration(s)
}

func ParseInLocation(layout, value string, loc *Location) (Time, error) {
	return time.ParseInLocation(layout, value, loc)
}

func Unix(sec int64, nsec int64) Time {
	return time.Unix(sec, nsec)
}

func Since(t Time) Duration {
	return provider.Now().Sub(t)
}

func Until(t Time) Duration {
	return t.Sub(provider.Now())
}

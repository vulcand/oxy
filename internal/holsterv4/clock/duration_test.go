package clock_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
)

type DurationSuite struct {
	suite.Suite
}

func TestDurationSuite(t *testing.T) {
	suite.Run(t, new(DurationSuite))
}

func (s *DurationSuite) TestNewOk() {
	for _, v := range []interface{}{
		42 * clock.Second,
		int(42000000000),
		int64(42000000000),
		42000000000.,
		"42s",
		[]byte("42s"),
	} {
		d, err := clock.NewDurationJSON(v)
		s.Nil(err)
		s.Equal(42*clock.Second, d.Duration)
	}
}

func (s *DurationSuite) TestNewError() {
	for _, tc := range []struct {
		v      interface{}
		errMsg string
	}{{
		v:      "foo",
		errMsg: "while parsing string: time: invalid duration \"foo\"",
	}, {
		v:      []byte("foo"),
		errMsg: "while parsing []byte: time: invalid duration \"foo\"",
	}, {
		v:      true,
		errMsg: "bad type bool",
	}} {
		d, err := clock.NewDurationJSON(tc.v)
		s.Equal(tc.errMsg, err.Error())
		s.Equal(clock.DurationJSON{}, d)
	}
}

func (s *DurationSuite) TestUnmarshal() {
	for _, v := range []string{
		`{"foo": 42000000000}`,
		`{"foo": 0.42e11}`,
		`{"foo": "42s"}`,
	} {
		var withDuration struct {
			Foo clock.DurationJSON `json:"foo"`
		}
		err := json.Unmarshal([]byte(v), &withDuration)
		s.Nil(err)
		s.Equal(42*clock.Second, withDuration.Foo.Duration)
	}
}

func (s *DurationSuite) TestMarshalling() {
	d, err := clock.NewDurationJSON(42 * clock.Second)
	s.Nil(err)
	encoded, err := d.MarshalJSON()
	s.Nil(err)
	var decoded clock.DurationJSON
	err = decoded.UnmarshalJSON(encoded)
	s.Nil(err)
	s.Equal(d, decoded)
	s.Equal("42s", decoded.String())
}

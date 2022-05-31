package clock

import (
	"encoding/json"
	"fmt"
)

type DurationJSON struct {
	Duration Duration
}

func NewDurationJSON(v interface{}) (DurationJSON, error) {
	switch v := v.(type) {
	case Duration:
		return DurationJSON{Duration: v}, nil
	case float64:
		return DurationJSON{Duration: Duration(v)}, nil
	case int64:
		return DurationJSON{Duration: Duration(v)}, nil
	case int:
		return DurationJSON{Duration: Duration(v)}, nil
	case []byte:
		duration, err := ParseDuration(string(v))
		if err != nil {
			return DurationJSON{}, fmt.Errorf("while parsing []byte: %w", err)
		}
		return DurationJSON{Duration: duration}, nil
	case string:
		duration, err := ParseDuration(v)
		if err != nil {
			return DurationJSON{}, fmt.Errorf("while parsing string: %w", err)
		}
		return DurationJSON{Duration: duration}, nil
	default:
		return DurationJSON{}, fmt.Errorf("bad type %T", v)
	}
}

func NewDurationJSONOrPanic(v interface{}) DurationJSON {
	d, err := NewDurationJSON(v)
	if err != nil {
		panic(err)
	}
	return d
}

func (d DurationJSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

func (d *DurationJSON) UnmarshalJSON(b []byte) error {
	var v interface{}
	var err error

	if err = json.Unmarshal(b, &v); err != nil {
		return err
	}

	*d, err = NewDurationJSON(v)
	return err
}

func (d DurationJSON) String() string {
	return d.Duration.String()
}

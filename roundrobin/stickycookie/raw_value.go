package stickycookie

import (
	"net/url"
)

// RawValue is a no-op that returns the raw strings as-is.
type RawValue struct{}

// Get returns the raw value.
func (v *RawValue) Get(raw *url.URL) string {
	return raw.String()
}

// FindURL gets url from array that match the value.
func (v *RawValue) FindURL(raw string, urls []*url.URL) (*url.URL, error) {
	for _, u := range urls {
		ok, err := areURLEqual(raw, u)
		if err != nil {
			return nil, err
		}

		if ok {
			return u, nil
		}
	}

	return nil, nil
}

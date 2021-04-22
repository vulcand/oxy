package stickycookie

import (
	"errors"
	"net/url"
)

// FallbackValue manages hashed sticky value.
type FallbackValue struct {
	from CookieValue
	to   CookieValue
}

// NewFallbackValue creates a new FallbackValue
func NewFallbackValue(from CookieValue, to CookieValue) (*FallbackValue, error) {
	if from == nil || to == nil {
		return nil, errors.New("from and to are mandatory")
	}

	return &FallbackValue{from: from, to: to}, nil
}

// Get hashes the sticky value.
func (v *FallbackValue) Get(raw *url.URL) string {
	return v.to.Get(raw)
}

// FindURL gets url from array that match the value.
// If it is a symmetric algorithm, it decodes the URL, otherwise it compares the ciphered values.
func (v *FallbackValue) FindURL(raw string, urls []*url.URL) (*url.URL, error) {
	findURL, err := v.from.FindURL(raw, urls)
	if findURL != nil {
		return findURL, err
	}

	return v.to.FindURL(raw, urls)
}

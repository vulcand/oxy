package stickycookie

import (
	"fmt"
	"net/url"

	"github.com/segmentio/fasthash/fnv1a"
)

// HashValue manages hashed sticky value.
type HashValue struct {
	// Salt secret to anonymize the hashed cookie
	Salt string
}

// Get hashes the sticky value.
func (v *HashValue) Get(raw *url.URL) string {
	return v.hash(raw.String())
}

// FindURL gets url from array that match the value.
func (v *HashValue) FindURL(raw string, urls []*url.URL) (*url.URL, error) {
	for _, u := range urls {
		if raw == v.hash(normalized(u)) {
			return u, nil
		}
	}

	return nil, nil
}

func (v *HashValue) hash(input string) string {
	return fmt.Sprintf("%x", fnv1a.HashString64(v.Salt+input))
}

func normalized(u *url.URL) string {
	normalized := url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}
	return normalized.String()
}

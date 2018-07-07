package utils

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

type readCloserTestImpl struct{}

func (r *readCloserTestImpl) Read(p []byte) (n int, err error) {
	return 0, nil
}

func (r *readCloserTestImpl) Close() error {
	return nil
}

// Just to make sure we don't panic, return err and not
// username and pass and cover the function
func TestHttpReqToString(t *testing.T) {
	req := &http.Request{
		URL:    &url.URL{Host: "localhost:2374", Path: "/unittest"},
		Method: http.MethodDelete,
		Cancel: make(chan struct{}),
		Body:   &readCloserTestImpl{},
	}

	assert.True(t, len(DumpHttpRequest(req)) > 0)
}

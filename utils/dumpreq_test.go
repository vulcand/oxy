package utils

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type readCloserTestImpl struct{}

func (r *readCloserTestImpl) Read(_ []byte) (n int, err error) {
	return 0, nil
}

func (r *readCloserTestImpl) Close() error {
	return nil
}

// Just to make sure we don't panic, return err and not
// username and pass and cover the function.
func TestHttpReqToString(t *testing.T) {
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodDelete, "http://localhost:2374/unittest", &readCloserTestImpl{})

	assert.NotEmpty(t, DumpHTTPRequest(req))
}

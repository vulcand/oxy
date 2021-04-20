// +build !go1.16

package forward

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
)

func TestChunkedResponseConversion(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		h := w.(http.Hijacker)
		conn, _, _ := h.Hijack()
		fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n4\r\ntest\r\n5\r\ntest1\r\n5\r\ntest2\r\n0\r\n\r\n")
		conn.Close()
	})
	defer srv.Close()

	f, err := New()
	require.NoError(t, err)

	proxy := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		f.ServeHTTP(w, req)
	})
	defer proxy.Close()

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, "testtest1test2", string(body))
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

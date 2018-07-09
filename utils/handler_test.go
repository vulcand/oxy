package utils

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultHandlerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.(http.Hijacker)
		conn, _, _ := h.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	request, err := http.NewRequest(http.MethodGet, srv.URL, strings.NewReader(""))
	require.NoError(t, err)

	_, err = http.DefaultTransport.RoundTrip(request)

	w := NewBufferWriter(NopWriteCloser(&bytes.Buffer{}))

	DefaultHandler.ServeHTTP(w, nil, err)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

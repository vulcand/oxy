package trace

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/testutils"
	"github.com/vulcand/oxy/v2/utils"
)

func TestTracer_simple(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "5")
		_, _ = w.Write([]byte("hello"))
	})

	trace := &bytes.Buffer{}
	tr, err := New(handler, trace)
	require.NoError(t, err)

	srv := httptest.NewServer(tr)
	t.Cleanup(srv.Close)

	re, _, err := testutils.MakeRequest(srv.URL+"/hello", testutils.Method(http.MethodPost), testutils.Body("123456"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	var r *Record
	require.NoError(t, json.Unmarshal(trace.Bytes(), &r))

	assert.Equal(t, http.MethodPost, r.Request.Method)
	assert.Equal(t, "/hello", r.Request.URL)
	assert.Equal(t, http.StatusOK, r.Response.Code)
	assert.EqualValues(t, 6, r.Request.BodyBytes)
	assert.NotEqual(t, float64(0), r.Response.Roundtrip)
	assert.EqualValues(t, 5, r.Response.BodyBytes)
}

func TestTracer_captureHeaders(t *testing.T) {
	respHeaders := http.Header{
		"X-Re-1": []string{"6", "7"},
		"X-Re-2": []string{"2", "3"},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		utils.CopyHeaders(w.Header(), respHeaders)
		_, _ = w.Write([]byte("hello"))
	})

	trace := &bytes.Buffer{}
	tr, err := New(handler, trace, RequestHeaders("X-Req-B", "X-Req-A"), ResponseHeaders("X-Re-1", "X-Re-2"))
	require.NoError(t, err)

	srv := httptest.NewServer(tr)
	t.Cleanup(srv.Close)

	reqHeaders := http.Header{"X-Req-A": []string{"1", "2"}, "X-Req-B": []string{"3", "4"}}
	re, _, err := testutils.Get(srv.URL+"/hello", testutils.Headers(reqHeaders))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	var r *Record
	require.NoError(t, json.Unmarshal(trace.Bytes(), &r))

	assert.Equal(t, reqHeaders, r.Request.Headers)
	assert.Equal(t, respHeaders, r.Response.Headers)
}

func TestTracer_TLS(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	trace := &bytes.Buffer{}
	tr, err := New(handler, trace)
	require.NoError(t, err)

	srv := httptest.NewUnstartedServer(tr)
	srv.StartTLS()
	t.Cleanup(srv.Close)

	config := &tls.Config{
		InsecureSkipVerify: true,
	}

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	conn, err := tls.Dial("tcp", u.Host, config)
	require.NoError(t, err)

	_, _ = fmt.Fprint(conn, "GET / HTTP/1.0\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "HTTP/1.0 200 OK\r\n", status)

	state := conn.ConnectionState()
	_ = conn.Close()

	var r *Record
	require.NoError(t, json.Unmarshal(trace.Bytes(), &r))
	assert.Equal(t, versionToString(state.Version), r.Request.TLS.Version)
}

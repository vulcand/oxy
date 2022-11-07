package forward

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/testutils"
)

func TestDefaultErrHandler(t *testing.T) {
	f := New(true)

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI("http://localhost:63450")
		f.ServeHTTP(w, req)
	}))
	defer proxy.Close()

	resp, err := http.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestXForwardedHostHeader(t *testing.T) {
	tests := []struct {
		Description            string
		PassHostHeader         bool
		TargetURL              string
		ProxyfiedURL           string
		ExpectedXForwardedHost string
	}{
		{
			Description:            "XForwardedHost without PassHostHeader",
			PassHostHeader:         false,
			TargetURL:              "http://xforwardedhost.com",
			ProxyfiedURL:           "http://backend.com",
			ExpectedXForwardedHost: "xforwardedhost.com",
		},
		{
			Description:            "XForwardedHost with PassHostHeader",
			PassHostHeader:         true,
			TargetURL:              "http://xforwardedhost.com",
			ProxyfiedURL:           "http://backend.com",
			ExpectedXForwardedHost: "xforwardedhost.com",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Description, func(t *testing.T) {
			t.Parallel()

			f := New(true)

			r, err := http.NewRequest(http.MethodGet, test.TargetURL, nil)
			require.NoError(t, err)

			backendURL, err := url.Parse(test.ProxyfiedURL)
			require.NoError(t, err)
			r.URL = backendURL

			f.Director(r)
			require.Equal(t, test.ExpectedXForwardedHost, r.Header.Get(XForwardedHost))
		})
	}
}

func TestForwardedProto(t *testing.T) {
	var proto string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		proto = req.Header.Get(XForwardedProto)
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	f := New(true)

	proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		f.ServeHTTP(w, req)
	}))
	proxy.StartTLS()
	defer proxy.Close()

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "https", proto)
}

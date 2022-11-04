package forward

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
)

func TestDefaultErrHandler(t *testing.T) {
	f := New(true)

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI("http://localhost:63450")
		f.ServeHTTP(w, req)
	}))
	defer proxy.Close()

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, re.StatusCode)
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
			r.URL = backendURL
			require.NoError(t, err)
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

	proxy := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		f.ServeHTTP(w, req)
	})
	tproxy := httptest.NewUnstartedServer(proxy)
	tproxy.StartTLS()
	defer tproxy.Close()

	re, _, err := testutils.Get(tproxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "https", proto)
}

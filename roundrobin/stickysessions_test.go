package roundrobin

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/testutils"
)

func TestBasic(t *testing.T) {
	a := testutils.NewResponder("a")
	b := testutils.NewResponder("b")

	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	sticky := NewStickySession("test")
	require.NotNil(t, sticky)

	lb, err := New(fwd, EnableStickySession(sticky))
	require.NoError(t, err)

	err = lb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = lb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)

	proxy := httptest.NewServer(lb)
	defer proxy.Close()

	client := http.DefaultClient

	for i := 0; i < 10; i++ {
		req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
		require.NoError(t, err)
		req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})

		resp, err := client.Do(req)
		require.NoError(t, err)

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)

		require.NoError(t, err)
		assert.Equal(t, "a", string(body))
	}
}

func TestStickyCookie(t *testing.T) {
	a := testutils.NewResponder("a")
	b := testutils.NewResponder("b")

	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	sticky := NewStickySession("test")
	require.NotNil(t, sticky)

	lb, err := New(fwd, EnableStickySession(sticky))
	require.NoError(t, err)

	err = lb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = lb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)

	proxy := httptest.NewServer(lb)
	defer proxy.Close()

	resp, err := http.Get(proxy.URL)
	require.NoError(t, err)

	cookie := resp.Cookies()[0]
	assert.Equal(t, "test", cookie.Name)
	assert.Equal(t, a.URL, cookie.Value)
}

func TestStickyCookieWithOptions(t *testing.T) {
	a := testutils.NewResponder("a")
	b := testutils.NewResponder("b")

	defer a.Close()
	defer b.Close()

	testCases := []struct {
		desc     string
		name     string
		options  CookieOptions
		expected *http.Cookie
	}{
		{
			desc:    "no options",
			name:    "test",
			options: CookieOptions{},
			expected: &http.Cookie{
				Name:  "test",
				Value: a.URL,
				Path:  "/",
				Raw:   fmt.Sprintf("test=%s; Path=/", a.URL),
			},
		},
		{
			desc: "HTTPOnly",
			name: "test",
			options: CookieOptions{
				HTTPOnly: true,
			},
			expected: &http.Cookie{
				Name:     "test",
				Value:    a.URL,
				Path:     "/",
				HttpOnly: true,
				Raw:      fmt.Sprintf("test=%s; Path=/; HttpOnly", a.URL),
				Unparsed: nil,
			},
		},
		{
			desc: "Secure",
			name: "test",
			options: CookieOptions{
				Secure: true,
			},
			expected: &http.Cookie{
				Name:   "test",
				Value:  a.URL,
				Path:   "/",
				Secure: true,
				Raw:    fmt.Sprintf("test=%s; Path=/; Secure", a.URL),
			},
		},
		{
			desc: "Path",
			name: "test",
			options: CookieOptions{
				Path: "/foo",
			},
			expected: &http.Cookie{
				Name:  "test",
				Value: a.URL,
				Path:  "/foo",
				Raw:   fmt.Sprintf("test=%s; Path=/foo", a.URL),
			},
		},
		{
			desc: "Domain",
			name: "test",
			options: CookieOptions{
				Domain: "example.org",
			},
			expected: &http.Cookie{
				Name:   "test",
				Value:  a.URL,
				Path:   "/",
				Domain: "example.org",
				Raw:    fmt.Sprintf("test=%s; Path=/; Domain=example.org", a.URL),
			},
		},
		{
			desc: "Expires",
			name: "test",
			options: CookieOptions{
				Expires: time.Date(1955, 11, 12, 1, 22, 0, 0, time.UTC),
			},
			expected: &http.Cookie{
				Name:       "test",
				Value:      a.URL,
				Path:       "/",
				Expires:    time.Date(1955, 11, 12, 1, 22, 0, 0, time.UTC),
				RawExpires: "Sat, 12 Nov 1955 01:22:00 GMT",
				Raw:        fmt.Sprintf("test=%s; Path=/; Expires=Sat, 12 Nov 1955 01:22:00 GMT", a.URL),
			},
		},
		{
			desc: "MaxAge",
			name: "test",
			options: CookieOptions{
				MaxAge: -20,
			},
			expected: &http.Cookie{
				Name:   "test",
				Value:  a.URL,
				Path:   "/",
				MaxAge: -1,
				Raw:    fmt.Sprintf("test=%s; Path=/; Max-Age=0", a.URL),
			},
		},
		{
			desc: "SameSite",
			name: "test",
			options: CookieOptions{
				SameSite: http.SameSiteNoneMode,
			},
			expected: &http.Cookie{
				Name:     "test",
				Value:    a.URL,
				Path:     "/",
				SameSite: http.SameSiteNoneMode,
				Raw:      fmt.Sprintf("test=%s; Path=/; SameSite=None", a.URL),
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {

			fwd, err := forward.New()
			require.NoError(t, err)

			sticky := NewStickySessionWithOptions(test.name, test.options)
			require.NotNil(t, sticky)

			lb, err := New(fwd, EnableStickySession(sticky))
			require.NoError(t, err)

			err = lb.UpsertServer(testutils.ParseURI(a.URL))
			require.NoError(t, err)
			err = lb.UpsertServer(testutils.ParseURI(b.URL))
			require.NoError(t, err)

			proxy := httptest.NewServer(lb)
			defer proxy.Close()

			resp, err := http.Get(proxy.URL)
			require.NoError(t, err)

			require.Len(t, resp.Cookies(), 1)
			assert.Equal(t, test.expected, resp.Cookies()[0])
		})
	}
}

func TestRemoveRespondingServer(t *testing.T) {
	a := testutils.NewResponder("a")
	b := testutils.NewResponder("b")

	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	sticky := NewStickySession("test")
	require.NotNil(t, sticky)

	lb, err := New(fwd, EnableStickySession(sticky))
	require.NoError(t, err)

	err = lb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = lb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)

	proxy := httptest.NewServer(lb)
	defer proxy.Close()

	client := http.DefaultClient

	for i := 0; i < 10; i++ {
		req, errReq := http.NewRequest(http.MethodGet, proxy.URL, nil)
		require.NoError(t, errReq)

		req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})

		resp, errReq := client.Do(req)
		require.NoError(t, errReq)
		defer resp.Body.Close()

		body, errReq := ioutil.ReadAll(resp.Body)
		require.NoError(t, errReq)

		assert.Equal(t, "a", string(body))
	}

	err = lb.RemoveServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)

	// Now, use the organic cookie response in our next requests.
	req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})
	resp, err := client.Do(req)
	require.NoError(t, err)

	assert.Equal(t, "test", resp.Cookies()[0].Name)
	assert.Equal(t, b.URL, resp.Cookies()[0].Value)

	for i := 0; i < 10; i++ {
		req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)

		require.NoError(t, err)
		assert.Equal(t, "b", string(body))
	}
}

func TestRemoveAllServers(t *testing.T) {
	a := testutils.NewResponder("a")
	b := testutils.NewResponder("b")

	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	sticky := NewStickySession("test")
	require.NotNil(t, sticky)

	lb, err := New(fwd, EnableStickySession(sticky))
	require.NoError(t, err)

	err = lb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = lb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)

	proxy := httptest.NewServer(lb)
	defer proxy.Close()

	client := http.DefaultClient

	for i := 0; i < 10; i++ {
		req, errReq := http.NewRequest(http.MethodGet, proxy.URL, nil)
		require.NoError(t, errReq)
		req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})

		resp, errReq := client.Do(req)
		require.NoError(t, errReq)
		defer resp.Body.Close()

		body, errReq := ioutil.ReadAll(resp.Body)
		require.NoError(t, errReq)

		assert.Equal(t, "a", string(body))
	}

	err = lb.RemoveServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = lb.RemoveServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)

	// Now, use the organic cookie response in our next requests.
	req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestBadCookieVal(t *testing.T) {
	a := testutils.NewResponder("a")

	defer a.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	sticky := NewStickySession("test")
	require.NotNil(t, sticky)

	lb, err := New(fwd, EnableStickySession(sticky))
	require.NoError(t, err)

	err = lb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)

	proxy := httptest.NewServer(lb)
	defer proxy.Close()

	client := http.DefaultClient

	req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "test", Value: "This is a patently invalid url!  You can't parse it!  :-)"})

	resp, err := client.Do(req)
	require.NoError(t, err)

	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "a", string(body))

	// Now, cycle off the good server to cause an error
	err = lb.RemoveServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)

	_, err = ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

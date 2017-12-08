package roundrobin

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/testutils"

	. "gopkg.in/check.v1"
)

func TestStickySession(t *testing.T) { TestingT(t) }

type StickySessionSuite struct{}

var _ = Suite(&StickySessionSuite{})

func (s *StickySessionSuite) TestBasic(c *C) {
	a := testutils.NewResponder("a")
	b := testutils.NewResponder("b")

	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	c.Assert(err, IsNil)

	sticky := NewStickySession("test")
	c.Assert(sticky, NotNil)

	lb, err := New(fwd, EnableStickySession(sticky))
	c.Assert(err, IsNil)

	err = lb.UpsertServer(testutils.ParseURI(a.URL))
	c.Assert(err, IsNil)
	err = lb.UpsertServer(testutils.ParseURI(b.URL))
	c.Assert(err, IsNil)

	proxy := httptest.NewServer(lb)
	defer proxy.Close()

	client := http.DefaultClient

	for i := 0; i < 10; i++ {
		req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
		c.Assert(err, IsNil)
		req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})

		resp, err := client.Do(req)
		c.Assert(err, IsNil)

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)

		c.Assert(err, IsNil)
		c.Assert(string(body), Equals, "a")
	}
}

func (s *StickySessionSuite) TestStickCookie(c *C) {
	a := testutils.NewResponder("a")
	b := testutils.NewResponder("b")

	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	c.Assert(err, IsNil)

	sticky := NewStickySession("test")
	c.Assert(sticky, NotNil)

	lb, err := New(fwd, EnableStickySession(sticky))
	c.Assert(err, IsNil)

	err = lb.UpsertServer(testutils.ParseURI(a.URL))
	c.Assert(err, IsNil)
	err = lb.UpsertServer(testutils.ParseURI(b.URL))
	c.Assert(err, IsNil)

	proxy := httptest.NewServer(lb)
	defer proxy.Close()

	resp, err := http.Get(proxy.URL)
	c.Assert(err, IsNil)

	cookie := resp.Cookies()[0]
	c.Assert(cookie.Name, Equals, "test")
	c.Assert(cookie.Value, Equals, a.URL)
}

func (s *StickySessionSuite) TestRemoveRespondingServer(c *C) {
	a := testutils.NewResponder("a")
	b := testutils.NewResponder("b")

	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	c.Assert(err, IsNil)

	sticky := NewStickySession("test")
	c.Assert(sticky, NotNil)

	lb, err := New(fwd, EnableStickySession(sticky))
	c.Assert(err, IsNil)

	err = lb.UpsertServer(testutils.ParseURI(a.URL))
	c.Assert(err, IsNil)
	err = lb.UpsertServer(testutils.ParseURI(b.URL))
	c.Assert(err, IsNil)

	proxy := httptest.NewServer(lb)
	defer proxy.Close()

	client := http.DefaultClient

	for i := 0; i < 10; i++ {
		req, errReq := http.NewRequest(http.MethodGet, proxy.URL, nil)
		c.Assert(errReq, IsNil)
		req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})

		resp, errReq := client.Do(req)
		c.Assert(errReq, IsNil)

		defer resp.Body.Close()
		body, errReq := ioutil.ReadAll(resp.Body)

		c.Assert(errReq, IsNil)
		c.Assert(string(body), Equals, "a")
	}

	err = lb.RemoveServer(testutils.ParseURI(a.URL))
	c.Assert(err, IsNil)

	// Now, use the organic cookie response in our next requests.
	req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
	c.Assert(err, IsNil)
	req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})
	resp, err := client.Do(req)
	c.Assert(err, IsNil)

	c.Assert(resp.Cookies()[0].Name, Equals, "test")
	c.Assert(resp.Cookies()[0].Value, Equals, b.URL)

	for i := 0; i < 10; i++ {
		req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
		c.Assert(err, IsNil)

		resp, err := client.Do(req)
		c.Assert(err, IsNil)

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)

		c.Assert(err, IsNil)
		c.Assert(string(body), Equals, "b")
	}
}

func (s *StickySessionSuite) TestRemoveAllServers(c *C) {
	a := testutils.NewResponder("a")
	b := testutils.NewResponder("b")

	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	c.Assert(err, IsNil)

	sticky := NewStickySession("test")
	c.Assert(sticky, NotNil)

	lb, err := New(fwd, EnableStickySession(sticky))
	c.Assert(err, IsNil)

	err = lb.UpsertServer(testutils.ParseURI(a.URL))
	c.Assert(err, IsNil)
	err = lb.UpsertServer(testutils.ParseURI(b.URL))
	c.Assert(err, IsNil)

	proxy := httptest.NewServer(lb)
	defer proxy.Close()

	client := http.DefaultClient

	for i := 0; i < 10; i++ {
		req, errReq := http.NewRequest(http.MethodGet, proxy.URL, nil)
		c.Assert(errReq, IsNil)
		req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})

		resp, errReq := client.Do(req)
		c.Assert(errReq, IsNil)

		defer resp.Body.Close()
		body, errReq := ioutil.ReadAll(resp.Body)

		c.Assert(errReq, IsNil)
		c.Assert(string(body), Equals, "a")
	}

	err = lb.RemoveServer(testutils.ParseURI(a.URL))
	c.Assert(err, IsNil)
	err = lb.RemoveServer(testutils.ParseURI(b.URL))
	c.Assert(err, IsNil)

	// Now, use the organic cookie response in our next requests.
	req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
	c.Assert(err, IsNil)
	req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})
	resp, err := client.Do(req)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, http.StatusInternalServerError)
}

func (s *StickySessionSuite) TestBadCookieVal(c *C) {
	a := testutils.NewResponder("a")

	defer a.Close()

	fwd, err := forward.New()
	c.Assert(err, IsNil)

	sticky := NewStickySession("test")
	c.Assert(sticky, NotNil)

	lb, err := New(fwd, EnableStickySession(sticky))
	c.Assert(err, IsNil)

	err = lb.UpsertServer(testutils.ParseURI(a.URL))
	c.Assert(err, IsNil)

	proxy := httptest.NewServer(lb)
	defer proxy.Close()

	client := http.DefaultClient

	req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
	c.Assert(err, IsNil)
	req.AddCookie(&http.Cookie{Name: "test", Value: "This is a patently invalid url!  You can't parse it!  :-)"})

	resp, err := client.Do(req)
	c.Assert(err, IsNil)

	body, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	c.Assert(string(body), Equals, "a")

	// Now, cycle off the good server to cause an error
	err = lb.RemoveServer(testutils.ParseURI(a.URL))
	c.Assert(err, IsNil)

	resp, err = client.Do(req)
	c.Assert(err, IsNil)

	_, err = ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, http.StatusInternalServerError)
}

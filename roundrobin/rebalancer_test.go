package roundrobin

import (
	"net/http"
	"net/http/httptest"

	"github.com/mailgun/oxy/forward"
	"github.com/mailgun/oxy/testutils"

	. "gopkg.in/check.v1"
)

func (s *RRSuite) TestRebalancerNormalOperation(c *C) {
	a := testutils.NewResponder("a")
	defer a.Close()

	b := testutils.NewResponder("b")
	defer b.Close()

	fwd, err := forward.New()
	c.Assert(err, IsNil)

	lb, err := New(fwd)
	c.Assert(err, IsNil)

	rb, err := NewRebalancer(lb)
	c.Assert(err, IsNil)

	rb.UpsertServer(testutils.ParseURI(a.URL))

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	c.Assert(s.seq(c, proxy.URL, 3), DeepEquals, []string{"a", "a", "a"})
}

func (s *RRSuite) TestRebalancerNoServers(c *C) {
	fwd, err := forward.New()
	c.Assert(err, IsNil)

	lb, err := New(fwd)
	c.Assert(err, IsNil)

	rb, err := NewRebalancer(lb)
	c.Assert(err, IsNil)

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	re, _, err := testutils.Get(proxy.URL)
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusInternalServerError)
}

func (s *RRSuite) TestRebalancerOneIsDown(c *C) {
	a := testutils.NewResponder("a")
	defer a.Close()

	b := testutils.NewResponder("b")
	defer b.Close()

	fwd, err := forward.New()
	c.Assert(err, IsNil)

	lb, err := New(fwd)
	c.Assert(err, IsNil)

	rb, err := NewRebalancer(lb)
	c.Assert(err, IsNil)

	rb.UpsertServer(testutils.ParseURI(a.URL))

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	c.Assert(s.seq(c, proxy.URL, 3), DeepEquals, []string{"a", "a", "a"})
}

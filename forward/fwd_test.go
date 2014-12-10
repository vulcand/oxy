package forward

import (
	"net/http"
	"testing"

	"github.com/mailgun/oxy/testutils"

	. "gopkg.in/check.v1"
)

func TestFwd(t *testing.T) { TestingT(t) }

type FwdSuite struct{}

var _ = Suite(&FwdSuite{})

func (s *FwdSuite) TestForwardSimple(c *C) {
	called := false
	srv := testutils.NewTestServer(func(w http.ResponseWriter, req *http.Request) {
		called = true
		w.Write([]byte("hello"))
	})
	defer srv.Close()

	f, err := New()
	c.Assert(err, IsNil)

	proxy := testutils.NewTestServer(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		f.ServeHTTP(w, req)
	})
	defer proxy.Close()

	re, body, err := testutils.Get(proxy.URL, testutils.Opts{})
	c.Assert(err, IsNil)
	c.Assert(string(body), Equals, "hello")
	c.Assert(re.StatusCode, Equals, http.StatusOK)

}

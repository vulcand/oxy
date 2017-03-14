package utils

import (
	. "gopkg.in/check.v1"
	"net/http"
	"net/url"
)

type DumpHttpReqSuite struct {
}

var _ = Suite(&DumpHttpReqSuite{})

type readCloserTestImpl struct {
}

func (r *readCloserTestImpl) Read(p []byte) (n int, err error) {
	return 0, nil
}

func (r *readCloserTestImpl) Close() error {
	return nil
}

//Just to make sure we don't panic, return err and not
//username and pass and cover the function
func (s *DumpHttpReqSuite) TestHttpReqToString(c *C) {
	req := &http.Request{
		URL:    &url.URL{Host: "localhost:2374", Path: "/unittest"},
		Method: "DELETE",
		Cancel: make(chan struct{}),
		Body:   &readCloserTestImpl{},
	}

	c.Assert(DumpHttpRequest(req), Equals, "{\"Method\":\"DELETE\",\"URL\":{\"Scheme\":\"\",\"Opaque\":\"\",\"User\":null,\"Host\":\"localhost:2374\",\"Path\":\"/unittest\",\"RawPath\":\"\",\"ForceQuery\":false,\"RawQuery\":\"\",\"Fragment\":\"\"},\"Proto\":\"\",\"ProtoMajor\":0,\"ProtoMinor\":0,\"Header\":null,\"ContentLength\":0,\"TransferEncoding\":null,\"Host\":\"\",\"Form\":null,\"PostForm\":null,\"MultipartForm\":null,\"Trailer\":null,\"RemoteAddr\":\"\",\"RequestURI\":\"\",\"TLS\":null}")
}

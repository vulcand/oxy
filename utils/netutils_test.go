package utils

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Make sure copy does it right, so the copied url
// is safe to alter without modifying the other
func TestCopyUrl(t *testing.T) {
	urlA := &url.URL{
		Scheme:   "http",
		Host:     "localhost:5000",
		Path:     "/upstream",
		Opaque:   "opaque",
		RawQuery: "a=1&b=2",
		Fragment: "#hello",
		User:     &url.Userinfo{},
	}

	urlB := CopyURL(urlA)
	assert.Equal(t, urlA, urlB)

	urlB.Scheme = "https"
	assert.NotEqual(t, urlA, urlB)
}

// Make sure copy headers is not shallow and copies all headers
func TestCopyHeaders(t *testing.T) {
	source, destination := make(http.Header), make(http.Header)
	source.Add("a", "b")
	source.Add("c", "d")

	CopyHeaders(destination, source)

	assert.Equal(t, "b", destination.Get("a"))
	assert.Equal(t, "d", destination.Get("c"))

	// make sure that altering source does not affect the destination
	source.Del("a")

	assert.Equal(t, "", source.Get("a"))
	assert.Equal(t, "b", destination.Get("a"))
}

func TestHasHeaders(t *testing.T) {
	source := make(http.Header)
	source.Add("a", "b")
	source.Add("c", "d")

	assert.True(t, HasHeaders([]string{"a", "f"}, source))
	assert.False(t, HasHeaders([]string{"i", "j"}, source))
}

func TestRemoveHeaders(t *testing.T) {
	source := make(http.Header)
	source.Add("a", "b")
	source.Add("a", "m")
	source.Add("c", "d")

	RemoveHeaders(source, "a")

	assert.Equal(t, "", source.Get("a"))
	assert.Equal(t, "d", source.Get("c"))
}

func BenchmarkCopyHeaders(b *testing.B) {
	dstHeaders := make([]http.Header, 0, b.N)
	sourceHeaders := make([]http.Header, 0, b.N)
	for n := 0; n < b.N; n++ {
		// example from a reverse proxy merging headers
		d := http.Header{}
		d.Add("Request-Id", "1bd36bcc-a0d1-4fc7-aedc-20bbdefa27c5")
		dstHeaders = append(dstHeaders, d)

		s := http.Header{}
		s.Add("Content-Length", "374")
		s.Add("Context-Type", "text/html; charset=utf-8")
		s.Add("Etag", `"op14g6ae"`)
		s.Add("Last-Modified", "Wed, 26 Apr 2017 18:24:06 GMT")
		s.Add("Server", "Caddy")
		s.Add("Date", "Fri, 28 Apr 2017 15:54:01 GMT")
		s.Add("Accept-Ranges", "bytes")
		sourceHeaders = append(sourceHeaders, s)
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		CopyHeaders(dstHeaders[n], sourceHeaders[n])
	}
}

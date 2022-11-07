// Package forward creates a pre-configured httputil.ReverseProxy.
package forward

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/vulcand/oxy/v2/utils"
)

// New creates a new ReverseProxy.
func New(passHostHeader bool) *httputil.ReverseProxy {
	h := NewHeaderRewriter()

	return &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			modifyRequest(request)

			h.Rewrite(request)

			if !passHostHeader {
				request.Host = request.URL.Host
			}
		},
		ErrorHandler: utils.DefaultHandler.ServeHTTP,
	}
}

// Modify the request to handle the target URL.
func modifyRequest(outReq *http.Request) {
	u := getURLFromRequest(outReq)

	outReq.URL.Path = u.Path
	outReq.URL.RawPath = u.RawPath
	outReq.URL.RawQuery = u.RawQuery
	outReq.RequestURI = "" // Outgoing request should not have RequestURI

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1
}

func getURLFromRequest(req *http.Request) *url.URL {
	// If the Request was created by Go via a real HTTP request,
	// RequestURI will contain the original query string.
	// If the Request was created in code,
	// RequestURI will be empty, and we will use the URL object instead
	u := req.URL
	if req.RequestURI != "" {
		parsedURL, err := url.ParseRequestURI(req.RequestURI)
		if err == nil {
			return parsedURL
		}
	}
	return u
}

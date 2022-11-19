package cbreaker

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/vulcand/oxy/v2/utils"
)

// Response response model.
type Response struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

// ResponseFallback fallback response handler.
type ResponseFallback struct {
	r Response

	debug bool
	log   utils.Logger
}

// NewResponseFallback creates a new ResponseFallback.
func NewResponseFallback(r Response, options ...ResponseFallbackOption) (*ResponseFallback, error) {
	rf := &ResponseFallback{r: r, log: &utils.NoopLogger{}}

	for _, s := range options {
		if err := s(rf); err != nil {
			return nil, err
		}
	}

	return rf, nil
}

func (f *ResponseFallback) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if f.debug {
		dump := utils.DumpHTTPRequest(req)
		f.log.Debug("vulcand/oxy/fallback/response: begin ServeHttp on request: %s", dump)
		defer f.log.Debug("vulcand/oxy/fallback/response: completed ServeHttp on request: %s", dump)
	}

	if f.r.ContentType != "" {
		w.Header().Set("Content-Type", f.r.ContentType)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(f.r.Body)))
	w.WriteHeader(f.r.StatusCode)
	_, err := w.Write(f.r.Body)
	if err != nil {
		f.log.Error("vulcand/oxy/fallback/response: failed to write response, err: %v", err)
	}
}

// Redirect redirect model.
type Redirect struct {
	URL          string
	PreservePath bool
}

// RedirectFallback fallback redirect handler.
type RedirectFallback struct {
	r Redirect

	u *url.URL

	debug bool
	log   utils.Logger
}

// NewRedirectFallback creates a new RedirectFallback.
func NewRedirectFallback(r Redirect, options ...RedirectFallbackOption) (*RedirectFallback, error) {
	u, err := url.ParseRequestURI(r.URL)
	if err != nil {
		return nil, err
	}

	rf := &RedirectFallback{r: r, u: u, log: &utils.NoopLogger{}}

	for _, s := range options {
		if err := s(rf); err != nil {
			return nil, err
		}
	}

	return rf, nil
}

func (f *RedirectFallback) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if f.debug {
		dump := utils.DumpHTTPRequest(req)
		f.log.Debug("vulcand/oxy/fallback/redirect: begin ServeHttp on request: %s", dump)
		defer f.log.Debug("vulcand/oxy/fallback/redirect: completed ServeHttp on request: %s", dump)
	}

	location := f.u.String()
	if f.r.PreservePath {
		location += req.URL.Path
	}

	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusFound)
	_, err := w.Write([]byte(http.StatusText(http.StatusFound)))
	if err != nil {
		f.log.Error("vulcand/oxy/fallback/redirect: failed to write response, err: %v", err)
	}
}

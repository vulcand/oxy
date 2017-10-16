package cbreaker

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/vulcand/oxy/utils"
)

type Response struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

type ResponseFallback struct {
	r Response

	log *log.Logger
}

func NewResponseFallbackWithLogger(r Response, l *log.Logger) (*ResponseFallback, error) {
	if r.StatusCode == 0 {
		return nil, fmt.Errorf("response code should not be 0")
	}
	return &ResponseFallback{r: r, log: l}, nil
}

func NewResponseFallback(r Response) (*ResponseFallback, error) {
	return NewResponseFallbackWithLogger(r, log.StandardLogger())
}

func (f *ResponseFallback) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if f.log.Level >= log.DebugLevel {
		logEntry := f.log.WithField("Request", utils.DumpHttpRequest(req))
		logEntry.Debug("vulcand/oxy/fallback/response: begin ServeHttp on request")
		defer logEntry.Debug("vulcand/oxy/fallback/response: competed ServeHttp on request")
	}

	if f.r.ContentType != "" {
		w.Header().Set("Content-Type", f.r.ContentType)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(f.r.Body)))
	w.WriteHeader(f.r.StatusCode)
	w.Write(f.r.Body)
}

type Redirect struct {
	URL string
}

type RedirectFallback struct {
	u *url.URL

	log *log.Logger
}

func NewRedirectFallbackWithLogger(r Redirect, l *log.Logger) (*RedirectFallback, error) {
	u, err := url.ParseRequestURI(r.URL)
	if err != nil {
		return nil, err
	}
	return &RedirectFallback{u: u, log: l}, nil
}

func NewRedirectFallback(r Redirect) (*RedirectFallback, error) {
	return NewRedirectFallbackWithLogger(r, log.StandardLogger())
}

func (f *RedirectFallback) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if f.log.Level >= log.DebugLevel {
		logEntry := f.log.WithField("Request", utils.DumpHttpRequest(req))
		logEntry.Debug("vulcand/oxy/fallback/redirect: begin ServeHttp on request")
		defer logEntry.Debug("vulcand/oxy/fallback/redirect: competed ServeHttp on request")
	}

	w.Header().Set("Location", f.u.String())
	w.WriteHeader(http.StatusFound)
	w.Write([]byte(http.StatusText(http.StatusFound)))
}

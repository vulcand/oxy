package utils

import (
	"net"
	"net/http"
)

type ErrorHandler interface {
	ServeHTTP(w http.ResponseWriter, req *http.Request, err error)
}

type DefaultHandler struct {
}

func (e *DefaultHandler) ServeHTTP(w http.ResponseWriter, req *http.Request, err error) {
	statusCode := http.StatusInternalServerError
	if e, ok := err.(net.Error); ok {
		if e.Timeout() {
			statusCode = http.StatusGatewayTimeout
		} else {
			statusCode = http.StatusBadGateway
		}
	}
	w.WriteHeader(statusCode)
	w.Write([]byte(http.StatusText(statusCode)))
}

type ErrorHandlerFunc func(http.ResponseWriter, *http.Request, error)

// ServeHTTP calls f(w, r).
func (f ErrorHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, err error) {
	f(w, r, err)
}

package utils

import (
	"context"
	"io"
	"net"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// ClientClosedRequest non-standard HTTP status code for client disconnection
const ClientClosedRequest = 499

// StatusClientClosedRequest non-standard HTTP status for client disconnection
const StatusClientClosedRequest = "Client Closed Request"

// ErrorHandler error handler
type ErrorHandler interface {
	ServeHTTP(w http.ResponseWriter, req *http.Request, err error)
}

// DefaultHandler default error handler
var DefaultHandler ErrorHandler = &StdHandler{}

// StdHandler Standard error handler
type StdHandler struct{}

func (e *StdHandler) ServeHTTP(w http.ResponseWriter, req *http.Request, err error) {
	statusCode := http.StatusInternalServerError
	statusText := http.StatusText(statusCode)

	if e, ok := err.(net.Error); ok {
		if e.Timeout() {
			statusCode = http.StatusGatewayTimeout
			statusText = http.StatusText(statusCode)
		} else {
			statusCode = http.StatusBadGateway
			statusText = http.StatusText(statusCode)
		}
	} else if err == io.EOF {
		statusCode = http.StatusBadGateway
		statusText = http.StatusText(statusCode)
	} else if err == context.Canceled {
		statusCode = ClientClosedRequest
		statusText = StatusClientClosedRequest
	}

	w.WriteHeader(statusCode)
	w.Write([]byte(statusText))
	log.Debugf("'%d %s' caused by: %v", statusCode, statusText, err)
}

// ErrorHandlerFunc error handler function type
type ErrorHandlerFunc func(http.ResponseWriter, *http.Request, error)

// ServeHTTP calls f(w, r).
func (f ErrorHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, err error) {
	f(w, r, err)
}

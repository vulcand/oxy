package forward

import (
	"net/http"
	"net/url"
)

// Connection states.
const (
	StateConnected = iota
	StateDisconnected
)

// URLForwardingStateListener URL forwarding state listener.
type URLForwardingStateListener func(*url.URL, int)

// StateListener listens on state change for urls.
type StateListener struct {
	next          http.Handler
	stateListener URLForwardingStateListener
}

// NewStateListener creates a new StateListener middleware.
func NewStateListener(next http.Handler, stateListener URLForwardingStateListener) *StateListener {
	return &StateListener{next: next, stateListener: stateListener}
}

func (s *StateListener) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.stateListener(req.URL, StateConnected)
	s.next.ServeHTTP(rw, req)
	s.stateListener(req.URL, StateDisconnected)
}

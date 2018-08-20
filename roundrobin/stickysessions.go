package roundrobin

import (
	"net/http"
	"net/url"
)

// StickySession is a mixin for load balancers that implements layer 7 (http cookie) session affinity
type StickySession struct {
	cookieName string
	obfuscator Obfuscator
}

// NewStickySession creates a new StickySession
func NewStickySession(cookieName string) *StickySession {
	return &StickySession{cookieName: cookieName, obfuscator: &DefaultObfuscator{}}
}

// NewStickySessionWithObfuscator creates a new StickySession with a custom Obfuscator
func NewStickySessionWithObfuscator(cookieName string, obfuscator Obfuscator) *StickySession {
	return &StickySession{cookieName: cookieName, obfuscator: obfuscator}
}

// GetBackend returns the backend URL stored in the sticky cookie, iff the backend is still in the valid list of servers.
func (s *StickySession) GetBackend(req *http.Request, servers []*url.URL) (*url.URL, bool, error) {
	cookie, err := req.Cookie(s.cookieName)
	switch err {
	case nil:
	case http.ErrNoCookie:
		return nil, false, nil
	default:
		return nil, false, err
	}

	clearValue := s.obfuscator.Normalize(cookie.Value)
	serverURL, err := url.Parse(clearValue)
	if err != nil {
		return nil, false, err
	}

	if s.isBackendAlive(serverURL, servers) {
		return serverURL, true, nil
	}
	return nil, false, nil
}

// StickBackend creates and sets the cookie
func (s *StickySession) StickBackend(backend *url.URL, w *http.ResponseWriter) {
	obfusValue := s.obfuscator.Obfuscate(backend.String())
	cookie := &http.Cookie{Name: s.cookieName, Value: obfusValue, Path: "/"}
	http.SetCookie(*w, cookie)
}

func (s *StickySession) isBackendAlive(needle *url.URL, haystack []*url.URL) bool {
	if len(haystack) == 0 {
		return false
	}

	for _, serverURL := range haystack {
		if sameURL(needle, serverURL) {
			return true
		}
	}
	return false
}

// package stickysession is a mixin for load balancers that implements layer 7 (http cookie) session affinity
package roundrobin

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
)

var (
	defaultSalt       = "vulcand/oxy"
	defaultObfuscator = &MD5Obfucator{salt: defaultSalt, data: make(map[string]string)}
)

type Obfuscator interface {
	Obfuscate(string) string
	Normalize(string) string
}

// md5Table stores two mappings, one is plaintext to md5, the other is md5 to plaintext
type MD5Obfucator struct {
	salt string
	data map[string]string
}

func MD5(text, salt string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(text+salt)))
}

func (m *MD5Obfucator) Obfuscate(text string) string {
	v, ok := m.data[text]
	if ok {
		return v
	}

	md5_value := MD5(text, m.salt)
	m.data[md5_value] = text
	m.data[text] = md5_value
	return md5_value
}

func (m *MD5Obfucator) Normalize(md5_value string) string {
	v, ok := m.data[md5_value]
	if !ok {
		return ""
	}
	return v
}

type StickySession struct {
	cookieName string
	obfuscator Obfuscator
}

func NewStickySession(cookieName string, o Obfuscator) *StickySession {
	if o == nil {
		o = defaultObfuscator
	}
	return &StickySession{cookieName: cookieName, obfuscator: o}
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
	backend := s.obfuscator.Normalize(cookie.Value)
	serverURL, err := url.Parse(backend)
	if err != nil {
		return nil, false, err
	}

	if s.isBackendAlive(serverURL, servers) {
		return serverURL, true, nil
	} else {
		return nil, false, nil
	}
}

func (s *StickySession) StickBackend(backend *url.URL, w *http.ResponseWriter) {
	cookieValue := s.obfuscator.Obfuscate(backend.String())
	cookie := &http.Cookie{Name: s.cookieName, Value: cookieValue, Path: "/"}
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

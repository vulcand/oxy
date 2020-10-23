package roundrobin

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/fasthash/fnv1a"
)

// CookieOptions has all the options one would like to set on the affinity cookie
type CookieOptions struct {
	HTTPOnly bool
	Secure   bool

	Path    string
	Domain  string
	Expires time.Time

	MaxAge   int
	SameSite http.SameSite
}

// StickySession is a mixin for load balancers that implements layer 7 (http cookie) session affinity
type StickySession struct {
	cookieName string
	options    CookieOptions
	hashCache  map[string]string
	hashRWMu   sync.RWMutex
	hashMu     sync.Mutex
}

// NewStickySession creates a new StickySession
func NewStickySession(cookieName string) *StickySession {
	return &StickySession{cookieName: cookieName, hashCache: make(map[string]string)}
}

// NewStickySessionWithOptions creates a new StickySession whilst allowing for options to
// shape its affinity cookie such as "httpOnly" or "secure"
func NewStickySessionWithOptions(cookieName string, options CookieOptions) *StickySession {
	return &StickySession{cookieName: cookieName, options: options}
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

	if found, serverURL := s.isBackendAlive(cookie.Value, servers); found {
		return serverURL, true, nil
	}
	return nil, false, nil
}

// StickBackend creates and sets the cookie
func (s *StickySession) StickBackend(backend *url.URL, w *http.ResponseWriter) {
	opt := s.options

	cp := "/"
	if opt.Path != "" {
		cp = opt.Path
	}

	cookie := &http.Cookie{
		Name:     s.cookieName,
		Value:    hash(backend.String()),
		Path:     cp,
		Domain:   opt.Domain,
		Expires:  opt.Expires,
		MaxAge:   opt.MaxAge,
		Secure:   opt.Secure,
		HttpOnly: opt.HTTPOnly,
		SameSite: opt.SameSite,
	}
	http.SetCookie(*w, cookie)
}

func (s *StickySession) isBackendAlive(needle string, haystack []*url.URL) (bool, *url.URL) {
	if len(haystack) == 0 {
		return false, nil
	}

	switch {
	case strings.HasPrefix(needle, "http"):
		for _, serverURL := range haystack {
			if needle == serverURL.String() {
				return true, serverURL
			}
		}
	default:
		for _, serverURL := range haystack {
			if needle == hash(serverURL.String()) {
				return true, serverURL
			}
		}
	}

	return false, nil
}

func (s *StickySession) isBackendAliveRWMutexRlock(needle string, haystack []*url.URL) (bool, *url.URL) {
	if len(haystack) == 0 {
		return false, nil
	}

	switch {
	case strings.HasPrefix(needle, "http"):
		for _, serverURL := range haystack {
			if needle == serverURL.String() {
				return true, serverURL
			}
		}
	default:
		var h string
		var str string
		var found bool

		for _, serverURL := range haystack {
			str = serverURL.String()

			s.hashRWMu.RLock()
			if h, found = s.hashCache[str]; !found {
				s.hashRWMu.RUnlock()

				h = hash(str)

				s.hashRWMu.Lock()
				s.hashCache[str] = h
				s.hashRWMu.Unlock()
			} else {
				s.hashRWMu.RUnlock()
			}

			if needle == h {
				return true, serverURL
			}
		}

		// hash cache clean up to remove old entries which do not exist anymore
		s.hashRWMu.Lock()
		for str, h = range s.hashCache {
			if h == needle {
				delete(s.hashCache, str)
				break
			}
		}
		s.hashRWMu.Unlock()
	}

	return false, nil
}

func (s *StickySession) isBackendAliveRWMutexLock(needle string, haystack []*url.URL) (bool, *url.URL) {
	if len(haystack) == 0 {
		return false, nil
	}

	switch {
	case strings.HasPrefix(needle, "http"):
		for _, serverURL := range haystack {
			if needle == serverURL.String() {
				return true, serverURL
			}
		}
	default:
		var h string
		var str string
		var found bool

		for _, serverURL := range haystack {
			str = serverURL.String()

			s.hashRWMu.Lock()
			if h, found = s.hashCache[str]; !found {
				h = hash(str)
				s.hashCache[str] = h
			}

			s.hashRWMu.Unlock()

			if needle == h {
				return true, serverURL
			}
		}

		// hash cache clean up to remove old entries which do not exist anymore
		s.hashRWMu.Lock()
		for str, h = range s.hashCache {
			if h == needle {
				delete(s.hashCache, str)
				break
			}
		}
		s.hashRWMu.Unlock()
	}

	return false, nil
}

func (s *StickySession) isBackendAliveMutexLock(needle string, haystack []*url.URL) (bool, *url.URL) {
	if len(haystack) == 0 {
		return false, nil
	}

	switch {
	case strings.HasPrefix(needle, "http"):
		for _, serverURL := range haystack {
			if needle == serverURL.String() {
				return true, serverURL
			}
		}
	default:
		var h string
		var str string
		var found bool

		for _, serverURL := range haystack {
			str = serverURL.String()

			s.hashMu.Lock()
			if h, found = s.hashCache[str]; !found {
				h = hash(str)
				s.hashCache[str] = h
			}
			s.hashMu.Unlock()

			if needle == h {
				return true, serverURL
			}
		}

		// hash cache clean up to remove old entries which do not exist anymore
		s.hashMu.Lock()
		for str, h = range s.hashCache {
			if h == needle {
				delete(s.hashCache, str)
				break
			}
		}
		s.hashMu.Unlock()
	}

	return false, nil
}

func hash(input string) string {
	return fmt.Sprintf("%x", fnv1a.HashString64(input))
}

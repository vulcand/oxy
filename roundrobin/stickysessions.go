package roundrobin

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/fasthash/fnv1a"
	"github.com/vulcand/oxy/utils"
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
	urlCache   map[*url.URL]string
	mu         sync.RWMutex
}

// NewStickySession creates a new StickySession
func NewStickySession(cookieName string) *StickySession {
	return &StickySession{
		cookieName: cookieName,
		hashCache:  make(map[string]string),
		urlCache:   make(map[*url.URL]string),
	}
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

	serverURLNoUser := utils.CopyURL(backend)
	serverURLNoUser.User = nil

	cookie := &http.Cookie{
		Name:     s.cookieName,
		Value:    hash(serverURLNoUser.String()),
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
	case strings.Contains(needle, "://"):
		// Honour old cookies which have URLs instead of hash
		needleURL, err := url.Parse(needle)
		if err != nil {
			return false, nil
		}
		for _, serverURL := range haystack {
			if sameURL(needleURL, serverURL) {
				return true, serverURL
			}
		}
	default:
		var h string
		var urlStr string
		var found bool

		for _, serverURL := range haystack {
			s.mu.RLock() // Lock in read mode

			if urlStr, found = s.urlCache[serverURL]; !found {
				// If we get here the url cache is not populated for this serverURL

				// We are going to modify url cache so we release the read lock
				s.mu.RUnlock()

				// Copy serverURL and remove user info that we don't want in the
				// needle/haystack comparison
				serverURLNoUser := utils.CopyURL(serverURL)
				serverURLNoUser.User = nil

				// Lock in write mode
				s.mu.Lock()

				// Truncate the url cache if the number of entries is larger than the haystack
				if len(s.urlCache) > len(haystack) {
					s.urlCache = make(map[*url.URL]string)
				}

				// Add the url string to the cache
				s.urlCache[serverURL] = serverURLNoUser.String()

				// Release the write lock
				s.mu.Unlock()

				urlStr = s.urlCache[serverURL]

				// Re-acquire read lock
				s.mu.RLock()
			}

			if h, found = s.hashCache[urlStr]; !found {
				// If we get here the hash cache is not populated for this serverURL

				// We are going to modify hash cache so we release the read lock
				s.mu.RUnlock()

				h = hash(urlStr)

				// Lock in write mode
				s.mu.Lock()

				// Truncate the hash cache if the number of entries is larger than the haystack
				if len(s.hashCache) > len(haystack) {
					s.hashCache = make(map[string]string)
				}

				// Add the hash string to the cache
				s.hashCache[urlStr] = h

				// Relase the write lock
				s.mu.Unlock()
			} else {
				// Release the read lock
				s.mu.RUnlock()
			}

			if needle == h {
				return true, serverURL
			}
		}
	}

	return false, nil
}

func hash(input string) string {
	return fmt.Sprintf("%x", fnv1a.HashString64(input))
}

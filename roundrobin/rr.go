// Package roundrobin implements dynamic weighted round robin load balancer http handler
package roundrobin

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"

	"github.com/vulcand/oxy/v2/utils"
)

// RoundRobin implements dynamic weighted round-robin load balancer http handler.
type RoundRobin struct {
	mutex      *sync.Mutex
	next       http.Handler
	errHandler utils.ErrorHandler
	// Current index (starts from -1)
	index                  int
	servers                []Server
	currentWeight          int
	stickySession          *StickySession
	requestRewriteListener RequestRewriteListener

	verbose bool
	log     utils.Logger
}

// New created a new RoundRobin.
func New(next http.Handler, opts ...LBOption) (*RoundRobin, error) {
	rr := &RoundRobin{
		next:          next,
		index:         -1,
		mutex:         &sync.Mutex{},
		servers:       []Server{},
		stickySession: nil,

		log: &utils.NoopLogger{},
	}
	for _, o := range opts {
		if err := o(rr); err != nil {
			return nil, err
		}
	}
	if rr.errHandler == nil {
		rr.errHandler = utils.DefaultHandler
	}
	return rr, nil
}

// Next returns the next handler.
func (r *RoundRobin) Next() http.Handler {
	return r.next
}

func (r *RoundRobin) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r.verbose {
		dump := utils.DumpHTTPRequest(req)
		r.log.Debug("vulcand/oxy/roundrobin/rr: begin ServeHttp on request: %s", dump)
		defer r.log.Debug("vulcand/oxy/roundrobin/rr: completed ServeHttp on request: %s", dump)
	}

	// make shallow copy of request before chaning anything to avoid side effects
	newReq := *req
	stuck := false
	if r.stickySession != nil {
		cookieURL, present, err := r.stickySession.GetBackend(&newReq, r.Servers())
		if err != nil {
			r.log.Warn("vulcand/oxy/roundrobin/rr: error using server from cookie: %v", err)
		}

		if present {
			newReq.URL = cookieURL
			stuck = true
		}
	}

	if !stuck {
		uri, err := r.NextServer(w, req, &newReq)
		if err != nil {
			r.errHandler.ServeHTTP(w, req, err)
			return
		}

		if r.stickySession != nil {
			r.stickySession.StickBackend(uri, w)
		}
		newReq.URL = uri
	}

	if r.verbose {
		// log which backend URL we're sending this request to
		dump := utils.DumpHTTPRequest(req)
		r.log.Debug("vulcand/oxy/roundrobin/rr: Forwarding this request to URL (%s): %s", newReq.URL, dump)
	}

	// Emit event to a listener if one exists
	if r.requestRewriteListener != nil {
		r.requestRewriteListener(req, &newReq)
	}

	r.next.ServeHTTP(w, &newReq)
}

// NextServer gets the next server.
func (r *RoundRobin) NextServer(w http.ResponseWriter, req *http.Request, neq *http.Request) (*url.URL, error) {
	// Use extension balance server, if extension return multiple servers, choose anyone.
	if ss := Strategy().Next(w, req, neq, r.servers); len(ss) > 0 {
		srv := ss[rand.Intn(len(ss))]
		return utils.CopyURL(srv.URL()), nil
	}
	srv, err := r.nextServer(w, req)
	if err != nil {
		return nil, err
	}
	return utils.CopyURL(srv.URL()), nil
}

func (r *RoundRobin) nextServer(w http.ResponseWriter, req *http.Request) (Server, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(r.servers) == 0 {
		return nil, fmt.Errorf("no servers in the pool")
	}

	// The algo below may look messy, but is actually very simple
	// it calculates the GCD  and subtracts it on every iteration, what interleaves servers
	// and allows us not to build an iterator every time we readjust weights

	// GCD across all enabled servers
	gcd := r.weightGcd()
	// Maximum weight across all enabled servers
	max := r.maxWeight()

	for {
		r.index = (r.index + 1) % len(r.servers)
		if r.index == 0 {
			r.currentWeight -= gcd
			if r.currentWeight <= 0 {
				r.currentWeight = max
				if r.currentWeight == 0 {
					return nil, fmt.Errorf("all servers have 0 weight")
				}
			}
		}
		srv := r.servers[r.index]
		if srv.Weight() >= r.currentWeight {
			return srv, nil
		}
	}
}

// RemoveServer remove a server.
func (r *RoundRobin) RemoveServer(u *url.URL) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	e, index := r.findServerByURL(u)
	if e == nil {
		return fmt.Errorf("server not found")
	}
	r.servers = append(r.servers[:index], r.servers[index+1:]...)
	r.resetState()
	return nil
}

// Servers gets servers URL.
func (r *RoundRobin) Servers() []*url.URL {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	out := make([]*url.URL, len(r.servers))
	for i, srv := range r.servers {
		out[i] = srv.URL()
	}
	return out
}

// ServerWeight gets the server weight.
func (r *RoundRobin) ServerWeight(u *url.URL) (int, bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if s, _ := r.findServerByURL(u); s != nil {
		return s.Weight(), true
	}
	return -1, false
}

// UpsertServer In case if server is already present in the load balancer, returns error.
func (r *RoundRobin) UpsertServer(u *url.URL, options ...ServerOption) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if u == nil {
		return fmt.Errorf("server URL can't be nil")
	}

	if s, _ := r.findServerByURL(u); s != nil {
		for _, o := range options {
			if err := o(s); err != nil {
				return err
			}
		}
		r.resetState()
		return nil
	}

	srv := &server{url: utils.CopyURL(u)}
	for _, o := range options {
		if err := o(srv); err != nil {
			return err
		}
	}

	if srv.weight == 0 {
		srv.weight = defaultWeight
	}

	r.servers = append(r.servers, srv)
	r.resetState()
	return nil
}

func (r *RoundRobin) resetIterator() {
	r.index = -1
	r.currentWeight = 0
}

func (r *RoundRobin) resetState() {
	r.resetIterator()
}

func (r *RoundRobin) findServerByURL(u *url.URL) (Server, int) {
	if len(r.servers) == 0 {
		return nil, -1
	}
	for i, s := range r.servers {
		if sameURL(u, s.URL()) {
			return s, i
		}
	}
	return nil, -1
}

func (r *RoundRobin) maxWeight() int {
	max := -1
	for _, s := range r.servers {
		if s.Weight() > max {
			max = s.Weight()
		}
	}
	return max
}

func (r *RoundRobin) weightGcd() int {
	divisor := -1
	for _, s := range r.servers {
		if divisor == -1 {
			divisor = s.Weight()
		} else {
			divisor = gcd(divisor, s.Weight())
		}
	}
	return divisor
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// Set additional parameters for the server can be supplied when adding server.
type server struct {
	url *url.URL
	// Relative weight for the enpoint to other enpoints in the load balancer
	weight int
}

func (that *server) URL() *url.URL {
	return that.url
}

func (that *server) Weight() int {
	return that.weight
}

func (that *server) Set(weight int) {
	that.weight = weight
}

var defaultWeight = 1

// SetDefaultWeight sets the default server weight.
func SetDefaultWeight(weight int) error {
	if weight < 0 {
		return fmt.Errorf("default weight should be >= 0")
	}
	defaultWeight = weight
	return nil
}

func sameURL(a, b *url.URL) bool {
	return a.Path == b.Path && a.Host == b.Host && a.Scheme == b.Scheme
}

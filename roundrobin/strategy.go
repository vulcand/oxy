package roundrobin

import (
	"net/http"
	"net/url"
	"sort"
)

func init() {
	var _ LBStrategy = new(CompositeStrategy)
}
func Strategy() LBStrategy {
	return strategies
}

func Provide(lbs LBStrategy) {
	strategies.Add(lbs)
}

var strategies = new(CompositeStrategy)

type Server interface {

	// URL server url.
	URL() *url.URL

	// Weight Relative weight for the endpoint to other endpoints in the load balancer.
	Weight() int

	// Set the weight.
	Set(weight int)
}

type LBStrategy interface {

	// Name is the strategy name.
	Name() string

	// Priority more than has more priority.
	Priority() int

	// Next servers
	// Load balancer extension for custom rules filter.
	Next(w http.ResponseWriter, req *http.Request, neq *http.Request, servers []Server) []Server

	// Strip filter the server URL
	Strip(w http.ResponseWriter, req *http.Request, neq *http.Request, uri *url.URL) *url.URL
}

type CompositeStrategy struct {
	strategies []LBStrategy
}

func (that *CompositeStrategy) Add(lbs LBStrategy) *CompositeStrategy {
	that.strategies = append(that.strategies, lbs)
	sort.Slice(that.strategies, func(i, j int) bool { return that.strategies[i].Priority() < that.strategies[j].Priority() })
	return that
}

func (that *CompositeStrategy) Name() string {
	return "composite"
}

func (that *CompositeStrategy) Priority() int {
	return 0
}

func (that *CompositeStrategy) Next(w http.ResponseWriter, req *http.Request, neq *http.Request, servers []Server) []Server {
	for _, strategy := range that.strategies {
		servers = strategy.Next(w, req, neq, servers)
	}
	return servers
}

func (that *CompositeStrategy) Strip(w http.ResponseWriter, req *http.Request, neq *http.Request, uri *url.URL) *url.URL {
	for _, strategy := range that.strategies {
		uri = strategy.Strip(w, req, neq, uri)
	}
	return uri
}

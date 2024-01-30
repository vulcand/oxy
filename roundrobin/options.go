package roundrobin

import (
	"fmt"
	"time"

	"github.com/vulcand/oxy/v2/utils"
)

// RebalancerOption represents an option you can pass to NewRebalancer.
type RebalancerOption func(*Rebalancer) error

// RebalancerBackoff sets a beck off duration.
func RebalancerBackoff(d time.Duration) RebalancerOption {
	return func(r *Rebalancer) error {
		r.backoffDuration = d
		return nil
	}
}

// RebalancerMeter sets a Meter builder function.
func RebalancerMeter(newMeter NewMeterFn) RebalancerOption {
	return func(r *Rebalancer) error {
		r.newMeter = newMeter
		return nil
	}
}

// RebalancerErrorHandler is a functional argument that sets error handler of the server.
func RebalancerErrorHandler(h utils.ErrorHandler) RebalancerOption {
	return func(r *Rebalancer) error {
		r.errHandler = h
		return nil
	}
}

// RebalancerStickySession sets a sticky session.
func RebalancerStickySession(stickySession *StickySession) RebalancerOption {
	return func(r *Rebalancer) error {
		r.stickySession = stickySession
		return nil
	}
}

// RebalancerRequestRewriteListener is a functional argument that sets error handler of the server.
func RebalancerRequestRewriteListener(rrl RequestRewriteListener) RebalancerOption {
	return func(r *Rebalancer) error {
		r.requestRewriteListener = rrl
		return nil
	}
}

// RebalancerLogger defines the logger used by Rebalancer.
func RebalancerLogger(l utils.Logger) RebalancerOption {
	return func(rb *Rebalancer) error {
		rb.log = l
		return nil
	}
}

// RebalancerDebug additional debug information.
func RebalancerDebug(debug bool) RebalancerOption {
	return func(rb *Rebalancer) error {
		rb.debug = debug
		return nil
	}
}

// ServerOption provides various options for server, e.g. weight.
type ServerOption func(s Server) error

// Weight is an optional functional argument that sets weight of the server.
func Weight(w int) ServerOption {
	return func(s Server) error {
		if w < 0 {
			return fmt.Errorf("Weight should be >= 0 ")
		}
		s.Set(w)
		return nil
	}
}

// LBOption provides options for load balancer.
type LBOption func(*RoundRobin) error

// ErrorHandler is a functional argument that sets error handler of the server.
func ErrorHandler(h utils.ErrorHandler) LBOption {
	return func(s *RoundRobin) error {
		s.errHandler = h
		return nil
	}
}

// EnableStickySession enable sticky session.
func EnableStickySession(stickySession *StickySession) LBOption {
	return func(s *RoundRobin) error {
		s.stickySession = stickySession
		return nil
	}
}

// RoundRobinRequestRewriteListener is a functional argument that sets error handler of the server.
func RoundRobinRequestRewriteListener(rrl RequestRewriteListener) LBOption {
	return func(s *RoundRobin) error {
		s.requestRewriteListener = rrl
		return nil
	}
}

// Logger defines the logger the RoundRobin will use.
func Logger(l utils.Logger) LBOption {
	return func(r *RoundRobin) error {
		r.log = l
		return nil
	}
}

// Verbose additional debug information.
func Verbose(verbose bool) LBOption {
	return func(r *RoundRobin) error {
		r.verbose = verbose
		return nil
	}
}

// Package cbreaker implements circuit breaker similar to  https://github.com/Netflix/Hystrix/wiki/How-it-Works
//
// Vulcan circuit breaker watches the error condtion to match
// after which it activates the fallback scenario, e.g. returns the response code
// or redirects the request to another location
//
// Circuit breakers start in the Standby state first, observing responses and watching location metrics.
//
// Once the Circuit breaker condition is met, it enters the "Tripped" state, where it activates fallback scenario
// for all requests during the FallbackDuration time period and reset the stats for the location.
//
// After FallbackDuration time period passes, Circuit breaker enters "Recovering" state, during that state it will
// start passing some traffic back to the endpoints, increasing the amount of passed requests using linear function:
//
//	allowedRequestsRatio = 0.5 * (Now() - StartRecovery())/RecoveryDuration
//
// Two scenarios are possible in the "Recovering" state:
// 1. Condition matches again, this will reset the state to "Tripped" and reset the timer.
// 2. Condition does not match, circuit breaker enters "Standby" state
//
// It is possible to define actions (e.g. webhooks) of transitions between states:
//
// * OnTripped action is called on transition (Standby -> Tripped)
// * OnStandby action is called on transition (Recovering -> Standby)
package cbreaker

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
	"github.com/vulcand/oxy/v2/memmetrics"
	"github.com/vulcand/oxy/v2/utils"
)

// CircuitBreaker is http.Handler that implements circuit breaker pattern.
type CircuitBreaker struct {
	m       *sync.RWMutex
	metrics *memmetrics.RTMetrics

	condition hpredicate

	fallbackDuration time.Duration
	recoveryDuration time.Duration

	onTripped SideEffect
	onStandby SideEffect

	state cbState
	until clock.Time

	rc *ratioController

	checkPeriod time.Duration
	lastCheck   clock.Time

	fallback http.Handler
	next     http.Handler

	verbose bool
	log     utils.Logger
}

// New creates a new CircuitBreaker middleware.
func New(next http.Handler, expression string, options ...Option) (*CircuitBreaker, error) {
	cb := &CircuitBreaker{
		m:    &sync.RWMutex{},
		next: next,
		// Default values. Might be overwritten by options below.
		checkPeriod:      defaultCheckPeriod,
		fallbackDuration: defaultFallbackDuration,
		recoveryDuration: defaultRecoveryDuration,
		fallback:         defaultFallback,
		log:              &utils.NoopLogger{},
	}

	for _, s := range options {
		if err := s(cb); err != nil {
			return nil, err
		}
	}

	condition, err := parseExpression(expression)
	if err != nil {
		return nil, err
	}
	cb.condition = condition

	mt, err := memmetrics.NewRTMetrics()
	if err != nil {
		return nil, err
	}
	cb.metrics = mt

	return cb, nil
}

func (c *CircuitBreaker) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if c.verbose {
		dump := utils.DumpHTTPRequest(req)
		c.log.Debug("vulcand/oxy/circuitbreaker: begin ServeHttp on request: %s", dump)
		defer c.log.Debug("vulcand/oxy/circuitbreaker: completed ServeHttp on request: %s", dump)
	}

	if c.activateFallback(w, req) {
		c.fallback.ServeHTTP(w, req)
		return
	}

	c.serve(w, req)
}

// Fallback sets the fallback handler to be called by circuit breaker handler.
func (c *CircuitBreaker) Fallback(f http.Handler) {
	c.fallback = f
}

// Wrap sets the next handler to be called by circuit breaker handler.
func (c *CircuitBreaker) Wrap(next http.Handler) {
	c.next = next
}

// updateState updates internal state and returns true if fallback should be used and false otherwise.
func (c *CircuitBreaker) activateFallback(_ http.ResponseWriter, _ *http.Request) bool {
	// Quick check with read locks optimized for normal operation use-case
	if c.isStandby() {
		return false
	}
	// Circuit breaker is in tripped or recovering state
	c.m.Lock()
	defer c.m.Unlock()

	c.log.Warn("%v is in error state", c)

	switch c.state {
	case stateStandby:
		// someone else has set it to standby just now
		return false
	case stateTripped:
		if clock.Now().UTC().Before(c.until) {
			return true
		}
		// We have been in active state enough, enter recovering state
		c.setRecovering()
		fallthrough
	case stateRecovering:
		// We have been in recovering state enough, enter standby and allow request
		if clock.Now().UTC().After(c.until) {
			c.setState(stateStandby, clock.Now().UTC())
			return false
		}
		// ratio controller allows this request
		if c.rc.allowRequest() {
			return false
		}
		return true
	}
	return false
}

func (c *CircuitBreaker) serve(w http.ResponseWriter, req *http.Request) {
	start := clock.Now().UTC()
	p := utils.NewProxyWriterWithLogger(w, c.log)

	c.next.ServeHTTP(p, req)

	latency := clock.Now().UTC().Sub(start)
	c.metrics.Record(p.StatusCode(), latency)

	// Note that this call is less expensive than it looks -- checkCondition only performs the real check
	// periodically. Because of that we can afford to call it here on every single response.
	c.checkAndSet()
}

func (c *CircuitBreaker) isStandby() bool {
	c.m.RLock()
	defer c.m.RUnlock()
	return c.state == stateStandby
}

// String returns log-friendly representation of the circuit breaker state.
func (c *CircuitBreaker) String() string {
	switch c.state {
	case stateTripped, stateRecovering:
		return fmt.Sprintf("CircuitBreaker(state=%v, until=%v)", c.state, c.until)
	default:
		return fmt.Sprintf("CircuitBreaker(state=%v)", c.state)
	}
}

// exec executes side effect.
func (c *CircuitBreaker) exec(s SideEffect) {
	if s == nil {
		return
	}
	go func() {
		if err := s.Exec(); err != nil {
			c.log.Error("%v side effect failure: %v", c, err)
		}
	}()
}

func (c *CircuitBreaker) setState(state cbState, until time.Time) {
	c.log.Debug("%v setting state to %v, until %v", c, state, until)
	c.state = state
	c.until = until
	switch state {
	case stateTripped:
		c.exec(c.onTripped)
	case stateStandby:
		c.exec(c.onStandby)
	}
}

func (c *CircuitBreaker) timeToCheck() bool {
	c.m.RLock()
	defer c.m.RUnlock()
	return clock.Now().UTC().After(c.lastCheck)
}

// Checks if tripping condition matches and sets circuit breaker to the tripped state.
func (c *CircuitBreaker) checkAndSet() {
	if !c.timeToCheck() {
		return
	}

	c.m.Lock()
	defer c.m.Unlock()

	// Other goroutine could have updated the lastCheck variable before we grabbed mutex
	if clock.Now().UTC().Before(c.lastCheck) {
		return
	}
	c.lastCheck = clock.Now().UTC().Add(c.checkPeriod)

	if c.state == stateTripped {
		c.log.Debug("%v skip set tripped", c)
		return
	}

	if !c.condition(c) {
		return
	}

	c.setState(stateTripped, clock.Now().UTC().Add(c.fallbackDuration))
	c.metrics.Reset()
}

func (c *CircuitBreaker) setRecovering() {
	c.setState(stateRecovering, clock.Now().UTC().Add(c.recoveryDuration))
	c.rc = newRatioController(c.recoveryDuration, c.log)
}

// cbState is the state of the circuit breaker.
type cbState int

func (s cbState) String() string {
	switch s {
	case stateStandby:
		return "standby"
	case stateTripped:
		return "tripped"
	case stateRecovering:
		return "recovering"
	}
	return "undefined"
}

const (
	// CircuitBreaker is passing all requests and watching stats.
	stateStandby = iota
	// CircuitBreaker activates fallback scenario for all requests.
	stateTripped
	// CircuitBreaker passes some requests to go through, rejecting others.
	stateRecovering
)

const (
	defaultFallbackDuration = 10 * clock.Second
	defaultRecoveryDuration = 10 * clock.Second
	defaultCheckPeriod      = 100 * clock.Millisecond
)

var defaultFallback = &fallback{}

type fallback struct{}

func (f *fallback) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(http.StatusText(http.StatusServiceUnavailable)))
}

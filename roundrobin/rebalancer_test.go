package roundrobin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/forward"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
	"github.com/vulcand/oxy/v2/testutils"
)

func TestRebalancer_normalOperation(t *testing.T) {
	a := testutils.NewResponder(t, "a")
	testutils.NewResponder(t, "b")

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	rb, err := NewRebalancer(lb)
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.MustParseRequestURI(a.URL))
	require.NoError(t, err)

	assert.Equal(t, a.URL, rb.Servers()[0].String())

	proxy := httptest.NewServer(rb)
	t.Cleanup(proxy.Close)

	assert.Equal(t, []string{"a", "a", "a"}, seq(t, proxy.URL, 3))
}

func TestRebalancer_noServers(t *testing.T) {
	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	rb, err := NewRebalancer(lb)
	require.NoError(t, err)

	proxy := httptest.NewServer(rb)
	t.Cleanup(proxy.Close)

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, re.StatusCode)
}

func TestRebalancer_removeServer(t *testing.T) {
	a := testutils.NewResponder(t, "a")
	b := testutils.NewResponder(t, "b")

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	rb, err := NewRebalancer(lb)
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.MustParseRequestURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(b.URL))
	require.NoError(t, err)

	proxy := httptest.NewServer(rb)
	t.Cleanup(proxy.Close)

	assert.Equal(t, []string{"a", "b", "a"}, seq(t, proxy.URL, 3))
	require.NoError(t, rb.RemoveServer(testutils.MustParseRequestURI(a.URL)))
	assert.Equal(t, []string{"b", "b", "b"}, seq(t, proxy.URL, 3))
}

// Test scenario when one server goes down after what it recovers.
func TestRebalancer_recovery(t *testing.T) {
	a := testutils.NewResponder(t, "a")
	b := testutils.NewResponder(t, "b")

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	newMeter := func() (Meter, error) {
		return &testMeter{}, nil
	}

	testutils.FreezeTime(t)

	rb, err := NewRebalancer(lb, RebalancerMeter(newMeter))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.MustParseRequestURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(b.URL))
	require.NoError(t, err)

	rb.servers[0].meter.(*testMeter).rating = 0.3

	proxy := httptest.NewServer(rb)
	t.Cleanup(proxy.Close)

	for range 6 {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.Advance(rb.backoffDuration + clock.Second)
	}

	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[1].curWeight)

	assert.Equal(t, 1, lb.servers[0].weight)
	assert.Equal(t, FSMMaxWeight, lb.servers[1].weight)

	// server a is now recovering, the weights should go back to the original state
	rb.servers[0].meter.(*testMeter).rating = 0

	for range 6 {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.Advance(rb.backoffDuration + clock.Second)
	}

	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, 1, rb.servers[1].curWeight)

	// Make sure we have applied the weights to the inner load balancer
	assert.Equal(t, 1, lb.servers[0].weight)
	assert.Equal(t, 1, lb.servers[1].weight)
}

// Test scenario when increaing the weight on good endpoints made it worse.
func TestRebalancer_cascading(t *testing.T) {
	a := testutils.NewResponder(t, "a")
	b := testutils.NewResponder(t, "b")
	d := testutils.NewResponder(t, "d")

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	newMeter := func() (Meter, error) {
		return &testMeter{}, nil
	}

	testutils.FreezeTime(t)

	rb, err := NewRebalancer(lb, RebalancerMeter(newMeter))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.MustParseRequestURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(b.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(d.URL))
	require.NoError(t, err)

	rb.servers[0].meter.(*testMeter).rating = 0.3

	proxy := httptest.NewServer(rb)
	t.Cleanup(proxy.Close)

	for range 6 {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.Advance(rb.backoffDuration + clock.Second)
	}

	// We have increased the load, and the situation became worse as the other servers started failing
	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[1].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[2].curWeight)

	// server a is now recovering, the weights should go back to the original state
	rb.servers[0].meter.(*testMeter).rating = 0.3
	rb.servers[1].meter.(*testMeter).rating = 0.2
	rb.servers[2].meter.(*testMeter).rating = 0.2

	for range 6 {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.Advance(rb.backoffDuration + clock.Second)
	}

	// the algo reverted it back
	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, 1, rb.servers[1].curWeight)
	assert.Equal(t, 1, rb.servers[2].curWeight)
}

// Test scenario when all servers started failing.
func TestRebalancer_allBad(t *testing.T) {
	a := testutils.NewResponder(t, "a")
	b := testutils.NewResponder(t, "b")
	d := testutils.NewResponder(t, "d")

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	newMeter := func() (Meter, error) {
		return &testMeter{}, nil
	}

	testutils.FreezeTime(t)

	rb, err := NewRebalancer(lb, RebalancerMeter(newMeter))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.MustParseRequestURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(b.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(d.URL))
	require.NoError(t, err)

	rb.servers[0].meter.(*testMeter).rating = 0.12
	rb.servers[1].meter.(*testMeter).rating = 0.13
	rb.servers[2].meter.(*testMeter).rating = 0.11

	proxy := httptest.NewServer(rb)
	t.Cleanup(proxy.Close)

	for range 6 {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.Advance(rb.backoffDuration + clock.Second)
	}

	// load balancer does nothing
	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, 1, rb.servers[1].curWeight)
	assert.Equal(t, 1, rb.servers[2].curWeight)
}

// Removing the server resets the state.
func TestRebalancer_reset(t *testing.T) {
	a := testutils.NewResponder(t, "a")
	b := testutils.NewResponder(t, "b")
	d := testutils.NewResponder(t, "d")

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	newMeter := func() (Meter, error) {
		return &testMeter{}, nil
	}

	testutils.FreezeTime(t)

	rb, err := NewRebalancer(lb, RebalancerMeter(newMeter))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.MustParseRequestURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(b.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(d.URL))
	require.NoError(t, err)

	rb.servers[0].meter.(*testMeter).rating = 0.3
	rb.servers[1].meter.(*testMeter).rating = 0
	rb.servers[2].meter.(*testMeter).rating = 0

	proxy := httptest.NewServer(rb)
	t.Cleanup(proxy.Close)

	for range 6 {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.Advance(rb.backoffDuration + clock.Second)
	}

	// load balancer changed weights
	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[1].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[2].curWeight)

	// Removing servers has reset the state
	err = rb.RemoveServer(testutils.MustParseRequestURI(d.URL))
	require.NoError(t, err)

	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, 1, rb.servers[1].curWeight)
}

func TestRebalancer_requestRewriteListenerLive(t *testing.T) {
	a := testutils.NewResponder(t, "a")
	b := testutils.NewResponder(t, "b")

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	testutils.FreezeTime(t)

	rb, err := NewRebalancer(lb, RebalancerBackoff(clock.Millisecond))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.MustParseRequestURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(b.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI("http://localhost:62345"))
	require.NoError(t, err)

	proxy := httptest.NewServer(rb)
	t.Cleanup(proxy.Close)

	for i := range 1000 {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		if i%10 == 0 {
			clock.Advance(rb.backoffDuration + clock.Second)
		}
	}

	// load balancer changed weights
	assert.Equal(t, FSMMaxWeight, rb.servers[0].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[1].curWeight)
	assert.Equal(t, 1, rb.servers[2].curWeight)
}

func TestRebalancer_requestRewriteListener(t *testing.T) {
	testutils.NewResponder(t, "a")
	testutils.NewResponder(t, "b")

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	rb, err := NewRebalancer(lb,
		RebalancerRequestRewriteListener(func(_ *http.Request, _ *http.Request) {}))
	require.NoError(t, err)

	assert.NotNil(t, rb.requestRewriteListener)
}

func TestRebalancer_stickySession(t *testing.T) {
	a := testutils.NewResponder(t, "a")
	b := testutils.NewResponder(t, "b")
	x := testutils.NewResponder(t, "x")

	sticky := NewStickySession("test")
	require.NotNil(t, sticky)

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	rb, err := NewRebalancer(lb, RebalancerStickySession(sticky))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.MustParseRequestURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(b.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.MustParseRequestURI(x.URL))
	require.NoError(t, err)

	proxy := httptest.NewServer(rb)
	t.Cleanup(proxy.Close)

	for range 10 {
		req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
		require.NoError(t, err)
		req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)

		require.NoError(t, err)
		assert.Equal(t, "a", string(body))
	}

	require.NoError(t, rb.RemoveServer(testutils.MustParseRequestURI(a.URL)))
	assert.Equal(t, []string{"b", "x", "b"}, seq(t, proxy.URL, 3))

	require.NoError(t, rb.RemoveServer(testutils.MustParseRequestURI(b.URL)))
	assert.Equal(t, []string{"x", "x", "x"}, seq(t, proxy.URL, 3))
}

type testMeter struct {
	rating   float64
	notReady bool
}

func (tm *testMeter) Rating() float64 {
	return tm.rating
}

func (tm *testMeter) Record(int, time.Duration) {
}

func (tm *testMeter) IsReady() bool {
	return !tm.notReady
}

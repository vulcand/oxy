package roundrobin

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/testutils"
)

func TestRebalancerNormalOperation(t *testing.T) {
	a, b := testutils.NewResponder("a"), testutils.NewResponder("b")
	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	lb, err := New(fwd)
	require.NoError(t, err)

	rb, err := NewRebalancer(lb)
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)

	assert.Equal(t, a.URL, rb.Servers()[0].String())

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	assert.Equal(t, []string{"a", "a", "a"}, seq(t, proxy.URL, 3))
}

func TestRebalancerNoServers(t *testing.T) {
	fwd, err := forward.New()
	require.NoError(t, err)

	lb, err := New(fwd)
	require.NoError(t, err)

	rb, err := NewRebalancer(lb)
	require.NoError(t, err)

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, re.StatusCode)
}

func TestRebalancerRemoveServer(t *testing.T) {
	a, b := testutils.NewResponder("a"), testutils.NewResponder("b")
	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	lb, err := New(fwd)
	require.NoError(t, err)

	rb, err := NewRebalancer(lb)
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	assert.Equal(t, []string{"a", "b", "a"}, seq(t, proxy.URL, 3))
	require.NoError(t, rb.RemoveServer(testutils.ParseURI(a.URL)))
	assert.Equal(t, []string{"b", "b", "b"}, seq(t, proxy.URL, 3))
}

// Test scenario when one server goes down after what it recovers
func TestRebalancerRecovery(t *testing.T) {
	a, b := testutils.NewResponder("a"), testutils.NewResponder("b")
	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	lb, err := New(fwd)
	require.NoError(t, err)

	newMeter := func() (Meter, error) {
		return &testMeter{}, nil
	}

	clock := testutils.GetClock()

	rb, err := NewRebalancer(lb, RebalancerMeter(newMeter), RebalancerClock(clock))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)

	rb.servers[0].meter.(*testMeter).rating = 0.3

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	for i := 0; i < 6; i++ {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.CurrentTime = clock.CurrentTime.Add(rb.backoffDuration + time.Second)
	}

	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[1].curWeight)

	assert.Equal(t, 1, lb.servers[0].weight)
	assert.Equal(t, FSMMaxWeight, lb.servers[1].weight)

	// server a is now recovering, the weights should go back to the original state
	rb.servers[0].meter.(*testMeter).rating = 0

	for i := 0; i < 6; i++ {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.CurrentTime = clock.CurrentTime.Add(rb.backoffDuration + time.Second)
	}

	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, 1, rb.servers[1].curWeight)

	// Make sure we have applied the weights to the inner load balancer
	assert.Equal(t, 1, lb.servers[0].weight)
	assert.Equal(t, 1, lb.servers[1].weight)
}

// Test scenario when increaing the weight on good endpoints made it worse
func TestRebalancerCascading(t *testing.T) {
	a, b, d := testutils.NewResponder("a"), testutils.NewResponder("b"), testutils.NewResponder("d")
	defer a.Close()
	defer b.Close()
	defer d.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	lb, err := New(fwd)
	require.NoError(t, err)

	newMeter := func() (Meter, error) {
		return &testMeter{}, nil
	}

	clock := testutils.GetClock()

	rb, err := NewRebalancer(lb, RebalancerMeter(newMeter), RebalancerClock(clock))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(d.URL))
	require.NoError(t, err)

	rb.servers[0].meter.(*testMeter).rating = 0.3

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	for i := 0; i < 6; i++ {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.CurrentTime = clock.CurrentTime.Add(rb.backoffDuration + time.Second)
	}

	// We have increased the load, and the situation became worse as the other servers started failing
	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[1].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[2].curWeight)

	// server a is now recovering, the weights should go back to the original state
	rb.servers[0].meter.(*testMeter).rating = 0.3
	rb.servers[1].meter.(*testMeter).rating = 0.2
	rb.servers[2].meter.(*testMeter).rating = 0.2

	for i := 0; i < 6; i++ {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.CurrentTime = clock.CurrentTime.Add(rb.backoffDuration + time.Second)
	}

	// the algo reverted it back
	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, 1, rb.servers[1].curWeight)
	assert.Equal(t, 1, rb.servers[2].curWeight)
}

// Test scenario when all servers started failing
func TestRebalancerAllBad(t *testing.T) {
	a, b, d := testutils.NewResponder("a"), testutils.NewResponder("b"), testutils.NewResponder("d")
	defer a.Close()
	defer b.Close()
	defer d.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	lb, err := New(fwd)
	require.NoError(t, err)

	newMeter := func() (Meter, error) {
		return &testMeter{}, nil
	}

	clock := testutils.GetClock()

	rb, err := NewRebalancer(lb, RebalancerMeter(newMeter), RebalancerClock(clock))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(d.URL))
	require.NoError(t, err)

	rb.servers[0].meter.(*testMeter).rating = 0.12
	rb.servers[1].meter.(*testMeter).rating = 0.13
	rb.servers[2].meter.(*testMeter).rating = 0.11

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	for i := 0; i < 6; i++ {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.CurrentTime = clock.CurrentTime.Add(rb.backoffDuration + time.Second)
	}

	// load balancer does nothing
	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, 1, rb.servers[1].curWeight)
	assert.Equal(t, 1, rb.servers[2].curWeight)
}

// Removing the server resets the state
func TestRebalancerReset(t *testing.T) {
	a, b, d := testutils.NewResponder("a"), testutils.NewResponder("b"), testutils.NewResponder("d")
	defer a.Close()
	defer b.Close()
	defer d.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	lb, err := New(fwd)
	require.NoError(t, err)

	newMeter := func() (Meter, error) {
		return &testMeter{}, nil
	}

	clock := testutils.GetClock()

	rb, err := NewRebalancer(lb, RebalancerMeter(newMeter), RebalancerClock(clock))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(d.URL))
	require.NoError(t, err)

	rb.servers[0].meter.(*testMeter).rating = 0.3
	rb.servers[1].meter.(*testMeter).rating = 0
	rb.servers[2].meter.(*testMeter).rating = 0

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	for i := 0; i < 6; i++ {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		clock.CurrentTime = clock.CurrentTime.Add(rb.backoffDuration + time.Second)
	}

	// load balancer changed weights
	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[1].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[2].curWeight)

	// Removing servers has reset the state
	err = rb.RemoveServer(testutils.ParseURI(d.URL))
	require.NoError(t, err)

	assert.Equal(t, 1, rb.servers[0].curWeight)
	assert.Equal(t, 1, rb.servers[1].curWeight)
}

func TestRebalancerRequestRewriteListenerLive(t *testing.T) {
	a, b := testutils.NewResponder("a"), testutils.NewResponder("b")
	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	lb, err := New(fwd)
	require.NoError(t, err)

	clock := testutils.GetClock()

	rb, err := NewRebalancer(lb, RebalancerBackoff(time.Millisecond), RebalancerClock(clock))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI("http://localhost:62345"))
	require.NoError(t, err)

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	for i := 0; i < 1000; i++ {
		_, _, err = testutils.Get(proxy.URL)
		require.NoError(t, err)
		if i%10 == 0 {
			clock.CurrentTime = clock.CurrentTime.Add(rb.backoffDuration + time.Second)
		}
	}

	// load balancer changed weights
	assert.Equal(t, FSMMaxWeight, rb.servers[0].curWeight)
	assert.Equal(t, FSMMaxWeight, rb.servers[1].curWeight)
	assert.Equal(t, 1, rb.servers[2].curWeight)
}

func TestRebalancerRequestRewriteListener(t *testing.T) {
	a, b := testutils.NewResponder("a"), testutils.NewResponder("b")
	defer a.Close()
	defer b.Close()

	fwd, err := forward.New()
	require.NoError(t, err)

	lb, err := New(fwd)
	require.NoError(t, err)

	rb, err := NewRebalancer(lb,
		RebalancerRequestRewriteListener(func(oldReq *http.Request, newReq *http.Request) {
		}))
	require.NoError(t, err)

	assert.NotNil(t, rb.requestRewriteListener)
}

func TestRebalancerStickySession(t *testing.T) {
	a, b, x := testutils.NewResponder("a"), testutils.NewResponder("b"), testutils.NewResponder("x")
	defer a.Close()
	defer b.Close()
	defer x.Close()

	sticky := NewStickySession("test")
	require.NotNil(t, sticky)

	fwd, err := forward.New()
	require.NoError(t, err)

	lb, err := New(fwd)
	require.NoError(t, err)

	rb, err := NewRebalancer(lb, RebalancerStickySession(sticky))
	require.NoError(t, err)

	err = rb.UpsertServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(b.URL))
	require.NoError(t, err)
	err = rb.UpsertServer(testutils.ParseURI(x.URL))
	require.NoError(t, err)

	proxy := httptest.NewServer(rb)
	defer proxy.Close()

	for i := 0; i < 10; i++ {
		req, err := http.NewRequest(http.MethodGet, proxy.URL, nil)
		require.NoError(t, err)
		req.AddCookie(&http.Cookie{Name: "test", Value: a.URL})

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)

		require.NoError(t, err)
		assert.Equal(t, "a", string(body))
	}

	require.NoError(t, rb.RemoveServer(testutils.ParseURI(a.URL)))
	assert.Equal(t, []string{"b", "x", "b"}, seq(t, proxy.URL, 3))

	require.NoError(t, rb.RemoveServer(testutils.ParseURI(b.URL)))
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

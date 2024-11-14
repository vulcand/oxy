package forward

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	gorillawebsocket "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
	"github.com/vulcand/oxy/v2/testutils"
	"golang.org/x/net/websocket"
)

func TestWebSocketTCPClose(t *testing.T) {
	f := New(true)

	errChan := make(chan error, 1)
	upgrader := gorillawebsocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func(c *gorillawebsocket.Conn) { _ = c.Close() }(c)
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				errChan <- err
				break
			}
		}
	}))
	t.Cleanup(srv.Close)

	proxy := createProxyWithForwarder(f, srv.URL)

	proxyAddr := proxy.Listener.Addr().String()
	_, conn, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
	).open()
	require.NoError(t, err)
	_ = conn.Close()

	serverErr := <-errChan

	wsErr := &gorillawebsocket.CloseError{}
	assert.ErrorAs(t, serverErr, &wsErr) //nolint:testifylint // The code is analyzed after this assertion.
	assert.Equal(t, 1006, wsErr.Code)
}

func TestWebSocketPingPong(t *testing.T) {
	f := New(true)

	upgrader := gorillawebsocket.Upgrader{
		HandshakeTimeout: 10 * clock.Second,
		CheckOrigin: func(*http.Request) bool {
			return true
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		ws, err := upgrader.Upgrade(writer, request, nil)
		require.NoError(t, err)

		ws.SetPingHandler(func(appData string) error {
			_ = ws.WriteMessage(gorillawebsocket.PongMessage, []byte(appData+"Pong"))
			return nil
		})

		_, _, _ = ws.ReadMessage()
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	proxy := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		f.ServeHTTP(w, req)
	})
	t.Cleanup(proxy.Close)

	serverAddr := proxy.Listener.Addr().String()

	headers := http.Header{}
	webSocketURL := "ws://" + serverAddr + "/ws"
	headers.Add("Origin", webSocketURL)

	conn, resp, err := gorillawebsocket.DefaultDialer.Dial(webSocketURL, headers)
	require.NoError(t, err, "Error during Dial with response: %+v", resp)
	defer conn.Close()

	goodErr := fmt.Errorf("signal: %s", "Good data")
	badErr := fmt.Errorf("signal: %s", "Bad data")
	conn.SetPongHandler(func(data string) error {
		if data == "PingPong" {
			return goodErr
		}
		return badErr
	})

	_ = conn.WriteControl(gorillawebsocket.PingMessage, []byte("Ping"), clock.Now().Add(clock.Second))
	_, _, err = conn.ReadMessage()

	if !errors.Is(err, goodErr) {
		require.NoError(t, err)
	}
}

func TestWebSocketEcho(t *testing.T) {
	f := New(true)

	mux := http.NewServeMux()
	mux.Handle("/ws", websocket.Handler(func(conn *websocket.Conn) {
		msg := make([]byte, 4)
		_, _ = conn.Read(msg)
		t.Log(string(msg))
		_, _ = conn.Write(msg)
		_ = conn.Close()
	}))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	proxy := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		f.ServeHTTP(w, req)
	})
	t.Cleanup(proxy.Close)

	serverAddr := proxy.Listener.Addr().String()

	headers := http.Header{}
	webSocketURL := "ws://" + serverAddr + "/ws"
	headers.Add("Origin", webSocketURL)

	conn, resp, err := gorillawebsocket.DefaultDialer.Dial(webSocketURL, headers)
	require.NoError(t, err, "Error during Dial with response: %+v", resp)

	_ = conn.WriteMessage(gorillawebsocket.TextMessage, []byte("OK"))
	t.Log(conn.ReadMessage())

	_ = conn.Close()
}

func TestWebSocketPassHost(t *testing.T) {
	testCases := []struct {
		desc     string
		passHost bool
		expected string
	}{
		{
			desc:     "PassHost false",
			passHost: false,
		},
		{
			desc:     "PassHost true",
			passHost: true,
			expected: "example.com",
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			f := New(test.passHost)

			mux := http.NewServeMux()
			mux.Handle("/ws", websocket.Handler(func(conn *websocket.Conn) {
				req := conn.Request()

				if test.passHost {
					require.Equal(t, test.expected, req.Host)
				} else {
					require.NotEqual(t, test.expected, req.Host)
				}

				msg := make([]byte, 4)
				_, _ = conn.Read(msg)
				t.Log(string(msg))
				_, _ = conn.Write(msg)
				_ = conn.Close()
			}))

			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			proxy := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
				req.URL = testutils.MustParseRequestURI(srv.URL)
				f.ServeHTTP(w, req)
			})
			t.Cleanup(proxy.Close)

			serverAddr := proxy.Listener.Addr().String()

			headers := http.Header{}
			webSocketURL := "ws://" + serverAddr + "/ws"
			headers.Add("Origin", webSocketURL)
			headers.Add("Host", "example.com")

			conn, resp, err := gorillawebsocket.DefaultDialer.Dial(webSocketURL, headers)
			require.NoError(t, err, "Error during Dial with response: %+v", resp)

			_ = conn.WriteMessage(gorillawebsocket.TextMessage, []byte("OK"))
			t.Log(conn.ReadMessage())

			_ = conn.Close()
		})
	}
}

func TestWebSocketServerWithoutCheckOrigin(t *testing.T) {
	f := New(true)

	upgrader := gorillawebsocket.Upgrader{CheckOrigin: func(_ *http.Request) bool {
		return true
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				break
			}
			err = c.WriteMessage(mt, message)
			if err != nil {
				break
			}
		}
	}))
	t.Cleanup(srv.Close)

	proxy := createProxyWithForwarder(f, srv.URL)
	t.Cleanup(proxy.Close)

	proxyAddr := proxy.Listener.Addr().String()
	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("ok"),
		withOrigin("http://127.0.0.2"),
	).send()

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestWebSocketRequestWithOrigin(t *testing.T) {
	f := New(true)

	upgrader := gorillawebsocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				break
			}
			err = c.WriteMessage(mt, message)
			if err != nil {
				break
			}
		}
	}))
	t.Cleanup(srv.Close)

	proxy := createProxyWithForwarder(f, srv.URL)
	t.Cleanup(proxy.Close)

	proxyAddr := proxy.Listener.Addr().String()
	_, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("echo"),
		withOrigin("http://127.0.0.2"),
	).send()
	require.EqualError(t, err, "bad status")

	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("ok"),
	).send()

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestWebSocketRequestWithQueryParams(t *testing.T) {
	f := New(true)

	upgrader := gorillawebsocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		assert.Equal(t, "test", r.URL.Query().Get("query"))
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			err = conn.WriteMessage(mt, message)
			if err != nil {
				break
			}
		}
	}))
	t.Cleanup(srv.Close)

	proxy := createProxyWithForwarder(f, srv.URL)
	t.Cleanup(proxy.Close)

	proxyAddr := proxy.Listener.Addr().String()

	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws?query=test"),
		withData("ok"),
	).send()

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestWebSocketRequestWithHeadersInResponseWriter(t *testing.T) {
	f := New(true)

	mux := http.NewServeMux()
	mux.Handle("/ws", websocket.Handler(func(conn *websocket.Conn) {
		_ = conn.Close()
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	proxy := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		w.Header().Set("HEADER-KEY", "HEADER-VALUE")
		f.ServeHTTP(w, req)
	})
	t.Cleanup(proxy.Close)

	serverAddr := proxy.Listener.Addr().String()

	headers := http.Header{}
	webSocketURL := "ws://" + serverAddr + "/ws"
	headers.Add("Origin", webSocketURL)
	conn, resp, err := gorillawebsocket.DefaultDialer.Dial(webSocketURL, headers)
	require.NoError(t, err, "Error during Dial with response: %+v", err, resp)
	t.Cleanup(func() { _ = conn.Close() })

	assert.Equal(t, "HEADER-VALUE", resp.Header.Get("HEADER-KEY"))
}

func TestWebSocketRequestWithEncodedChar(t *testing.T) {
	f := New(true)

	upgrader := gorillawebsocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		assert.Equal(t, "/%3A%2F%2F", r.URL.EscapedPath())
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			err = conn.WriteMessage(mt, message)
			if err != nil {
				break
			}
		}
	}))
	t.Cleanup(srv.Close)

	proxy := createProxyWithForwarder(f, srv.URL)
	t.Cleanup(proxy.Close)

	proxyAddr := proxy.Listener.Addr().String()

	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/%3A%2F%2F"),
		withData("ok"),
	).send()

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestWebSocketUpgradeFailed(t *testing.T) {
	f := New(true)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	proxy := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path // keep the original path

		if path == "/ws" {
			// Set new backend URL
			req.URL = testutils.MustParseRequestURI(srv.URL)
			req.URL.Path = path
			f.ServeHTTP(w, req)
		}
	})
	t.Cleanup(proxy.Close)

	proxyAddr := proxy.Listener.Addr().String()
	conn, err := net.DialTimeout("tcp", proxyAddr, dialTimeout)

	require.NoError(t, err)
	defer conn.Close()

	req, err := http.NewRequest(http.MethodGet, "ws://127.0.0.1/ws", nil)
	require.NoError(t, err)

	req.Header.Add("upgrade", "websocket")
	req.Header.Add("Connection", "upgrade")

	_ = req.Write(conn)

	// First request works with 400
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	require.NoError(t, err)

	assert.Equal(t, 400, resp.StatusCode)
}

func TestForwardsWebsocketTraffic(t *testing.T) {
	f := New(true)

	mux := http.NewServeMux()
	mux.Handle("/ws", websocket.Handler(func(conn *websocket.Conn) {
		_, _ = conn.Write([]byte("ok"))
		_ = conn.Close()
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	proxy := createProxyWithForwarder(f, srv.URL)
	t.Cleanup(proxy.Close)

	proxyAddr := proxy.Listener.Addr().String()
	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("echo"),
	).send()

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func createTLSWebsocketServer() *httptest.Server {
	upgrader := gorillawebsocket.Upgrader{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			err = conn.WriteMessage(mt, message)
			if err != nil {
				break
			}
		}
	}))
	return srv
}

func createProxyWithForwarder(forwarder http.Handler, uri string) *httptest.Server {
	return testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path // keep the original path
		// Set new backend URL
		req.URL = testutils.MustParseRequestURI(uri)
		req.URL.Path = path

		forwarder.ServeHTTP(w, req)
	})
}

func TestWebSocketTransferTLSConfig(t *testing.T) {
	srv := createTLSWebsocketServer()
	t.Cleanup(srv.Close)

	forwarderWithoutTLSConfig := New(true)

	proxyWithoutTLSConfig := createProxyWithForwarder(forwarderWithoutTLSConfig, srv.URL)
	defer proxyWithoutTLSConfig.Close()

	proxyAddr := proxyWithoutTLSConfig.Listener.Addr().String()

	_, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("ok"),
	).send()

	require.EqualError(t, err, "bad status")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	forwarderWithTLSConfig := New(true)
	forwarderWithTLSConfig.Transport = transport

	proxyWithTLSConfig := createProxyWithForwarder(forwarderWithTLSConfig, srv.URL)
	defer proxyWithTLSConfig.Close()

	proxyAddr = proxyWithTLSConfig.Listener.Addr().String()

	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("ok"),
	).send()

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	forwarderWithTLSConfigFromDefaultTransport := New(true)

	proxyWithTLSConfigFromDefaultTransport := createProxyWithForwarder(forwarderWithTLSConfigFromDefaultTransport, srv.URL)
	defer proxyWithTLSConfig.Close()

	proxyAddr = proxyWithTLSConfigFromDefaultTransport.Listener.Addr().String()

	resp, err = newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("ok"),
	).send()

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

const dialTimeout = clock.Second

type websocketRequestOpt func(w *websocketRequest)

func withServer(server string) websocketRequestOpt {
	return func(w *websocketRequest) {
		w.ServerAddr = server
	}
}

func withPath(path string) websocketRequestOpt {
	return func(w *websocketRequest) {
		w.Path = path
	}
}

func withData(data string) websocketRequestOpt {
	return func(w *websocketRequest) {
		w.Data = data
	}
}

func withOrigin(origin string) websocketRequestOpt {
	return func(w *websocketRequest) {
		w.Origin = origin
	}
}

func newWebsocketRequest(opts ...websocketRequestOpt) *websocketRequest {
	wsrequest := &websocketRequest{}
	for _, opt := range opts {
		opt(wsrequest)
	}
	if wsrequest.Origin == "" {
		wsrequest.Origin = "http://" + wsrequest.ServerAddr
	}
	if wsrequest.Config == nil {
		wsrequest.Config, _ = websocket.NewConfig(fmt.Sprintf("ws://%s%s", wsrequest.ServerAddr, wsrequest.Path), wsrequest.Origin)
	}
	return wsrequest
}

type websocketRequest struct {
	ServerAddr string
	Path       string
	Data       string
	Origin     string
	Config     *websocket.Config
}

func (w *websocketRequest) send() (string, error) {
	conn, _, err := w.open()
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(w.Data)); err != nil {
		return "", err
	}

	msg := make([]byte, 512)
	var n int
	n, err = conn.Read(msg)
	if err != nil {
		return "", err
	}

	received := string(msg[:n])
	return received, nil
}

func (w *websocketRequest) open() (*websocket.Conn, net.Conn, error) {
	client, err := net.DialTimeout("tcp", w.ServerAddr, dialTimeout)
	if err != nil {
		return nil, nil, err
	}
	conn, err := websocket.NewClient(w.Config, client)
	if err != nil {
		return nil, nil, err
	}
	return conn, client, err
}

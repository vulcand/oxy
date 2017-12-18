package forward

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"time"

	gorillawebsocket "github.com/gorilla/websocket"
	"github.com/vulcand/oxy/testutils"
	"golang.org/x/net/websocket"
	. "gopkg.in/check.v1"
)

func (s *FwdSuite) TestWebSocketEcho(c *C) {
	f, err := New()
	c.Assert(err, IsNil)

	mux := http.NewServeMux()
	mux.Handle("/ws", websocket.Handler(func(conn *websocket.Conn) {
		msg := make([]byte, 4)
		conn.Read(msg)
		c.Log(msg)
		conn.Write(msg)
		conn.Close()
	}))
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		mux.ServeHTTP(w, req)
	})
	defer srv.Close()
	proxy := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		f.ServeHTTP(w, req)
	})
	serverAddr := proxy.Listener.Addr().String()
	c.Log(serverAddr)
	headers := http.Header{}
	webSocketURL := "ws://" + serverAddr + "/ws"
	headers.Add("Origin", webSocketURL)
	conn, resp, err := gorillawebsocket.DefaultDialer.Dial(webSocketURL, headers)
	if err != nil {
		c.Errorf("Error [%s] during Dial with response: %+v", err, resp)
		return
	}
	conn.WriteMessage(gorillawebsocket.TextMessage, []byte("OK"))
	c.Log(conn.ReadMessage())

}

func (s *FwdSuite) TestWebSocketServerWithoutCheckOrigin(c *C) {
	f, err := New()
	c.Assert(err, IsNil)

	upgrader := gorillawebsocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
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
	defer srv.Close()

	proxy := createProxyWithForwarder(f, srv.URL)
	defer proxy.Close()

	proxyAddr := proxy.Listener.Addr().String()
	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("ok"),
		withOrigin("http://127.0.0.2"),
	).send()

	c.Assert(err, IsNil)
	c.Assert(resp, Equals, "ok")
}
func (s *FwdSuite) TestWebSocketRequestWithOrigin(c *C) {
	f, err := New()
	c.Assert(err, IsNil)

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
	defer srv.Close()

	proxy := createProxyWithForwarder(f, srv.URL)
	defer proxy.Close()

	proxyAddr := proxy.Listener.Addr().String()
	_, err = newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("echo"),
		withOrigin("http://127.0.0.2"),
	).send()

	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, "bad status")

	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("ok"),
	).send()

	c.Assert(err, IsNil)
	c.Assert(resp, Equals, "ok")
}

func (s *FwdSuite) TestWebSocketRequestWithQueryParams(c *C) {
	f, err := New()
	c.Assert(err, IsNil)

	upgrader := gorillawebsocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		c.Assert(r.URL.Query().Get("query"), Equals, "test")
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
	defer srv.Close()

	proxy := createProxyWithForwarder(f, srv.URL)
	defer proxy.Close()

	proxyAddr := proxy.Listener.Addr().String()

	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws?query=test"),
		withData("ok"),
	).send()

	c.Assert(err, IsNil)
	c.Assert(resp, Equals, "ok")
}

func (s *FwdSuite) TestWebSocketRequestWithEncodedChar(c *C) {
	f, err := New()
	c.Assert(err, IsNil)

	upgrader := gorillawebsocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		c.Assert(r.URL.Path, Equals, "/%3A%2F%2F")
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
	defer srv.Close()

	proxy := createProxyWithForwarder(f, srv.URL)
	defer proxy.Close()

	proxyAddr := proxy.Listener.Addr().String()

	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/%3A%2F%2F"),
		withData("ok"),
	).send()

	c.Assert(err, IsNil)
	c.Assert(resp, Equals, "ok")
}

func (s *FwdSuite) TestDetectsWebSocketRequest(c *C) {
	mux := http.NewServeMux()
	mux.Handle("/ws", websocket.Handler(func(conn *websocket.Conn) {
		conn.Write([]byte("ok"))
		conn.Close()
	}))
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		websocketRequest := IsWebsocketRequest(req)
		c.Assert(websocketRequest, Equals, true)
		mux.ServeHTTP(w, req)
	})
	defer srv.Close()

	serverAddr := srv.Listener.Addr().String()

	resp, err := newWebsocketRequest(
		withServer(serverAddr),
		withPath("/ws"),
		withData("echo"),
	).send()

	c.Assert(err, IsNil)
	c.Assert(resp, Equals, "ok")
}

func (s *FwdSuite) TestWebSocketUpgradeFailed(c *C) {
	f, err := New()
	c.Assert(err, IsNil)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(400)
	})
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		mux.ServeHTTP(w, req)
	})
	defer srv.Close()

	proxy := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path // keep the original path

		if path == "/ws" {
			// Set new backend URL
			req.URL = testutils.ParseURI(srv.URL)
			req.URL.Path = path
			websocketRequest := IsWebsocketRequest(req)
			c.Assert(websocketRequest, Equals, true)
			f.ServeHTTP(w, req)
		} else {
			w.WriteHeader(200)
		}
	})
	defer proxy.Close()

	proxyAddr := proxy.Listener.Addr().String()
	conn, err := net.DialTimeout("tcp", proxyAddr, time.Second*5)

	c.Assert(err, IsNil)
	defer conn.Close()

	req, err := http.NewRequest(http.MethodGet, "ws://127.0.0.1/ws", nil)
	c.Assert(err, IsNil)

	req.Header.Add("upgrade", "websocket")
	req.Header.Add("Connection", "upgrade")

	req.Write(conn)

	// First request works with 400
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 400)

	req, err = http.NewRequest(http.MethodGet, "ws://127.0.0.1/ws2", nil)
	c.Assert(err, IsNil)
	req.Header.Add("upgrade", "websocket")
	req.Header.Add("Connection", "upgrade")
	req.Write(conn)

	br = bufio.NewReader(conn)
	_, err = http.ReadResponse(br, req)
	c.Assert(err, Equals, io.ErrUnexpectedEOF)
}

func (s *FwdSuite) TestForwardsWebsocketTraffic(c *C) {
	f, err := New()
	c.Assert(err, IsNil)

	mux := http.NewServeMux()
	mux.Handle("/ws", websocket.Handler(func(conn *websocket.Conn) {
		conn.Write([]byte("ok"))
		conn.Close()
	}))
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		mux.ServeHTTP(w, req)
	})
	defer srv.Close()

	proxy := createProxyWithForwarder(f, srv.URL)
	defer proxy.Close()

	proxyAddr := proxy.Listener.Addr().String()
	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("echo"),
	).send()

	c.Assert(err, IsNil)
	c.Assert(resp, Equals, "ok")
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

func createProxyWithForwarder(forwarder *Forwarder, URL string) *httptest.Server {
	return testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path // keep the original path
		// Set new backend URL
		req.URL = testutils.ParseURI(URL)
		req.URL.Path = path

		forwarder.ServeHTTP(w, req)
	})
}

func (s *FwdSuite) TestWebSocketTransferTLSConfig(c *C) {
	srv := createTLSWebsocketServer()
	defer srv.Close()

	forwarderWithoutTLSConfig, err := New()
	c.Assert(err, IsNil)

	proxyWithoutTLSConfig := createProxyWithForwarder(forwarderWithoutTLSConfig, srv.URL)
	defer proxyWithoutTLSConfig.Close()

	proxyAddr := proxyWithoutTLSConfig.Listener.Addr().String()

	_, err = newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("ok"),
	).send()

	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, "bad status")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	forwarderWithTLSConfig, err := New(RoundTripper(transport))
	c.Assert(err, IsNil)

	proxyWithTLSConfig := createProxyWithForwarder(forwarderWithTLSConfig, srv.URL)
	defer proxyWithTLSConfig.Close()

	proxyAddr = proxyWithTLSConfig.Listener.Addr().String()

	resp, err := newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("ok"),
	).send()

	c.Assert(err, IsNil)
	c.Assert(resp, Equals, "ok")

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	forwarderWithTLSConfigFromDefaultTransport, err := New()
	c.Assert(err, IsNil)

	proxyWithTLSConfigFromDefaultTransport := createProxyWithForwarder(forwarderWithTLSConfigFromDefaultTransport, srv.URL)
	defer proxyWithTLSConfig.Close()

	proxyAddr = proxyWithTLSConfigFromDefaultTransport.Listener.Addr().String()

	resp, err = newWebsocketRequest(
		withServer(proxyAddr),
		withPath("/ws"),
		withData("ok"),
	).send()

	c.Assert(err, IsNil)
	c.Assert(resp, Equals, "ok")
}

const dialTimeout = time.Second

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
	client, err := net.DialTimeout("tcp", w.ServerAddr, dialTimeout)
	if err != nil {
		return "", err
	}
	conn, err := websocket.NewClient(w.Config, client)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(w.Data)); err != nil {
		return "", err
	}
	var msg = make([]byte, 512)
	var n int
	n, err = conn.Read(msg)
	if err != nil {
		return "", err
	}

	received := string(msg[:n])
	return received, nil
}

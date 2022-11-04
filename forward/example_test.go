package forward

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
)

func ExampleNew_customErrHandler() {
	f := New(true)
	f.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(http.StatusText(http.StatusTeapot)))
	}

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL, _ = url.ParseRequestURI("http://localhost:63450")
		f.ServeHTTP(w, req)
	}))
	defer proxy.Close()

	resp, err := http.Get(proxy.URL)
	if err != nil {
		fmt.Println(err)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(resp.StatusCode)
	fmt.Println(string(body))

	// output:
	// 418
	// I'm a teapot
}

func ExampleNew_responseModifier() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	f := New(true)
	f.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Add("X-Test", "CUSTOM")
		return nil
	}

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL, _ = url.ParseRequestURI(srv.URL)
		f.ServeHTTP(w, req)
	}))
	defer proxy.Close()

	resp, err := http.Get(proxy.URL)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Header.Get("X-Test"))

	// Output:
	// 200
	// CUSTOM
}

func ExampleNew_customTransport() {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	f := New(true)

	f.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL, _ = url.ParseRequestURI(srv.URL)
		f.ServeHTTP(w, req)
	}))
	defer proxy.Close()

	resp, err := http.Get(proxy.URL)
	if err != nil {
		fmt.Println(err)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(resp.StatusCode)
	fmt.Println(string(body))

	// Output:
	// 200
	// hello
}

func ExampleNewStateListener() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	f := New(true)
	f.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Add("X-Test", "CUSTOM")
		return nil
	}

	stateLn := NewStateListener(f, func(u *url.URL, i int) {
		fmt.Println(u.Hostname(), i)
	})
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL, _ = url.ParseRequestURI(srv.URL)
		stateLn.ServeHTTP(w, req)
	}))
	defer proxy.Close()

	resp, err := http.Get(proxy.URL)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(resp.StatusCode)

	// Output:
	// 127.0.0.1 0
	// 127.0.0.1 1
	// 200
}

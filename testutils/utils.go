package testutils

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/mailgun/oxy/utils"
)

func NewHandler(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(handler))
}

func NewResponder(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	}))
}

// ParseURI is the version of url.ParseRequestURI that panics if incorrect, helpful to shorten the tests
func ParseURI(uri string) *url.URL {
	out, err := url.ParseRequestURI(uri)
	if err != nil {
		panic(err)
	}
	return out
}

type reqOpts struct {
	Host    string
	Method  string
	Body    string
	Headers http.Header
}

type reqOptSetter func(o *reqOpts) error

func Method(m string) reqOptSetter {
	return func(o *reqOpts) error {
		o.Method = m
		return nil
	}
}

func Host(h string) reqOptSetter {
	return func(o *reqOpts) error {
		o.Host = h
		return nil
	}
}

func Body(b string) reqOptSetter {
	return func(o *reqOpts) error {
		o.Body = b
		return nil
	}
}

func Header(name, val string) reqOptSetter {
	return func(o *reqOpts) error {
		if o.Headers == nil {
			o.Headers = make(http.Header)
		}
		o.Headers.Add(name, val)
		return nil
	}
}

func Headers(h http.Header) reqOptSetter {
	return func(o *reqOpts) error {
		if o.Headers == nil {
			o.Headers = make(http.Header)
		}
		utils.CopyHeaders(o.Headers, h)
		return nil
	}
}

func MakeRequest(url string, opts ...reqOptSetter) (*http.Response, []byte, error) {
	o := &reqOpts{}
	for _, s := range opts {
		if err := s(o); err != nil {
			return nil, nil, err
		}
	}

	method := "GET"
	if o.Method == "" {
		o.Method = "GET"
	}
	request, _ := http.NewRequest(method, url, strings.NewReader(o.Body))
	if o.Headers != nil {
		utils.CopyHeaders(request.Header, o.Headers)
	}

	if len(o.Host) != 0 {
		request.Host = o.Host
	}

	var tr *http.Transport
	if strings.HasPrefix(url, "https") {
		tr = &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		}
	} else {
		tr = &http.Transport{
			DisableKeepAlives: true,
		}
	}

	client := &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("No redirects")
		},
	}
	response, err := client.Do(request)
	if err == nil {
		bodyBytes, err := ioutil.ReadAll(response.Body)
		return response, bodyBytes, err
	}
	return response, nil, err
}

func Get(url string, opts ...reqOptSetter) (*http.Response, []byte, error) {
	opts = append(opts, Method("GET"))
	return MakeRequest(url, opts...)
}

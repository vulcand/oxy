package forward

import (
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/vulcand/oxy/v2/utils"
)

// HeaderRewriter is responsible for removing hop-by-hop headers and setting forwarding headers.
type HeaderRewriter struct {
	TrustForwardHeader bool
	Hostname           string
}

// NewHeaderRewriter creates a new HeaderRewriter middleware.
func NewHeaderRewriter() *HeaderRewriter {
	h, err := os.Hostname()
	if err != nil {
		h = "localhost"
	}

	return &HeaderRewriter{TrustForwardHeader: true, Hostname: h}
}

// clean up IP in case if it is ipv6 address and it has {zone} information in it, like "[fe80::d806:a55d:eb1b:49cc%vEthernet (vmxnet3 Ethernet Adapter - Virtual Switch)]:64692".
func ipv6fix(clientIP string) string {
	return strings.Split(clientIP, "%")[0]
}

// Rewrite request headers.
func (rw *HeaderRewriter) Rewrite(req *http.Request) {
	if !rw.TrustForwardHeader {
		utils.RemoveHeaders(req.Header, XHeaders...)
	}

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		clientIP = ipv6fix(clientIP)

		if req.Header.Get(XRealIP) == "" {
			req.Header.Set(XRealIP, clientIP)
		}
	}

	xfProto := req.Header.Get(XForwardedProto)
	if xfProto == "" {
		if req.TLS != nil {
			req.Header.Set(XForwardedProto, "https")
		} else {
			req.Header.Set(XForwardedProto, "http")
		}
	}

	if xfPort := req.Header.Get(XForwardedPort); xfPort == "" {
		req.Header.Set(XForwardedPort, forwardedPort(req))
	}

	if xfHost := req.Header.Get(XForwardedHost); xfHost == "" && req.Host != "" {
		req.Header.Set(XForwardedHost, req.Host)
	}

	if rw.Hostname != "" {
		req.Header.Set(XForwardedServer, rw.Hostname)
	}
}

func forwardedPort(req *http.Request) string {
	if req == nil {
		return ""
	}

	if _, port, err := net.SplitHostPort(req.Host); err == nil && port != "" {
		return port
	}

	if req.Header.Get(XForwardedProto) == "https" || req.Header.Get(XForwardedProto) == "wss" {
		return "443"
	}

	if req.TLS != nil {
		return "443"
	}

	return "80"
}

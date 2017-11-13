package forward

const (
	XForwardedProto    = "X-Forwarded-Proto"
	XForwardedFor      = "X-Forwarded-For"
	XForwardedHost     = "X-Forwarded-Host"
	XForwardedPort     = "X-Forwarded-Port"
	XForwardedServer   = "X-Forwarded-Server"
	XRealIp            = "X-Real-Ip"
	Connection         = "Connection"
	KeepAlive          = "Keep-Alive"
	ProxyAuthenticate  = "Proxy-Authenticate"
	ProxyAuthorization = "Proxy-Authorization"
	Te                 = "Te" // canonicalized version of "TE"
	Trailers           = "Trailers"
	TransferEncoding   = "Transfer-Encoding"
	Upgrade            = "Upgrade"
	ContentLength      = "Content-Length"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
// Copied from reverseproxy.go, too bad
var HopHeaders = []string{
	Connection,
	KeepAlive,
	ProxyAuthenticate,
	ProxyAuthorization,
	Te, // canonicalized version of "TE"
	Trailers,
	TransferEncoding,
	Upgrade,
}

var XHeaders = []string{
	XForwardedProto,
	XForwardedFor,
	XForwardedHost,
	XForwardedPort,
	XForwardedServer,
	XRealIp,
}

package forward

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ipv6fix(t *testing.T) {
	testCases := []struct {
		desc     string
		clientIP string
		expected string
	}{
		{
			desc:     "empty",
			clientIP: "",
			expected: "",
		},
		{
			desc:     "ipv4 localhost",
			clientIP: "127.0.0.1",
			expected: "127.0.0.1",
		},
		{
			desc:     "ipv4",
			clientIP: "10.13.14.15",
			expected: "10.13.14.15",
		},
		{
			desc:     "ipv6 zone",
			clientIP: `fe80::d806:a55d:eb1b:49cc%vEthernet (vmxnet3 Ethernet Adapter - Virtual Switch)`,
			expected: "fe80::d806:a55d:eb1b:49cc",
		},
		{
			desc:     "ipv6 medium",
			clientIP: `fe80::1`,
			expected: "fe80::1",
		},
		{
			desc:     "ipv6 small",
			clientIP: `2000::`,
			expected: "2000::",
		},
		{
			desc:     "ipv6",
			clientIP: `2001:3452:4952:2837::`,
			expected: "2001:3452:4952:2837::",
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			actual := ipv6fix(test.clientIP)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestHeaderRewriter_Rewrite_XForwardedFor(t *testing.T) {
	testCases := []struct {
		desc                 string
		trustForwardHeader   bool
		remoteAddr           string
		initialHeaders       http.Header
		expectedXRealIP      string
		expectedForwardedFor string
	}{
		{
			desc:               "appends remote addr to existing X-Forwarded-For when trusted",
			trustForwardHeader: true,
			remoteAddr:         "192.0.2.1:12345",
			initialHeaders: http.Header{
				XForwardedFor: []string{"203.0.113.1"},
			},
			expectedXRealIP:      "192.0.2.1",
			expectedForwardedFor: "203.0.113.1, 192.0.2.1",
		},
		{
			desc:               "appends remote addr to multiple existing X-Forwarded-For when trusted",
			trustForwardHeader: true,
			remoteAddr:         "192.0.2.1:12345",
			initialHeaders: http.Header{
				XForwardedFor: []string{"203.0.113.1", "198.51.100.2"},
			},
			expectedXRealIP:      "192.0.2.1",
			expectedForwardedFor: "203.0.113.1, 198.51.100.2, 192.0.2.1",
		},
		{
			desc:                 "sets remote addr when no existing X-Forwarded-For",
			trustForwardHeader:   true,
			remoteAddr:           "192.0.2.1:12345",
			initialHeaders:       http.Header{},
			expectedXRealIP:      "192.0.2.1",
			expectedForwardedFor: "192.0.2.1",
		},
		{
			desc:               "strips existing X-Forwarded-For and sets remote addr when not trusted",
			trustForwardHeader: false,
			remoteAddr:         "192.0.2.1:12345",
			initialHeaders: http.Header{
				XForwardedFor: []string{"203.0.113.1"},
			},
			expectedXRealIP:      "192.0.2.1",
			expectedForwardedFor: "192.0.2.1",
		},
		{
			desc:                 "handles remote addr without port",
			trustForwardHeader:   true,
			remoteAddr:           "192.0.2.1",
			initialHeaders:       http.Header{},
			expectedXRealIP:      "192.0.2.1",
			expectedForwardedFor: "192.0.2.1",
		},
		{
			desc:                 "handles IPv6 remote addr",
			trustForwardHeader:   true,
			remoteAddr:           "[2001:db8::1]:12345",
			initialHeaders:       http.Header{},
			expectedXRealIP:      "2001:db8::1",
			expectedForwardedFor: "2001:db8::1",
		},
		{
			desc:                 "handles IPv6 remote addr with zone",
			trustForwardHeader:   true,
			remoteAddr:           "[fe80::1%eth0]:12345",
			initialHeaders:       http.Header{},
			expectedXRealIP:      "fe80::1",
			expectedForwardedFor: "fe80::1",
		},
		{
			desc:               "preserves existing X-Real-IP if present",
			trustForwardHeader: true,
			remoteAddr:         "192.0.2.1:12345",
			initialHeaders: http.Header{
				XRealIP: []string{"10.0.0.1"},
			},
			expectedXRealIP:      "10.0.0.1",
			expectedForwardedFor: "192.0.2.1",
		},
		{
			desc:               "omits X-Forwarded-For if key set to nil",
			trustForwardHeader: true,
			remoteAddr:         "192.0.2.1:12345",
			initialHeaders: http.Header{
				XForwardedFor: nil,
			},
			expectedXRealIP:      "192.0.2.1",
			expectedForwardedFor: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			rw := &HeaderRewriter{
				TrustForwardHeader: tc.trustForwardHeader,
				Hostname:           "test-host",
			}

			req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
			require.NoError(t, err)
			req.RemoteAddr = tc.remoteAddr
			for k, values := range tc.initialHeaders {
				if values == nil {
					req.Header[k] = nil
				} else {
					for _, v := range values {
						req.Header.Add(k, v)
					}
				}
			}

			rw.Rewrite(req)

			assert.Equal(t, tc.expectedXRealIP, req.Header.Get(XRealIP))
			assert.Equal(t, tc.expectedForwardedFor, req.Header.Get(XForwardedFor))
		})
	}
}

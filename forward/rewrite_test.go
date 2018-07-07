package forward

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIPv6Fix(t *testing.T) {
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
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			actual := ipv6fix(test.clientIP)
			assert.Equal(t, test.expected, actual)
		})
	}
}

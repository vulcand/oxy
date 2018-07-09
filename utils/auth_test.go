package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Just to make sure we don't panic, return err and not
// username and pass and cover the function
func TestParseBadHeaders(t *testing.T) {
	headers := []string{
		// just empty string
		"",
		// missing auth type
		"justplainstring",
		// unknown auth type
		"Whut justplainstring",
		// invalid base64
		"Basic Shmasic",
		// random encoded string
		"Basic YW55IGNhcm5hbCBwbGVhcw==",
	}
	for _, h := range headers {
		_, err := ParseAuthHeader(h)
		require.Error(t, err)
	}
}

// Just to make sure we don't panic, return err and not
// username and pass and cover the function
func TestParseSuccess(t *testing.T) {
	headers := []struct {
		Header   string
		Expected BasicAuth
	}{
		{
			"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==",
			BasicAuth{Username: "Aladdin", Password: "open sesame"},
		},
		// Make sure that String() produces valid header
		{
			(&BasicAuth{Username: "Alice", Password: "Here's bob"}).String(),
			BasicAuth{Username: "Alice", Password: "Here's bob"},
		},
		// empty pass
		{
			"Basic QWxhZGRpbjo=",
			BasicAuth{Username: "Aladdin", Password: ""},
		},
	}
	for _, h := range headers {
		request, err := ParseAuthHeader(h.Header)
		require.NoError(t, err)
		assert.Equal(t, h.Expected.Username, request.Username)
		assert.Equal(t, h.Expected.Password, request.Password)

	}
}

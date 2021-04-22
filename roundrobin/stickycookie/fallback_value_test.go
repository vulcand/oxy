package stickycookie

import (
	"fmt"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestFallbackValue_FindURL(t *testing.T) {
	servers := []*url.URL{
		{Scheme: "https", Host: "10.10.10.42", Path: "/"},
		{Scheme: "http", Host: "10.10.10.10", Path: "/foo"},
		{Scheme: "http", Host: "10.10.10.11", Path: "/", User: url.User("John Doe")},
		{Scheme: "http", Host: "10.10.10.10", Path: "/"},
	}

	aesValue, err := NewAESValue([]byte("95Bx9JkKX3xbd7z3"), 5*time.Second)
	require.NoError(t, err)

	values := []struct {
		Name        string
		CookieValue CookieValue
	}{
		{Name: "rawValue", CookieValue: &RawValue{}},
		{Name: "hashValue", CookieValue: &HashValue{Salt: "foo"}},
		{Name: "aesValue", CookieValue: aesValue},
	}

	for _, from := range values {
		from := from
		for _, to := range values {
			to := to
			t.Run(fmt.Sprintf("From: %s, To %s", from.Name, to.Name), func(t *testing.T) {
				t.Parallel()

				value, err := NewFallbackValue(from.CookieValue, to.CookieValue)
				if from.CookieValue == nil && to.CookieValue == nil {
					assert.Error(t, err)
					return
				}
				require.NoError(t, err)

				if from.CookieValue != nil {
					// URL found From value
					findURL, err := value.FindURL(from.CookieValue.Get(servers[0]), servers)
					require.NoError(t, err)
					assert.Equal(t, servers[0], findURL)

					// URL not found From value
					findURL, _ = value.FindURL(from.CookieValue.Get(mustJoin(t, servers[0], "bar")), servers)
					assert.Nil(t, findURL)
				}

				if to.CookieValue != nil {
					// URL found To Value
					findURL, err := value.FindURL(to.CookieValue.Get(servers[0]), servers)
					require.NoError(t, err)
					assert.Equal(t, servers[0], findURL)

					// URL not found To value
					findURL, _ = value.FindURL(to.CookieValue.Get(mustJoin(t, servers[0], "bar")), servers)
					assert.Nil(t, findURL)
				}
			})
		}
	}
}

func TestFallbackValue_FindURL_error(t *testing.T) {
	servers := []*url.URL{
		{Scheme: "http", Host: "10.10.10.10", Path: "/"},
		{Scheme: "https", Host: "10.10.10.42", Path: "/"},
		{Scheme: "http", Host: "10.10.10.10", Path: "/foo"},
		{Scheme: "http", Host: "10.10.10.11", Path: "/", User: url.User("John Doe")},
	}

	hashValue := &HashValue{Salt: "foo"}
	rawValue := &RawValue{}
	aesValue, err := NewAESValue([]byte("95Bx9JkKX3xbd7z3"), 5*time.Second)
	require.NoError(t, err)

	tests := []struct {
		name             string
		From             CookieValue
		To               CookieValue
		rawValue         string
		want             *url.URL
		expectError      bool
		expectErrorOnNew bool
	}{
		{
			name:     "From RawValue To HashValue with RawValue value",
			From:     rawValue,
			To:       hashValue,
			rawValue: "http://10.10.10.10/",
			want:     servers[0],
		},
		{
			name:     "From RawValue To HashValue with RawValue non matching value",
			From:     rawValue,
			To:       hashValue,
			rawValue: "http://24.10.10.10/",
		},
		{
			name:     "From RawValue To HashValue with HashValue value",
			From:     rawValue,
			To:       hashValue,
			rawValue: hashValue.Get(mustParse(t, "http://10.10.10.10/")),
			want:     servers[0],
		},
		{
			name:     "From RawValue To HashValue with HashValue non matching value",
			From:     rawValue,
			To:       hashValue,
			rawValue: hashValue.Get(mustParse(t, "http://24.10.10.10/")),
		},
		{
			name:     "From HashValue To AESValue with AESValue value",
			From:     hashValue,
			To:       aesValue,
			rawValue: aesValue.Get(mustParse(t, "http://10.10.10.10/")),
			want:     servers[0],
		},
		{
			name:     "From HashValue To AESValue with AESValue non matching value",
			From:     hashValue,
			To:       aesValue,
			rawValue: aesValue.Get(mustParse(t, "http://24.10.10.10/")),
		},
		{
			name:     "From HashValue To AESValue with HashValue value",
			From:     hashValue,
			To:       aesValue,
			rawValue: hashValue.Get(mustParse(t, "http://10.10.10.10/")),
			want:     servers[0],
		},
		{
			name:     "From HashValue To AESValue with AESValue non matching value",
			From:     hashValue,
			To:       aesValue,
			rawValue: aesValue.Get(mustParse(t, "http://24.10.10.10/")),
		},
		{
			name:     "From AESValue To AESValue with AESValue value",
			From:     aesValue,
			To:       aesValue,
			rawValue: aesValue.Get(mustParse(t, "http://10.10.10.10/")),
			want:     servers[0],
		},
		{
			name:     "From AESValue To AESValue with AESValue non matching value",
			From:     aesValue,
			To:       aesValue,
			rawValue: aesValue.Get(mustParse(t, "http://24.10.10.10/")),
		},
		{
			name:     "From AESValue To HashValue with HashValue non matching value",
			From:     aesValue,
			To:       hashValue,
			rawValue: hashValue.Get(mustParse(t, "http://24.10.10.10/")),
		},
		{
			name:             "From nil To RawValue",
			To:               hashValue,
			rawValue:         "http://24.10.10.10/",
			expectErrorOnNew: true,
		},
		{
			name:             "From RawValue To nil",
			From:             rawValue,
			rawValue:         "http://24.10.10.10/",
			expectErrorOnNew: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := NewFallbackValue(tt.From, tt.To)
			if tt.expectErrorOnNew {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			findURL, err := value.FindURL(tt.rawValue, servers)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.want, findURL)
		})
	}
}

func mustJoin(t *testing.T, u *url.URL, part string) *url.URL {
	t.Helper()

	nu, err := u.Parse(path.Join(u.Path, part))
	require.NoError(t, err)

	return nu
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()

	u, err := url.Parse(raw)
	require.NoError(t, err)

	return u
}

package utils

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// BasicAuth basic auth information.
type BasicAuth struct {
	Username string
	Password string
}

func (ba *BasicAuth) String() string {
	encoded := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s:%s", ba.Username, ba.Password))
	return fmt.Sprintf("Basic %s", encoded)
}

// ParseAuthHeader creates a new BasicAuth from header values.
func ParseAuthHeader(header string) (*BasicAuth, error) {
	values := strings.Fields(header)
	if len(values) != 2 {
		return nil, fmt.Errorf("failed to parse header '%s'", header)
	}

	authType := strings.ToLower(values[0])
	if authType != "basic" {
		return nil, fmt.Errorf("expected basic auth type, got '%s'", authType)
	}

	encodedString := values[1]

	decodedString, err := base64.StdEncoding.DecodeString(encodedString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse header '%s', base64 failed: %w", header, err)
	}

	values = strings.SplitN(string(decodedString), ":", 2)
	if len(values) != 2 {
		return nil, fmt.Errorf("failed to parse header '%s', expected separator ':'", header)
	}

	return &BasicAuth{Username: values[0], Password: values[1]}, nil
}

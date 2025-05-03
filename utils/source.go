package utils

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// SourceExtractor extracts the source from the request, e.g. that may be client ip, or particular header that
// identifies the source. amount stands for amount of connections the source consumes, usually 1 for connection limiters
// error should be returned when source can not be identified.
type SourceExtractor interface {
	Extract(req *http.Request) (token string, amount int64, err error)
}

// ExtractorFunc extractor function type.
type ExtractorFunc func(req *http.Request) (token string, amount int64, err error)

// Extract extract from request.
func (f ExtractorFunc) Extract(req *http.Request) (string, int64, error) {
	return f(req)
}

// ExtractSource extract source function type.
type ExtractSource func(req *http.Request)

// NewExtractor creates a new SourceExtractor.
func NewExtractor(variable string) (SourceExtractor, error) {
	if variable == "client.ip" {
		return ExtractorFunc(extractClientIP), nil
	}
	if strings.HasPrefix(variable, "client.ip.api.") {
		apiRegex := strings.TrimPrefix(variable, "client.ip.api.(")
		if len(apiRegex) == 0 {
			return nil, fmt.Errorf("api extraction regex not provided")
		}

		reg := strings.TrimSuffix(apiRegex, ")")
		lambda := makeClientIPWithPathExtractor(reg)
		if lambda == nil {
			return nil, fmt.Errorf("Failed to compile regex: %s", reg)
		}

		return lambda, nil
	}
	if variable == "request.host" {
		return ExtractorFunc(extractHost), nil
	}
	if strings.HasPrefix(variable, "request.header.") {
		header := strings.TrimPrefix(variable, "request.header.")
		if header == "" {
			return nil, fmt.Errorf("wrong header: %s", header)
		}
		return makeHeaderExtractor(header), nil
	}
	return nil, fmt.Errorf("unsupported limiting variable: '%s'", variable)
}

func extractClientIP(req *http.Request) (string, int64, error) {
	vals := strings.SplitN(req.RemoteAddr, ":", 2)
	if vals[0] == "" {
		return "", 0, fmt.Errorf("failed to parse client IP: %v", req.RemoteAddr)
	}
	return vals[0], 1, nil
}

func extractHost(req *http.Request) (string, int64, error) {
	return req.Host, 1, nil
}

func makeHeaderExtractor(header string) SourceExtractor {
	return ExtractorFunc(func(req *http.Request) (string, int64, error) {
		return req.Header.Get(header), 1, nil
	})
}

//Create regex extractor function from client ip +request path
func makeClientIPWithPathExtractor(extractRegex string) SourceExtractor {
	compiled, err := regexp.Compile(extractRegex)
	if err != nil {
		return nil
	}

	return ExtractorFunc(func(req *http.Request) (string, int64, error) {
		vals := strings.SplitN(req.RemoteAddr, ":", 2)
		if len(vals[0]) == 0 {
			return "", 0, fmt.Errorf("failed to parse client IP: %v", req.RemoteAddr)
		}

		client := vals[0]
		u, err := url.Parse(req.RequestURI)
		if err != nil {
			return "", 0, fmt.Errorf("Failed to parse URI")
		}

		api := compiled.FindString(u.Path)
		ret := client + api
		if log.StandardLogger().Level >= log.DebugLevel {
			fields := log.Fields{}
			fields["api"] = api
			fields["client"] = client
			logEntry := log.StandardLogger().WithFields(fields)
			logEntry.Debug("vulcand/oxy/SourceExtractor: extracted API")
		}
		return ret, 1, nil
	})
}

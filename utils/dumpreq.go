package utils

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
)

// SerializableHttpRequest alias on SerializableHTTPRequest.
// Deprecated: use SerializableHTTPRequest instead.
type SerializableHttpRequest = SerializableHTTPRequest

// SerializableHTTPRequest serializable HTTP request.
type SerializableHTTPRequest struct {
	Method           string
	URL              *url.URL
	Proto            string // "HTTP/1.0"
	ProtoMajor       int    // 1
	ProtoMinor       int    // 0
	Header           http.Header
	ContentLength    int64
	TransferEncoding []string
	Host             string
	Form             url.Values
	PostForm         url.Values
	MultipartForm    *multipart.Form
	Trailer          http.Header
	RemoteAddr       string
	RequestURI       string
	TLS              *tls.ConnectionState
}

// Clone clone a request.
func Clone(r *http.Request) *SerializableHTTPRequest {
	if r == nil {
		return nil
	}

	rc := new(SerializableHTTPRequest)
	rc.Method = r.Method
	rc.URL = r.URL
	rc.Proto = r.Proto
	rc.ProtoMajor = r.ProtoMajor
	rc.ProtoMinor = r.ProtoMinor
	rc.Header = r.Header
	rc.ContentLength = r.ContentLength
	rc.Host = r.Host
	rc.RemoteAddr = r.RemoteAddr
	rc.RequestURI = r.RequestURI
	return rc
}

// ToJson serializes to JSON.
// Deprecated: use ToJSON instead.
func (s *SerializableHTTPRequest) ToJson() string {
	return s.ToJSON()
}

// ToJSON serializes to JSON.
func (s *SerializableHTTPRequest) ToJSON() string {
	jsonVal, err := json.Marshal(s)
	if err != nil || jsonVal == nil {
		return fmt.Sprintf("Error marshalling SerializableHTTPRequest to json: %s", err)
	}
	return string(jsonVal)
}

// DumpHttpRequest dump a HTTP request to JSON.
// Deprecated: use DumpHTTPRequest instead.
func DumpHttpRequest(req *http.Request) string {
	return DumpHTTPRequest(req)
}

// DumpHTTPRequest dump a HTTP request to JSON.
func DumpHTTPRequest(req *http.Request) string {
	return Clone(req).ToJSON()
}

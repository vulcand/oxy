package utils

import (
	"net/http"
)

type BadGatewayHandler struct {
}

func (e *BadGatewayHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusBadGateway)
	w.Write([]byte(http.StatusText(http.StatusBadGateway)))
}

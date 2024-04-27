package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPathExtractor(t *testing.T) {
	extractFunc, err := NewExtractor("client.ip.api.(^([a-zA-Z0-9/]{0,10}))")
	if err != nil {
		panic("Fail to init extractor")
	}

	testUrl := "https://test.com/cms/apiv2/workspaces/8148f4dc-f8c1-4d03-8fd8-6b14488aa016/tables/W3CIISLog?test=1234&xxx=567"
	req := httptest.NewRequest(http.MethodGet, testUrl, nil)

	key, status, err := extractFunc.Extract(req)

	t.Log(key)
	t.Log(status)

}

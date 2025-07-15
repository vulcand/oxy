package cbreaker

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/vulcand/oxy/v2/utils"
)

// SideEffect a side effect.
type SideEffect interface {
	Exec() error
}

// Webhook Web hook.
type Webhook struct {
	URL     string
	Method  string
	Headers http.Header
	Form    url.Values
	Body    []byte
}

// WebhookSideEffect a web hook side effect.
type WebhookSideEffect struct {
	w Webhook

	log utils.Logger
}

// NewWebhookSideEffectsWithLogger creates a new WebhookSideEffect.
func NewWebhookSideEffectsWithLogger(w Webhook, l utils.Logger) (*WebhookSideEffect, error) {
	if w.Method == "" {
		return nil, errors.New("supply method")
	}

	_, err := url.Parse(w.URL)
	if err != nil {
		return nil, err
	}

	return &WebhookSideEffect{w: w, log: l}, nil
}

// NewWebhookSideEffect creates a new WebhookSideEffect.
func NewWebhookSideEffect(w Webhook) (*WebhookSideEffect, error) {
	return NewWebhookSideEffectsWithLogger(w, &utils.NoopLogger{})
}

// Exec execute the side effect.
func (w *WebhookSideEffect) Exec() error {
	r, err := http.NewRequest(w.w.Method, w.w.URL, w.getBody())
	if err != nil {
		return err
	}

	if len(w.w.Headers) != 0 {
		utils.CopyHeaders(r.Header, w.w.Headers)
	}

	if len(w.w.Form) != 0 {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	re, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}

	if re.Body != nil {
		defer func() { _ = re.Body.Close() }()
	}

	body, err := io.ReadAll(re.Body)
	if err != nil {
		return err
	}

	w.log.Debug("%v got response: (%s): %s", w, re.Status, string(body))

	return nil
}

func (w *WebhookSideEffect) getBody() io.Reader {
	if len(w.w.Form) != 0 {
		return strings.NewReader(w.w.Form.Encode())
	}

	if len(w.w.Body) != 0 {
		return bytes.NewBuffer(w.w.Body)
	}

	return nil
}

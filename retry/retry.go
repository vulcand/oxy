package retry

import (
	"io"
	"net/http"

	"github.com/mailgun/oxy/utils"
)

type Retry struct {
	next        http.Handler
	errHandler  http.Handler
	maxAttempts int
	predicate   hpredicate
}

func New(next http.Handler, predicate string, settings ...optSetter) (*Retry, error) {
	p, err := parseExpression(predicate)
	if err != nil {
		return nil, err
	}

	r := &Retry{
		next:      next,
		predicate: p,
	}
	for _, s := range settings {
		if err := s(r); err != nil {
			return nil, err
		}
	}
	if r.maxAttempts == 0 {
		r.maxAttempts = DefaultMaxAttempts
	}
	return r, nil
}

func (r *Retry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := &context{r: req}
	for i := 0; i < r.maxAttempts; i++ {
		pw := &utils.ProxyWriter{W: w}

		// this is not the first attempt, we need to rewind the body
		if i != 0 {
			seeker, ok := req.Body.(io.Seeker)
			if !ok {
				r.errHandler.ServeHTTP(w, req)
				return
			}
			_, err := seeker.Seek(0, 0)
			if err != nil {
				r.errHandler.ServeHTTP(w, req)
				return
			}
		}
		r.next.ServeHTTP(pw, req)
		c.attempt = i + 1
		if !r.predicate(c) {
			return
		}
	}
	// if we ended up here, we have excceded all the max attempts what is a system error
	r.errHandler.ServeHTTP(w, req)
}

func MaxAttempts(a int) optSetter {
	return func(r *Retry) error {
		r.maxAttempts = a
		return nil
	}
}

func ErrorHandler(h http.Handler) optSetter {
	return func(r *Retry) error {
		r.errHandler = h
		return nil
	}
}

type optSetter func(r *Retry) error

const DefaultMaxAttempts = 10

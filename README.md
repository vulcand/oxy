Oxy
=====

Oxy is a Go library with HTTP handlers that enhance HTTP standard library:

* Stream middleware retries and buffers requests and responses http://godoc.org/github.com/mailgun/oxy/stream
* Forward middleware forwards requests to remote location and rewrites headers http://godoc.org/github.com/mailgun/oxy/stream
* Roundrobin middleware is a round-robin load balancer http://godoc.org/github.com/mailgun/oxy/stream

It is designed to be fully compatible with http standard library, easy to customize and reuse.

Status
------

* Initial design is completed
* Covered by tests
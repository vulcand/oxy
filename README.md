Oxy
=====

Oxy is a Go library with HTTP handlers that enhance HTTP standard library:

* [Stream](http://godoc.org/github.com/mailgun/oxy/stream) middleware retries and buffers requests and responses 
* [Forward](http://godoc.org/github.com/mailgun/oxy/forward) middleware forwards requests to remote location and rewrites headers 
* [Roundrobin](http://godoc.org/github.com/mailgun/oxy/roundrobin) middleware is a round-robin load balancer 

It is designed to be fully compatible with http standard library, easy to customize and reuse.

Status
------

* Initial design is completed
* Covered by tests

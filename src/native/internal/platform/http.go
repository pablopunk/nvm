package platform

import "net/http"

// HTTP is the intent "talk to a network service". Callers set deadlines and cancellation through the
// request context; the port adds no hidden timeouts. Every OS supports it, subject to network state.
type HTTP interface {
	// Do sends a request and returns its response. The caller closes the response body.
	Do(req *http.Request) (*http.Response, error)
}

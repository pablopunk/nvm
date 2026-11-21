package real

import (
	"net/http"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// httpClient sends requests with the standard transport. It sets no timeout: callers bound each request
// through its context.
type httpClient struct {
	client *http.Client
}

// newHTTPClient returns the production HTTP port.
func newHTTPClient() platform.HTTP {
	return httpClient{client: &http.Client{}}
}

// Do sends the request.
func (c httpClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

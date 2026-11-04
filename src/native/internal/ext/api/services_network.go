package api

// Network fetches web resources through the host's public-address policy. It needs the network capability.
// Loopback, private, link-local and reserved destinations are refused, and so is every redirect that leads to
// one. Long requests belong in a data loader (DataLoader), not in a search call, which has a 100 ms hard budget.
type Network interface {
	Fetch(request FetchRequest) (FetchResponse, error)
}

// FetchRequest describes one HTTP request. Method defaults to GET; the allowed methods are GET, HEAD, POST,
// PUT, PATCH and DELETE. TimeoutMs defaults to 10 s and is capped at 30 s. MaxBytes defaults to 1 MiB and is
// capped at 8 MiB; a larger body is a quota failure, not a truncated answer.
type FetchRequest struct {
	Method    string            `json:"method,omitempty"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
	TimeoutMs int64             `json:"timeoutMs,omitempty"`
	MaxBytes  int64             `json:"maxBytes,omitempty"`
}

// FetchResponse is the final response after redirects. Body holds UTF-8 text; a body that is not valid UTF-8
// arrives in BodyBase64 instead, with Body empty.
type FetchResponse struct {
	Status     int               `json:"status"`
	FinalURL   string            `json:"finalUrl"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	BodyBase64 string            `json:"bodyBase64,omitempty"`
}

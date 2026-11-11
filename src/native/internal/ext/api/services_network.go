package api

// Network fetches web resources through the host's public-address policy. It needs the network capability.
// Loopback, private, link-local and reserved destinations are refused, and so is every redirect that leads to
// one. Probe network.fetch for a wired implementation; this grants no network declaration or readiness.
// Calls stop future hops and suppress late results after caller/runtime cancellation or host closure.
// Already-admitted requests may finish externally; they are never retried automatically.
// Long requests belong in a data loader (DataLoader), not in a search call, which has a 100 ms hard budget.
type Network interface {
	Fetch(request FetchRequest) (FetchResponse, error)
	Download(request DownloadRequest) (DownloadResponse, error)
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

// DownloadRequest streams a GET to an absolute path in an existing folder. It requires network and
// desktop.files capabilities. MaxBytes defaults to and is capped at 64 MiB; TimeoutMs defaults to and
// is capped at 90 seconds. Redirects and overwrites are refused unless explicitly enabled. MediaTypes,
// when set, must match both the response Content-Type and the sniffed content. Cancellation before
// publication removes staging; a request already admitted for publication may have an effect.
//
// IndicatorID, when set, makes the host show byte progress on the extension's own indicator of that id: a loading
// indicator titled IndicatorTitle (default "Downloading…") whose subtitle reads "3.2 MB of 8.0 MB", with Value and
// Total set when the server sends a length and only the bytes received otherwise. Updates are throttled to a few per
// second. When the download fails or is cancelled the host hides that indicator; on success it stays for the
// extension to replace with its result. A host that draws no indicators downloads without progress.
type DownloadRequest struct {
	URL             string            `json:"url"`
	Path            string            `json:"path"`
	Headers         map[string]string `json:"headers,omitempty"`
	MaxBytes        int64             `json:"maxBytes,omitempty"`
	TimeoutMs       int64             `json:"timeoutMs,omitempty"`
	Overwrite       bool              `json:"overwrite,omitempty"`
	FollowRedirects bool              `json:"followRedirects,omitempty"`
	MediaTypes      []string          `json:"mediaTypes,omitempty"`
	IndicatorID     string            `json:"indicatorId,omitempty"`
	IndicatorTitle  string            `json:"indicatorTitle,omitempty"`
}

// DownloadResponse reports the final response and the published file, without buffering its body.
type DownloadResponse struct {
	Status   int               `json:"status"`
	FinalURL string            `json:"finalUrl"`
	Headers  map[string]string `json:"headers,omitempty"`
	Bytes    int64             `json:"bytes"`
	File     FileRef           `json:"file"`
}

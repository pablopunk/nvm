package platform

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Fetch sends a GET for url through h and returns the status code and the body. The body is read in full up
// to limit bytes; a longer body is an error, so a hostile server cannot exhaust memory. The caller bounds the
// request through ctx. Feature packages use Fetch instead of building requests themselves, because they may
// not import net/http.
func Fetch(ctx context.Context, h HTTP, url string, limit int64) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := h.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if int64(len(body)) > limit {
		return resp.StatusCode, nil, fmt.Errorf("response body exceeds %d bytes", limit)
	}
	return resp.StatusCode, body, nil
}

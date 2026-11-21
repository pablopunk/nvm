package fake

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func newTestRequest(t *testing.T, ctx context.Context, method, url, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return req
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(data)
}

func TestScriptedResponsesAreServedInOrderAndRequestsRecorded(t *testing.T) {
	h := NewHTTP()
	h.Script(
		Response{Status: http.StatusCreated, Body: []byte("one")},
		Response{Body: []byte("two")},
	)
	first, err := h.Do(newTestRequest(t, context.Background(), http.MethodPost, "https://api.test/a", "payload"))
	if err != nil {
		t.Fatalf("first Do: %v", err)
	}
	if first.StatusCode != http.StatusCreated || readBody(t, first) != "one" {
		t.Fatalf("first response status %d", first.StatusCode)
	}
	second, err := h.Do(newTestRequest(t, context.Background(), http.MethodGet, "https://api.test/b", ""))
	if err != nil {
		t.Fatalf("second Do: %v", err)
	}
	if second.StatusCode != http.StatusOK || readBody(t, second) != "two" {
		t.Fatalf("second response status %d, want the 200 default", second.StatusCode)
	}

	recorded := h.Requests()
	if len(recorded) != 2 || recorded[0].Method != http.MethodPost || string(recorded[0].Body) != "payload" {
		t.Fatalf("recorded = %+v", recorded)
	}
	if recorded[1].URL != "https://api.test/b" {
		t.Fatalf("second recorded URL = %q", recorded[1].URL)
	}
}

func TestScriptedErrorIsReturnedWithoutResponse(t *testing.T) {
	h := NewHTTP()
	boom := errors.New("connection reset")
	h.Script(Response{Err: boom})
	resp, err := h.Do(newTestRequest(t, context.Background(), http.MethodGet, "https://api.test/", ""))
	if !errors.Is(err, boom) || resp != nil {
		t.Fatalf("Do = %v, %v; want the scripted error and no response", resp, err)
	}
}

func TestUnscriptedRequestFailsLoudly(t *testing.T) {
	_, err := NewHTTP().Do(newTestRequest(t, context.Background(), http.MethodGet, "https://api.test/", ""))
	if !errors.Is(err, ErrNoResponse) {
		t.Fatalf("Do without script or handler: %v, want ErrNoResponse", err)
	}
}

func TestCancelledContextFailsBeforeAnswering(t *testing.T) {
	h := NewHTTP()
	h.Script(Response{Body: []byte("unused")})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.Do(newTestRequest(t, ctx, http.MethodGet, "https://api.test/", ""))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do with cancelled context: %v, want context.Canceled", err)
	}
}

func TestHandlerServesRequestsWithoutScript(t *testing.T) {
	h := NewHTTP()
	h.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Route", r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, "short and stout")
	}))
	resp, err := h.Do(newTestRequest(t, context.Background(), http.MethodGet, "https://api.test/tea", ""))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusTeapot || resp.Header.Get("X-Route") != "/tea" || readBody(t, resp) != "short and stout" {
		t.Fatalf("handler response status %d header %v", resp.StatusCode, resp.Header)
	}
}

func TestHandlerThatHangsReturnsTheContextError(t *testing.T) {
	h := NewHTTP()
	started := make(chan struct{})
	h.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	outcome := make(chan error, 1)
	go func() {
		_, err := h.Do(newTestRequest(t, ctx, http.MethodGet, "https://api.test/hang", ""))
		outcome <- err
	}()
	<-started
	cancel()
	if err := <-outcome; !errors.Is(err, context.Canceled) {
		t.Fatalf("hanging Do returned %v, want context.Canceled", err)
	}
}

func TestRecorderDefaultsToOKWhenHandlerWritesNothing(t *testing.T) {
	h := NewHTTP()
	h.Handle(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	resp, err := h.Do(newTestRequest(t, context.Background(), http.MethodGet, "https://api.test/", ""))
	if err != nil || resp.StatusCode != http.StatusOK || readBody(t, resp) != "" {
		t.Fatalf("empty handler: %v, %v", resp, err)
	}
}

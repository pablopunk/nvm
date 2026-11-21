package platform

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// stubHTTP answers every request with one canned response and remembers the last request it saw.
type stubHTTP struct {
	resp *http.Response
	err  error
	last *http.Request
}

func (s *stubHTTP) Do(req *http.Request) (*http.Response, error) {
	s.last = req
	return s.resp, s.err
}

func stubResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func TestFetchSendsGETAndReturnsStatusAndBody(t *testing.T) {
	stub := &stubHTTP{resp: stubResponse(http.StatusCreated, "payload")}
	status, body, err := Fetch(context.Background(), stub, "https://api.test/rate", 64)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if status != http.StatusCreated || string(body) != "payload" {
		t.Fatalf("status %d body %q", status, body)
	}
	if stub.last.Method != http.MethodGet || stub.last.URL.String() != "https://api.test/rate" {
		t.Fatalf("request %s %s", stub.last.Method, stub.last.URL)
	}
}

func TestFetchRejectsBodiesOverTheLimit(t *testing.T) {
	stub := &stubHTTP{resp: stubResponse(http.StatusOK, strings.Repeat("x", 9))}
	if _, _, err := Fetch(context.Background(), stub, "https://api.test/big", 8); err == nil {
		t.Fatal("Fetch accepted a body over the limit")
	}
}

func TestFetchReturnsTransportErrors(t *testing.T) {
	want := errors.New("offline")
	stub := &stubHTTP{err: want}
	if _, _, err := Fetch(context.Background(), stub, "https://api.test/", 8); !errors.Is(err, want) {
		t.Fatalf("Fetch error %v, want %v", err, want)
	}
}

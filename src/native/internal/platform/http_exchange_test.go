package platform

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// scriptedHTTP answers requests in order and records each one.
type scriptedHTTP struct {
	responses []*http.Response
	requests  []*http.Request
	bodies    []string
}

func (s *scriptedHTTP) Do(req *http.Request) (*http.Response, error) {
	var body string
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		body = string(data)
	}
	s.requests = append(s.requests, req)
	s.bodies = append(s.bodies, body)
	next := s.responses[0]
	s.responses = s.responses[1:]
	return next, nil
}

func redirectResponse(status int, location string) *http.Response {
	response := stubResponse(status, "")
	response.Header = http.Header{"Location": {location}}
	return response
}

func TestExchangeReturnsStatusLowerCaseHeadersAndBody(t *testing.T) {
	stub := &scriptedHTTP{responses: []*http.Response{stubResponse(200, "hello")}}
	stub.responses[0].Header = http.Header{"X-Trace": {"a", "b"}}
	result, err := FetchExchange(context.Background(), stub, Exchange{
		Method:   "GET",
		URL:      "https://api.test/items",
		Headers:  map[string]string{"Accept": "text/plain"},
		MaxBytes: 64,
	})
	if err != nil || result.Status != 200 || string(result.Body) != "hello" || result.Headers["x-trace"] != "a, b" {
		t.Fatalf("result = %+v (%v)", result, err)
	}
	if got := stub.requests[0].Header.Get("Accept"); got != "text/plain" {
		t.Fatalf("Accept header = %q", got)
	}
}

func TestExchangeTurnsPostIntoGetOnSeeOther(t *testing.T) {
	stub := &scriptedHTTP{responses: []*http.Response{
		redirectResponse(303, "/done"),
		stubResponse(200, "ok"),
	}}
	result, err := FetchExchange(context.Background(), stub, Exchange{Method: "POST", URL: "https://api.test/submit", Body: []byte("x=1"), MaxBytes: 64, MaxRedirects: 5})
	if err != nil || result.FinalURL != "https://api.test/done" {
		t.Fatalf("result = %+v (%v)", result, err)
	}
	if stub.requests[1].Method != "GET" || stub.bodies[1] != "" {
		t.Fatalf("second hop = %s with body %q; want GET without a body", stub.requests[1].Method, stub.bodies[1])
	}
}

func TestExchangeKeepsMethodAndBodyOn307(t *testing.T) {
	stub := &scriptedHTTP{responses: []*http.Response{
		redirectResponse(307, "https://api.test/next"),
		stubResponse(200, "ok"),
	}}
	if _, err := FetchExchange(context.Background(), stub, Exchange{Method: "POST", URL: "https://api.test/a", Body: []byte("body"), MaxBytes: 64, MaxRedirects: 5}); err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if stub.requests[1].Method != "POST" || stub.bodies[1] != "body" {
		t.Fatalf("second hop = %s %q; want POST with the body", stub.requests[1].Method, stub.bodies[1])
	}
}

func TestExchangeDropsAuthorizationOnCrossHostRedirect(t *testing.T) {
	stub := &scriptedHTTP{responses: []*http.Response{
		redirectResponse(302, "https://other.test/x"),
		stubResponse(200, "ok"),
	}}
	headers := map[string]string{"Authorization": "Bearer t"}
	if _, err := FetchExchange(context.Background(), stub, Exchange{Method: "GET", URL: "https://api.test/a", Headers: headers, MaxBytes: 64, MaxRedirects: 5}); err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if stub.requests[1].Header.Get("Authorization") != "" {
		t.Fatal("Authorization reached another host")
	}
}

func TestExchangeChecksEveryDestination(t *testing.T) {
	stub := &scriptedHTTP{responses: []*http.Response{redirectResponse(302, "file:///etc/passwd")}}
	refused := errors.New("refused")
	_, err := FetchExchange(context.Background(), stub, Exchange{
		Method: "GET", URL: "https://api.test/a", MaxBytes: 64, MaxRedirects: 5,
		Check: func(target *url.URL) error {
			if target.Scheme != "https" {
				return refused
			}
			return nil
		},
	})
	if !errors.Is(err, refused) || len(stub.requests) != 1 {
		t.Fatalf("err = %v with %d requests; want the check's error before a second hop", err, len(stub.requests))
	}
}

func TestExchangeBoundsRedirectsAndBodies(t *testing.T) {
	loop := &scriptedHTTP{responses: []*http.Response{
		redirectResponse(301, "https://api.test/a"),
		redirectResponse(301, "https://api.test/a"),
	}}
	if _, err := FetchExchange(context.Background(), loop, Exchange{Method: "GET", URL: "https://api.test/a", MaxBytes: 64, MaxRedirects: 1}); !errors.Is(err, ErrTooManyRedirects) {
		t.Fatalf("loop err = %v; want ErrTooManyRedirects", err)
	}
	big := &scriptedHTTP{responses: []*http.Response{stubResponse(200, strings.Repeat("a", 9))}}
	if _, err := FetchExchange(context.Background(), big, Exchange{Method: "GET", URL: "https://api.test/a", MaxBytes: 8}); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("big err = %v; want ErrResponseTooLarge", err)
	}
}

package services

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func networkManifest(id string) api.Manifest {
	return manifestFor(id, api.CapabilityNetwork)
}

func fetchArgs(url string) api.FetchRequest {
	return api.FetchRequest{URL: url}
}

func TestFetchReturnsStatusHeadersAndText(t *testing.T) {
	r := newRig(t)
	r.network.Script(fake.Response{Status: 200, Header: http.Header{"Content-Type": {"text/plain"}}, Body: []byte("hello")})
	h := handlerFor(t, r, networkManifest("acme.one"))
	var response api.FetchResponse
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, fetchArgs("https://example.com/a"))), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Status != 200 || response.Body != "hello" || response.Headers["content-type"] != "text/plain" {
		t.Fatalf("response = %+v", response)
	}
	recorded := r.network.Requests()
	if len(recorded) != 1 || recorded[0].Method != http.MethodGet || recorded[0].Header.Get("User-Agent") == "" {
		t.Fatalf("recorded = %+v; want one GET with a user agent", recorded)
	}
}

func TestBinaryBodyComesBackAsBase64(t *testing.T) {
	r := newRig(t)
	r.network.Script(fake.Response{Status: 200, Body: []byte{0xff, 0xfe, 0x00}})
	h := handlerFor(t, r, networkManifest("acme.one"))
	var response api.FetchResponse
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, fetchArgs("https://example.com/bin"))), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(response.BodyBase64)
	if err != nil || response.Body != "" || string(decoded) != string([]byte{0xff, 0xfe, 0x00}) {
		t.Fatalf("response = %+v (%v); want the bytes in BodyBase64", response, err)
	}
}

func TestRedirectsAreFollowedOneHopAtATime(t *testing.T) {
	r := newRig(t)
	r.network.Script(
		fake.Response{Status: 302, Header: http.Header{"Location": {"/next"}}},
		fake.Response{Status: 200, Body: []byte("done")},
	)
	h := handlerFor(t, r, networkManifest("acme.one"))
	var response api.FetchResponse
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, fetchArgs("https://example.com/start"))), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Body != "done" || response.FinalURL != "https://example.com/next" {
		t.Fatalf("response = %+v; want the second hop", response)
	}
}

func TestRedirectLoopIsBounded(t *testing.T) {
	r := newRig(t)
	for i := 0; i <= maxFetchRedirects; i++ {
		r.network.Script(fake.Response{Status: 301, Header: http.Header{"Location": {"https://example.com/loop"}}})
	}
	h := handlerFor(t, r, networkManifest("acme.one"))
	requireCode(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, fetchArgs("https://example.com/loop")), api.CodeUnavailable, "redirected")
}

func TestRedirectToAForbiddenSchemeIsRefused(t *testing.T) {
	r := newRig(t)
	r.network.Script(fake.Response{Status: 302, Header: http.Header{"Location": {"file:///etc/passwd"}}})
	h := handlerFor(t, r, networkManifest("acme.one"))
	requireCode(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, fetchArgs("https://example.com/x")), api.CodeUnavailable, "refused")
}

func TestNonPublicDestinationIsRefusedByThePublicPort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("private"))
	}))
	defer server.Close()
	r := newRig(t)
	ports := real.NewPorts()
	h := New(Deps{Files: ports.FS, Clock: r.clock, DataRoot: r.root, Network: ports.PublicHTTP, Logs: r.logs}, networkManifest("acme.one"))
	t.Cleanup(func() { _ = h.Close() })
	requireCode(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, fetchArgs(server.URL)), api.CodePermission, "not public")
}

func TestRequestsThatBreakTheRulesAreInvalid(t *testing.T) {
	h := handlerFor(t, newRig(t), networkManifest("acme.one"))
	bad := map[string]api.FetchRequest{
		"file scheme":       {URL: "file:///etc/hosts"},
		"user name":         {URL: "https://user:pass@example.com/"},
		"no host":           {URL: "https:///path"},
		"unknown method":    {URL: "https://example.com/", Method: "CONNECT"},
		"body on get":       {URL: "https://example.com/", Body: "x"},
		"host header":       {URL: "https://example.com/", Headers: map[string]string{"Host": "other"}},
		"line break header": {URL: "https://example.com/", Headers: map[string]string{"X-A": "a\r\nInjected: yes"}},
		"negative timeout":  {URL: "https://example.com/", TimeoutMs: -1},
	}
	for name, request := range bad {
		t.Run(name, func(t *testing.T) {
			requireCode(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, request), api.CodeInvalid, "")
		})
	}
}

func TestOversizeBodyIsAQuotaFailureNotATruncatedAnswer(t *testing.T) {
	r := newRig(t)
	r.network.Script(fake.Response{Status: 200, Body: []byte(strings.Repeat("a", defaultFetchBytes+1))})
	h := handlerFor(t, r, networkManifest("acme.one"))
	requireCode(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, fetchArgs("https://example.com/big")), api.CodeQuota, "larger")
}

func TestFetchRateIsLimitedPerMinute(t *testing.T) {
	r := newRig(t)
	for i := 0; i < fetchesPerMinute; i++ {
		r.network.Script(fake.Response{Status: 204})
	}
	h := handlerFor(t, r, networkManifest("acme.one"))
	for i := 0; i < fetchesPerMinute; i++ {
		mustOK(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, fetchArgs("https://example.com/")))
	}
	requireCode(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, fetchArgs("https://example.com/")), api.CodeQuota, "too many")
}

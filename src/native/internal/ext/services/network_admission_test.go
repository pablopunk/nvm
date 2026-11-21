package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

type networkAdmissionPort struct {
	calls      int
	stop       func()
	redirect   bool
	closed     bool
	stopOnRead bool
}
type networkAdmissionBody struct {
	io.Reader
	port *networkAdmissionPort
}

func (b *networkAdmissionBody) Read(data []byte) (int, error) {
	if b.port.stopOnRead && b.port.stop != nil {
		b.port.stop()
	}
	return b.Reader.Read(data)
}
func (b *networkAdmissionBody) Close() error { b.port.closed = true; return nil }
func (p *networkAdmissionPort) Do(*http.Request) (*http.Response, error) {
	p.calls++
	if p.stop != nil && !p.stopOnRead {
		p.stop()
	}
	status := 200
	header := http.Header{}
	if p.redirect {
		status = 302
		header.Set("Location", "/next")
	}
	return &http.Response{StatusCode: status, Header: header, Body: &networkAdmissionBody{Reader: strings.NewReader("ok"), port: p}}, nil
}
func TestNetworkAdmissionStopsEffectsAndLateResults(t *testing.T) {
	for _, scenario := range []string{"stopped", "redirect", "late"} {
		t.Run(scenario, func(t *testing.T) {
			p := &networkAdmissionPort{redirect: scenario == "redirect"}
			h := desktopHandler(t, Deps{Network: p}, api.CapabilityNetwork)
			lifetime, stop := context.WithCancel(context.Background())
			defer stop()
			if scenario == "stopped" {
				stop()
			} else {
				p.stop = stop
			}
			_, err := h.fetch(protocol.WithLifetime(context.Background(), lifetime), fetchArgs("https://example.com/start"))
			if err == nil {
				t.Fatal("stopped request succeeded")
			}
			want := 1
			if scenario == "stopped" {
				want = 0
			}
			if p.calls != want {
				t.Fatalf("sends = %d, want %d", p.calls, want)
			}
			if want > 0 && !p.closed {
				t.Fatal("response body was not closed")
			}
		})
	}
}

func TestNetworkAdmissionAbsentPortAndDeclarationHaveNoEffects(t *testing.T) {
	host := desktopHandler(t, Deps{}, api.CapabilityNetwork)
	if host.publicCapability("network.fetch") {
		t.Fatal("absent public port advertised")
	}
	if _, err := host.fetch(context.Background(), fetchArgs("https://example.com/")); err == nil || failureOf(err).Code != api.CodeUnavailable {
		t.Fatalf("missing port: %v", err)
	}
	port := &networkAdmissionPort{}
	host = desktopHandler(t, Deps{Network: port})
	if !host.publicCapability("network.fetch") || port.calls != 0 {
		t.Fatal("support probe checked readiness or declaration")
	}
	raw, err := json.Marshal(fetchArgs("https://example.com/"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.serveNetwork(context.Background(), protocol.ServiceCall{Op: protocol.OpFetch, Args: raw})
	if err == nil || failureOf(err).Code != api.CodePermission || port.calls != 0 {
		t.Fatalf("declaration: %v sends=%d", err, port.calls)
	}
}

func TestNetworkAdmissionClosedHostAndCancelledCallerDoNotSend(t *testing.T) {
	for _, scenario := range []string{"closed", "caller", "deadline"} {
		t.Run(scenario, func(t *testing.T) {
			port := &networkAdmissionPort{}
			host := desktopHandler(t, Deps{Network: port}, api.CapabilityNetwork)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := api.CodeCancelled
			switch scenario {
			case "closed":
				_ = host.Close()
				want = api.CodeUnavailable
			case "caller":
				cancel()
			case "deadline":
				ctx, cancel = context.WithDeadline(context.Background(), time.Time{})
				defer cancel()
				want = api.CodeTimeout
			}
			_, err := host.fetch(ctx, fetchArgs("https://example.com/"))
			if err == nil || failureOf(err).Code != want || port.calls != 0 {
				t.Fatalf("error=%v sends=%d", err, port.calls)
			}
		})
	}
}

func TestNetworkAdmissionStopsRedirectAndBodyReadOnHostOrRuntimeExpiry(t *testing.T) {
	for _, scenario := range []string{"close-redirect", "close-response", "close-body", "runtime-body", "caller-body"} {
		t.Run(scenario, func(t *testing.T) {
			port := &networkAdmissionPort{redirect: scenario == "close-redirect", stopOnRead: strings.HasSuffix(scenario, "body")}
			host := desktopHandler(t, Deps{Network: port}, api.CapabilityNetwork)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			lifetime, stop := context.WithCancel(context.Background())
			defer stop()
			want := api.CodeUnavailable
			switch scenario {
			case "runtime-body":
				port.stop = stop
				want = api.CodeCancelled
			case "caller-body":
				port.stop = cancel
				want = api.CodeCancelled
			default:
				port.stop = func() { _ = host.Close() }
			}
			result, err := host.fetch(protocol.WithLifetime(ctx, lifetime), fetchArgs("https://example.com/"))
			if err == nil || failureOf(err).Code != want || result.Body != "" || port.calls != 1 || !port.closed {
				t.Fatalf("result=%+v err=%v sends=%d closed=%v", result, err, port.calls, port.closed)
			}
		})
	}
}

type networkLifetimePort struct {
	stop                 context.CancelFunc
	calls                int
	observedCancellation bool
}

func (p *networkLifetimePort) Do(request *http.Request) (*http.Response, error) {
	p.calls++
	p.stop()
	select {
	case <-request.Context().Done():
		p.observedCancellation = true
		return nil, request.Context().Err()
	case <-time.After(time.Second):
		return nil, errors.New("runtime cancellation did not reach transport")
	}
}

func TestNetworkAdmissionRuntimeCancellationReachesInflightTransport(t *testing.T) {
	lifetime, stop := context.WithCancel(context.Background())
	defer stop()
	port := &networkLifetimePort{stop: stop}
	host := desktopHandler(t, Deps{Network: port}, api.CapabilityNetwork)
	_, err := host.fetch(protocol.WithLifetime(context.Background(), lifetime), fetchArgs("https://example.com/"))
	if !errors.Is(err, context.Canceled) || failureOf(err).Code != api.CodeCancelled || port.calls != 1 || !port.observedCancellation {
		t.Fatalf("error=%v sends=%d cancelledTransport=%v", err, port.calls, port.observedCancellation)
	}
}

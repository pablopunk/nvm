package services

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

type networkAdmissionPort struct {
	calls    int
	stop     func()
	redirect bool
	closed   bool
}
type networkAdmissionBody struct {
	io.Reader
	port *networkAdmissionPort
}

func (b *networkAdmissionBody) Close() error { b.port.closed = true; return nil }
func (p *networkAdmissionPort) Do(*http.Request) (*http.Response, error) {
	p.calls++
	if p.stop != nil {
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

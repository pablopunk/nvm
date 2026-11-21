package real

import (
	"context"
	"errors"
	"golang.org/x/net/dns/dnsmessage"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

type redirectThenPublic struct {
	target string
	public platform.HTTP
	calls  int
}

func (h *redirectThenPublic) Do(req *http.Request) (*http.Response, error) {
	h.calls++
	if h.calls == 1 {
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{h.target}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	}
	return h.public.Do(req)
}

func TestPublicRedirectToResolvedLoopbackIsBlocked(t *testing.T) {
	var served atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served.Store(true) }))
	defer server.Close()
	target := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	client := &redirectThenPublic{target: target, public: newPublicHTTP()}
	_, err := platform.FetchExchange(context.Background(), client, platform.Exchange{Method: http.MethodGet, URL: "https://public.example/image", MaxBytes: 1024, MaxRedirects: 3})
	if !errors.Is(err, platform.ErrNonPublicAddress) {
		t.Fatalf("redirect error = %v", err)
	}
	if served.Load() || client.calls != 2 {
		t.Fatalf("loopback was contacted: served=%v calls=%d", served.Load(), client.calls)
	}
}

// TestPublicDialRejectsReboundDNS resolves a name as public, then changes its DNS answer to loopback before dialing.
func TestPublicDialRejectsReboundDNS(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var rebound atomic.Bool
	go func() {
		buf := make([]byte, 1500)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			var msg dnsmessage.Message
			if msg.Unpack(buf[:n]) != nil || len(msg.Questions) == 0 {
				continue
			}
			msg.Header.Response = true
			msg.Header.RecursionAvailable = true
			q := msg.Questions[0]
			if q.Type == dnsmessage.TypeA {
				ip := [4]byte{93, 184, 216, 34}
				if rebound.Load() {
					ip = [4]byte{127, 0, 0, 1}
				}
				msg.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 1}, Body: &dnsmessage.AResource{A: ip}}}
			}
			data, err := msg.Pack()
			if err == nil {
				_, _ = conn.WriteTo(data, addr)
			}
		}
	}()
	resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", conn.LocalAddr().String())
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addresses, err := resolver.LookupIPAddr(ctx, "rebind.test")
	if err != nil || len(addresses) == 0 || addresses[0].IP.String() != "93.184.216.34" {
		t.Fatalf("initial lookup = %v, %v", addresses, err)
	}
	rebound.Store(true)
	dialer := net.Dialer{Resolver: resolver, Control: refuseNonPublicDial}
	socket, err := dialer.DialContext(ctx, "tcp", "rebind.test:80")
	if err == nil {
		socket.Close()
		t.Fatal("rebound connection succeeded")
	}
	if !errors.Is(err, platform.ErrNonPublicAddress) {
		t.Fatalf("dial error = %v", err)
	}
}

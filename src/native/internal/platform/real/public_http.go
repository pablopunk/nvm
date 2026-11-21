package real

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// publicDialTimeout bounds one TCP connect of the public client. Request deadlines come from the caller.
const publicDialTimeout = 5 * time.Second

// newPublicHTTP returns the public-address client. Every connection is checked after name resolution: the
// Control hook of the dialer sees the address the socket is about to connect to, so a name that resolves to
// a private address, or changes its answer between two lookups, is refused. The client ignores proxy
// environment variables (a proxy would hide the real destination), follows no redirects (the caller checks
// each hop) and keeps no cookies.
func newPublicHTTP() platform.HTTP {
	dialer := &net.Dialer{Timeout: publicDialTimeout, Control: refuseNonPublicDial}
	transport := &http.Transport{
		Proxy:               nil,
		DialContext:         dialer.DialContext,
		TLSHandshakeTimeout: publicDialTimeout,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        4,
		IdleConnTimeout:     30 * time.Second,
	}
	client := &http.Client{
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return httpClient{client: client}
}

// refuseNonPublicDial is the dialer's Control hook. It runs once per connection attempt with the resolved
// address, and an error stops the connection before any byte is sent.
func refuseNonPublicDial(_, address string, _ syscall.RawConn) error {
	addrPort, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("refusing connection to an unparsed address: %w", err)
	}
	if !platform.IsPublicAddress(addrPort.Addr()) {
		return platform.ErrNonPublicAddress
	}
	return nil
}

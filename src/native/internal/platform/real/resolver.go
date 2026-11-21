package real

import (
	"context"
	"fmt"
	"net"
	"net/netip"
)

// resolver looks host names up through the system resolver.
type resolver struct{}

// LookupAddrs returns every IPv4 and IPv6 address host resolves to. An empty answer is an error.
func (resolver) LookupAddrs(ctx context.Context, host string) ([]netip.Addr, error) {
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no address for %s", host)
	}
	return addrs, nil
}

package platform

import (
	"context"
	"net/netip"
)

// Resolver is the intent "find the addresses of a host name". The public network port checks every
// address before it connects, so a name that resolves to a private address is refused before any request
// is sent. Every OS supports it, subject to network state.
type Resolver interface {
	// LookupAddrs returns every address host resolves to. A host that resolves to nothing is an error.
	LookupAddrs(ctx context.Context, host string) ([]netip.Addr, error)
}

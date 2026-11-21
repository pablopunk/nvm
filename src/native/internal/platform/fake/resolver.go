package fake

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ErrNoHost is returned by Resolver.LookupAddrs for a host the test did not map, so an unexpected lookup
// fails the test instead of reaching a real name server.
var ErrNoHost = errors.New("fake resolver: no address mapped for this host")

// Resolver answers lookups from a table the test fills, and records every host asked for.
//
//	resolver := fake.NewResolver()
//	resolver.Map("cdn.example", "10.0.0.5")
type Resolver struct {
	mu      sync.Mutex
	table   map[string][]netip.Addr
	lookups []string
}

var _ platform.Resolver = (*Resolver)(nil)

// NewResolver returns a resolver with no names.
func NewResolver() *Resolver {
	return &Resolver{table: map[string][]netip.Addr{}}
}

// Map sets the addresses host resolves to. It panics on an address that does not parse, which is a test
// programming error.
func (r *Resolver) Map(host string, addrs ...string) {
	parsed := make([]netip.Addr, 0, len(addrs))
	for _, text := range addrs {
		addr, err := netip.ParseAddr(text)
		if err != nil {
			panic(fmt.Sprintf("fake resolver: %q is not an address", text))
		}
		parsed = append(parsed, addr)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.table[host] = parsed
}

// Lookups returns the hosts asked for, in order.
func (r *Resolver) Lookups() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lookups...)
}

// LookupAddrs returns the mapped addresses of host, or ErrNoHost. A cancelled context fails first.
func (r *Resolver) LookupAddrs(ctx context.Context, host string) ([]netip.Addr, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lookups = append(r.lookups, host)
	addrs, ok := r.table[host]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoHost, host)
	}
	return append([]netip.Addr(nil), addrs...), nil
}

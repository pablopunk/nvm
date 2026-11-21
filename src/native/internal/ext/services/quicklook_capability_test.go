package services

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestQuickLookPublicCapabilityFollowsPlatform(t *testing.T) {
	for _, tc := range []struct {
		name string
		caps []platform.Capability
		want bool
	}{
		{name: "supported", caps: []platform.Capability{platform.CapabilityShellQuickLook}, want: true},
		{name: "unsupported"},
		{name: "unrelated capability", caps: []platform.Capability{platform.CapabilityShellReveal}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, deps := newDesktopHost(t, tc.caps...)
			h := desktopHandler(t, deps)
			var got bool
			err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceSystem, protocol.OpHasCapability, protocol.CapabilityArgs{Name: "quick-look"})), &got)
			if err != nil || got != tc.want {
				t.Fatalf("quick-look = %v (%v), want %v", got, err, tc.want)
			}
		})
	}
}

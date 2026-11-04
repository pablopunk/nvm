package real

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func environment(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestWindowPlacementIsOmittedOnlyInWaylandSessions(t *testing.T) {
	tests := []struct {
		name  string
		goos  string
		env   map[string]string
		wants bool
	}{
		{"macOS", "darwin", nil, true},
		{"Windows", "windows", nil, true},
		{"X11 desktop", "linux", map[string]string{"XDG_SESSION_TYPE": "x11"}, true},
		{"Wayland by session type", "linux", map[string]string{"XDG_SESSION_TYPE": "wayland"}, false},
		{"Wayland by display", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, false},
		{"macOS ignores Wayland variables", "darwin", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := capabilitiesFor(tt.goos, environment(tt.env))
			if got := set.Supported(platform.CapabilityWindowPlacement); got != tt.wants {
				t.Fatalf("window placement supported = %v, want %v", got, tt.wants)
			}
		})
	}
}

package real

import (
	"reflect"
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
			set := capabilitiesFor(tt.goos, environment(tt.env), func(string) bool { return true })
			if got := set.Supported(platform.CapabilityWindowPlacement); got != tt.wants {
				t.Fatalf("window placement supported = %v, want %v", got, tt.wants)
			}
		})
	}
}

func TestFocusReturnNeedsAnX11SessionAndXdotool(t *testing.T) {
	tests := []struct {
		name  string
		goos  string
		env   map[string]string
		tools map[string]bool
		wants bool
	}{
		{"X11 with xdotool", "linux", map[string]string{"XDG_SESSION_TYPE": "x11", "DISPLAY": ":0"}, map[string]bool{"xdotool": true}, true},
		{"X11 tools without a display", "linux", map[string]string{"XDG_SESSION_TYPE": "x11"}, map[string]bool{"xdotool": true}, false},
		{"X11 without xdotool", "linux", nil, nil, false},
		{"Wayland with xdotool", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, map[string]bool{"xdotool": true}, false},
		{"macOS keeps its own focus", "darwin", nil, map[string]bool{"xdotool": true}, false},
		{"Windows keeps its own focus", "windows", nil, map[string]bool{"xdotool": true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasTool := func(name string) bool { return tt.tools[name] }
			set := capabilitiesFor(tt.goos, environment(tt.env), hasTool)
			if got := set.Supported(platform.CapabilityFocusReturn); got != tt.wants {
				t.Fatalf("focus return supported = %v, want %v", got, tt.wants)
			}
		})
	}
}

func TestTextCapabilitiesFollowTheAssetsAndTheScreenTool(t *testing.T) {
	recognize, capture := platform.CapabilityTextRecognize, platform.CapabilityScreenCapture
	cases := []struct {
		hasOCR, canCapture bool
		want               []platform.Capability
	}{
		{false, false, nil},
		{true, false, []platform.Capability{recognize}},
		{false, true, []platform.Capability{capture}},
		{true, true, []platform.Capability{recognize, capture}},
	}
	for _, test := range cases {
		if got := textCapabilities(test.hasOCR, test.canCapture); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("assets=%t capture=%t: got %v, want %v", test.hasOCR, test.canCapture, got, test.want)
		}
	}
}

func TestScreenCaptureIsOnMacOSAndOnLinuxWithATool(t *testing.T) {
	none := func(string) bool { return false }
	scrot := func(name string) bool { return name == "scrot" }
	x11 := func(name string) string {
		if name == "DISPLAY" {
			return ":0"
		}
		return ""
	}
	if !screenCaptureCapable("darwin", x11, none) || screenCaptureCapable("windows", x11, scrot) {
		t.Fatal("macOS always captures and Windows never does")
	}
	if screenCaptureCapable("linux", x11, none) || !screenCaptureCapable("linux", x11, scrot) {
		t.Fatal("Linux captures only with a tool")
	}
}

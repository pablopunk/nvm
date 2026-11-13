package real

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

var (
	x11Env     = map[string]string{"XDG_SESSION_TYPE": "x11", "DISPLAY": ":0"}
	waylandEnv = map[string]string{"XDG_SESSION_TYPE": "wayland", "WAYLAND_DISPLAY": "wayland-1"}
	// xwaylandEnv is a Wayland session that also exports an XWayland DISPLAY: xdotool cannot reach native windows.
	xwaylandEnv = map[string]string{"XDG_SESSION_TYPE": "wayland", "WAYLAND_DISPLAY": "wayland-1", "DISPLAY": ":0"}
)

func toolSet(names ...string) func(string) bool {
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	return func(name string) bool { return set[name] }
}

// TestPasteCapabilityMatrix covers every session type against every paste tool: X11 needs xdotool, Wayland needs
// wtype, and neither borrows the other's tool.
func TestPasteCapabilityMatrix(t *testing.T) {
	tests := []struct {
		name  string
		goos  string
		env   map[string]string
		tools []string
		want  bool
		tool  string
	}{
		{"X11 with xdotool", "linux", x11Env, []string{"xdotool"}, true, "xdotool"},
		{"X11 with wtype only", "linux", x11Env, []string{"wtype"}, false, ""},
		{"X11 with both", "linux", x11Env, []string{"xdotool", "wtype"}, true, "xdotool"},
		{"X11 with neither", "linux", x11Env, nil, false, ""},
		{"Wayland with wtype", "linux", waylandEnv, []string{"wtype"}, true, "wtype"},
		{"Wayland with xdotool only", "linux", waylandEnv, []string{"xdotool"}, false, ""},
		{"Wayland with both", "linux", waylandEnv, []string{"xdotool", "wtype"}, true, "wtype"},
		{"Wayland with neither", "linux", waylandEnv, nil, false, ""},
		{"XWayland with xdotool only", "linux", xwaylandEnv, []string{"xdotool"}, false, ""},
		{"XWayland with wtype", "linux", xwaylandEnv, []string{"wtype"}, true, "wtype"},
		{"macOS needs no tool", "darwin", nil, nil, true, ""},
		{"Windows has no paste", "windows", nil, []string{"wtype", "xdotool"}, false, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hasTool := toolSet(test.tools...)
			if got := pasteCapable(test.goos, environment(test.env), hasTool); got != test.want {
				t.Fatalf("pasteCapable = %v, want %v", got, test.want)
			}
			caps := capabilitiesFor(test.goos, environment(test.env), hasTool)
			if got := caps.Supported(platform.CapabilityClipboardPaste); got != test.want {
				t.Fatalf("clipboard.paste supported = %v, want %v", got, test.want)
			}
			if test.goos == "linux" {
				if got := linuxPasteTool(test.goos, environment(test.env), hasTool); got != test.tool {
					t.Fatalf("linuxPasteTool = %q, want %q", got, test.tool)
				}
			}
		})
	}
}

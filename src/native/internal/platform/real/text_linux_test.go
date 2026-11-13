//go:build linux

package real

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func tools(names ...string) func(string) bool {
	return func(name string) bool { return slices.Contains(names, name) }
}

func TestLinuxScreenCaptureUsesTheToolForTheSession(t *testing.T) {
	wayland := map[string]string{"XDG_SESSION_TYPE": "wayland", "WAYLAND_DISPLAY": "wayland-0"}
	x11 := map[string]string{"DISPLAY": ":0", "XDG_SESSION_TYPE": "x11"}
	tests := []struct {
		name   string
		env    map[string]string
		tools  []string
		binary string
		args   []string
	}{
		{"grim on wlroots", wayland, []string{"grim", "scrot"}, "grim", []string{"/tmp/x.png"}},
		{"gnome on wayland", wayland, []string{"gnome-screenshot", "scrot"}, "gnome-screenshot", []string{"-f", "/tmp/x.png"}},
		{"kde", wayland, []string{"spectacle"}, "spectacle", []string{"-b", "-n", "-f", "-o", "/tmp/x.png"}},
		{"scrot on x11", x11, []string{"scrot", "import"}, "scrot", []string{"-o", "/tmp/x.png"}},
		{"imagemagick on x11", x11, []string{"import"}, "import", []string{"-window", "root", "/tmp/x.png"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			helper := fake.NewHelper()
			helper.Script(test.binary, fake.HelperScript{ExitCode: 0})
			capture := linuxScreenCapture{helper: helper, getenv: env(test.env), hasTool: tools(test.tools...)}
			if err := capture.Capture(context.Background(), "/tmp/x.png", nil); err != nil {
				t.Fatal(err)
			}
			calls := helper.Calls()
			if len(calls) != 1 || calls[0].Binary != test.binary || !slices.Equal(calls[0].Args, test.args) {
				t.Fatalf("calls = %+v", calls)
			}
		})
	}
}

func TestLinuxScreenCaptureRefusesWhatItCannotDo(t *testing.T) {
	x11 := env(map[string]string{"DISPLAY": ":0"})
	cases := map[string]struct {
		capture linuxScreenCapture
		region  *platform.Rect
	}{
		"no tool":    {linuxScreenCapture{helper: fake.NewHelper(), getenv: x11, hasTool: tools()}, nil},
		"no display": {linuxScreenCapture{helper: fake.NewHelper(), getenv: env(nil), hasTool: tools("scrot")}, nil},
		"a region":   {linuxScreenCapture{helper: fake.NewHelper(), getenv: x11, hasTool: tools("scrot")}, &platform.Rect{Width: 1, Height: 1}},
	}
	for name, test := range cases {
		if err := test.capture.Capture(context.Background(), "/tmp/x.png", test.region); !errors.Is(err, platform.ErrUnsupported) {
			t.Errorf("%s: err = %v, want ErrUnsupported", name, err)
		}
	}
}

func TestLinuxScreenCaptureReportsTheToolsFailure(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script("scrot", fake.HelperScript{ExitCode: 1, Stderr: []byte("cannot open display")})
	capture := linuxScreenCapture{helper: helper, getenv: env(map[string]string{"DISPLAY": ":0"}), hasTool: tools("scrot")}
	if err := capture.Capture(context.Background(), "/tmp/x.png", nil); err == nil || err.Error() != "cannot open display" {
		t.Fatalf("err = %v", err)
	}
}

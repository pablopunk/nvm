package real

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestMacMenusContract(t *testing.T) {
	h := fake.NewHelper()
	h.Script("helper", fake.HelperScript{Stdout: []byte(`{"pid":123,"identity":"token"}`)}, fake.HelperScript{Stdout: []byte(`[{"reference":"ref","title":"Save","path":["File"],"shortcut":"Command+S","enabled":true,"checked":false}]`)}, fake.HelperScript{Stdout: []byte(`{"ok":true}`)})
	m := macMenus{macHelper{helper: h, binary: "helper"}}
	source, err := m.Capture(context.Background(), 123)
	if err != nil || source.PID != 123 || source.Identity != "token" {
		t.Fatalf("capture: %+v %v", source, err)
	}
	entries, err := m.Snapshot(context.Background(), source)
	if err != nil || len(entries) != 1 || entries[0].Title != "Save" || !entries[0].Enabled {
		t.Fatalf("snapshot: %+v %v", entries, err)
	}
	if err := m.Invoke(context.Background(), source, entries[0].Reference); err != nil {
		t.Fatal(err)
	}
	calls := h.Calls()
	for i, args := range [][]string{{"menus", "source", "123"}, {"menus", "snapshot", "123", "token"}, {"menus", "invoke", "123", "token", "ref"}} {
		if !reflect.DeepEqual(calls[i].Args, args) {
			t.Fatalf("args: %v", calls[i].Args)
		}
	}
}
func TestMacMenusFailures(t *testing.T) {
	tests := []struct {
		name   string
		script fake.HelperScript
		denied bool
	}{
		{"mismatch", fake.HelperScript{Stdout: []byte(`{"pid":124,"identity":"token"}`)}, false},
		{"empty identity", fake.HelperScript{Stdout: []byte(`{"pid":123,"identity":""}`)}, false},
		{"malformed", fake.HelperScript{Stdout: []byte(`{`)}, false},
		{"denied", fake.HelperScript{ExitCode: 2}, true},
		{"exit", fake.HelperScript{ExitCode: 1, Stdout: []byte(`{"pid":123,"identity":"token"}`)}, false},
		{"oversized", fake.HelperScript{Stdout: []byte(strings.Repeat("x", platform.MaxHelperOutputBytes+1))}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := fake.NewHelper()
			h.Script("helper", tc.script)
			m := macMenus{macHelper{helper: h, binary: "helper"}}
			_, err := m.Capture(context.Background(), 123)
			var e *platform.WindowError
			if !errors.As(err, &e) || (e.Code == platform.WindowErrorDenied) != tc.denied {
				t.Fatalf("error: %v", err)
			}
		})
	}
	h := fake.NewHelper()
	m := macMenus{macHelper{helper: h, binary: "helper"}}
	source := platform.MenuSource{PID: 123, Identity: "token"}
	if m.Invoke(context.Background(), source, "") == nil {
		t.Fatal("accepted empty reference")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(m.Invoke(ctx, source, "ref"), context.Canceled) {
		t.Fatal("cancellation lost")
	}
	if len(h.Calls()) != 0 {
		t.Fatal("unexpected helper invocation")
	}
}

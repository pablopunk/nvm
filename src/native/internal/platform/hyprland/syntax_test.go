package hyprland

import (
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	for in, want := range map[string]Version{"0.55.1": {0, 55, 1}, "v0.53.0": {0, 53, 0}, "v0.45.2-12-gabc": {0, 45, 2}, "0.57": {0, 57, 0}} {
		if got, err := ParseVersion(in); err != nil || got != want {
			t.Errorf("%q: %v %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "git", "1", "a.b.c"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	if v, err := parseVersionJSON([]byte(`{"branch":"","tag":"v0.44.1-5-g1","version":""}`)); err != nil || v != (Version{0, 44, 1}) {
		t.Errorf("tag fallback: %v %v", v, err)
	}
	if v, err := parseVersionJSON([]byte(`{"version":"0.56.0","tag":"v0.56.0"}`)); err != nil || v != (Version{0, 56, 0}) {
		t.Errorf("version field: %v %v", v, err)
	}
}

func TestSupportTable(t *testing.T) {
	cases := []struct {
		v    Version
		f    Format
		want Support
	}{
		{Version{0, 44, 0}, FormatConf, Support{}},
		{Version{0, 50, 0}, FormatConf, Support{Binds: true}},
		{Version{0, 53, 0}, FormatConf, Support{Binds: true, WindowRules: true}},
		{Version{0, 54, 0}, FormatLua, Support{}},
		{Version{0, 55, 0}, FormatLua, Support{Binds: true, WindowRules: true}},
		{Version{0, 57, 2}, FormatConf, Support{Binds: true, WindowRules: true}},
	}
	for _, c := range cases {
		if got := SupportFor(c.v, c.f); got != c.want {
			t.Errorf("%s %s: %+v", c.v, c.f, got)
		}
	}
}

func TestParseAccelerator(t *testing.T) {
	ok := map[string]string{
		"Control+Alt+K":       "CTRL ALT|K",
		"Shift+Command+Space": "SUPER SHIFT|space",
		"CommandOrControl+1":  "CTRL|1",
		"F13":                 "|F13",
		"Alt+Up":              "ALT|Up",
	}
	for in, want := range ok {
		c, err := parseAccelerator(in)
		if err != nil || strings.Join(c.mods, " ")+"|"+c.key != want {
			t.Errorf("%q: %v %v", in, c, err)
		}
	}
	for _, bad := range []string{"", "Alt", "Control+Ä", "Hyper+K", "F25"} {
		if _, err := parseAccelerator(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	c, _ := parseAccelerator("Command+Control+Alt+Shift+K")
	if c.modmask() != 1|4|8|64 {
		t.Errorf("modmask %d", c.modmask())
	}
}

const exe = "/usr/bin/nevermind-next"

func TestRenderConfBindsAndRule(t *testing.T) {
	out, skipped := Render(FormatConf, Version{0, 54, 0}, Desired{
		Binds: []Bind{
			{Accelerator: "Control+Alt+K", Command: exe + " run palette-toggle"},
			{Accelerator: "Alt+N", Command: exe + " run 'x#y'"},
			{Accelerator: "Alt+M", Command: exe + " run '$HOME'"},
		},
		Appearance: Appearance{Undecorated: true, Classes: []string{"nevermind-next", "nvm"}},
	})
	text := string(out)
	for _, want := range []string{
		"bind = CTRL ALT, K, exec, /usr/bin/nevermind-next run palette-toggle\n",
		"windowrule = match:class (nevermind-next|nvm), border_size 0, rounding 0, no_shadow on, no_blur on, decorate off\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
	if len(skipped) != 2 || strings.Contains(text, "x#y") || strings.Contains(text, "$HOME") {
		t.Errorf("unsafe binds written: %v\n%s", skipped, text)
	}
	if !ownedModule(out) {
		t.Error("the module is not recognized as Nevermind's")
	}
}

func TestRenderLuaQuotesEverything(t *testing.T) {
	out, skipped := Render(FormatLua, Version{0, 56, 0}, Desired{
		Binds:      []Bind{{Accelerator: "Command+Shift+N", Command: exe + ` run 'a"b\c$d#e'`}},
		Appearance: Appearance{Undecorated: true, Classes: []string{"nevermind-next"}},
	})
	text := string(out)
	if len(skipped) != 0 {
		t.Fatalf("skipped %v", skipped)
	}
	for _, want := range []string{
		`hl.bind("SUPER + SHIFT + N", hl.dsp.exec_cmd("/usr/bin/nevermind-next run 'a\"b\\c$d#e'"))`,
		`hl.window_rule({ name = "nevermind-undecorated", match = { class = "(nevermind-next)" }, border_size = 0, rounding = 0, no_shadow = true, no_blur = true, decorate = false })`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s in\n%s", want, text)
		}
	}
	if !strings.HasPrefix(text, "-- ") {
		t.Errorf("Lua module starts with %q", text[:3])
	}
}

func TestRenderSkipsWhatTheVersionCannotTake(t *testing.T) {
	_, skipped := Render(FormatConf, Version{0, 50, 0}, Desired{Appearance: Appearance{Undecorated: true, Classes: []string{"nvm"}}})
	if len(skipped) != 1 || !strings.Contains(skipped[0].Reason, "0.53.0") {
		t.Fatalf("skipped %v", skipped)
	}
	_, skipped = Render(FormatConf, Version{0, 54, 0}, Desired{Appearance: Appearance{Undecorated: true}})
	if len(skipped) != 1 {
		t.Fatalf("a rule without an app ID was written: %v", skipped)
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	d := Desired{Binds: []Bind{{"Alt+B", exe + " run b"}, {"Alt+A", exe + " run a"}}}
	a, _ := Render(FormatConf, Version{0, 54, 0}, d)
	d.Binds[0], d.Binds[1] = d.Binds[1], d.Binds[0]
	b, _ := Render(FormatConf, Version{0, 54, 0}, d)
	if string(a) != string(b) {
		t.Fatal("bind order changed the module")
	}
}

func TestLuaQuote(t *testing.T) {
	if got := luaQuote("a\"b\\c\nd\x01é"); got != `"a\"b\\c\010d\001é"` {
		t.Fatalf("%s", got)
	}
}

func TestIncludeRoundTripKeepsUserContent(t *testing.T) {
	user := "# my config\nbind = SUPER, Q, exec, kitty"
	lines := IncludeLines(FormatConf, "/h/.config/hypr/nevermind.conf")
	added := withInclude([]byte(user), lines)
	if !includePresent(FormatConf, added, "/h/.config/hypr/nevermind.conf", "/h") {
		t.Fatal("include not found after adding it")
	}
	removed, ok := withoutInclude(added, lines)
	if !ok || string(removed) != user+"\n" {
		t.Fatalf("removal left %q", removed)
	}
	if _, ok := withoutInclude([]byte(user), lines); ok {
		t.Fatal("removed lines setup never added")
	}
}

func TestIncludeWrittenByTheUserCounts(t *testing.T) {
	if !includePresent(FormatConf, []byte("source = ~/.config/hypr/nevermind.conf # mine\n"), "/h/.config/hypr/nevermind.conf", "/h") {
		t.Error("a user's source line was missed")
	}
	if includePresent(FormatConf, []byte("# source = ~/.config/hypr/nevermind.conf\n"), "/h/.config/hypr/nevermind.conf", "/h") {
		t.Error("a commented-out source line counted")
	}
	if !includePresent(FormatLua, []byte("require('nevermind')\n"), "", "") || !includePresent(FormatLua, []byte(`pcall(require, "nevermind")`), "", "") {
		t.Error("a Lua require was missed")
	}
}

func TestMatchBinds(t *testing.T) {
	c, _ := parseAccelerator("Control+Alt+K")
	ours := []renderedBind{{combo: c, command: exe + " run palette-toggle"}}
	loaded, conflicts := matchBinds(ours, []compositorBind{
		{Modmask: 12, Key: "k", Dispatcher: "exec", Arg: exe + " run palette-toggle"},
		{Modmask: 12, Key: "K", Dispatcher: "exec", Arg: "rofi -show drun"},
		{Modmask: 64, Key: "K", Dispatcher: "exec", Arg: "other"},
	})
	if loaded != LoadedConfirmed || len(conflicts) != 1 || !strings.Contains(conflicts[0], "rofi") {
		t.Fatalf("%s %v", loaded, conflicts)
	}
	if loaded, _ := matchBinds(ours, nil); loaded != LoadedMissing {
		t.Fatalf("%s", loaded)
	}
}

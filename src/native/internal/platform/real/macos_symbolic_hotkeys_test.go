package real

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const hotkeysJSON = `{"AppleSymbolicHotKeys":{
 "30":{"enabled":true,"value":{"parameters":[52,21,1179648],"type":"standard"}},
 "31":{"enabled":false,"value":{"parameters":[52,21,1441792],"type":"standard"}},
 "64":{"enabled":true,"value":{"parameters":[32,49,1048576],"type":"standard"}},
 "79":{"enabled":true,"value":{"parameters":[65535,123,8650752],"type":"standard"}},
 "999":{"enabled":true,"value":{"parameters":[65535,250,0],"type":"standard"}}}}`

func TestParseSymbolicHotkeysKeepsOnlyEnabledEntriesWithAKnownKey(t *testing.T) {
	got, err := parseSymbolicHotkeys([]byte(hotkeysJSON))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]platform.SystemShortcut{}
	for _, shortcut := range got {
		byName[shortcut.Name] = shortcut
	}
	if len(got) != 3 {
		t.Fatalf("got %d shortcuts: %+v", len(got), got)
	}
	shot := byName["Save picture of selected area as a file"]
	if shot.Key != "4" || !shot.Command || !shot.Shift || shot.Alt || shot.Control {
		t.Errorf("screenshot shortcut = %+v, want Command+Shift+4", shot)
	}
	if spotlight := byName["Spotlight search"]; spotlight.Key != "Space" || !spotlight.Command || spotlight.Shift {
		t.Errorf("spotlight = %+v, want Command+Space", spotlight)
	}
	if left := byName["Move left a space"]; left.Key != "Left" || !left.Control || left.Command {
		t.Errorf("move left = %+v, want Control+Left without the function bit", left)
	}
}

func TestEnabledReadsThePreferencesThroughDefaultsAndPlutil(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(defaultsPath, fake.HelperScript{Stdout: []byte("<plist/>")})
	helper.Script(plutilPath, fake.HelperScript{Stdout: []byte(hotkeysJSON)})
	got, err := symbolicHotkeys{helper: helper}.Enabled(context.Background())
	if err != nil || len(got) != 3 {
		t.Fatalf("Enabled() = %v, %v", got, err)
	}
}

func TestEnabledFailsWhenThePreferencesCannotBeRead(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(defaultsPath, fake.HelperScript{ExitCode: 1})
	if _, err := (symbolicHotkeys{helper: helper}).Enabled(context.Background()); err == nil {
		t.Fatal("a failed export was reported as success")
	}
	if _, err := parseSymbolicHotkeys([]byte("not json")); err == nil {
		t.Fatal("garbage parsed")
	}
}

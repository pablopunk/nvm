package shortcuts

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/portable"
)

const aliasConfigPath = "/home/owner/.config/nevermind/config.json"

const aliasSampleConfig = `{
  "version": 1,
  "settings": {},
  "aliases": {
    "extension:nevermind.unknown:thing": ["keep me"]
  },
  "shortcuts": {}
}
`

func openAliasConfig(t *testing.T, files *fake.FS, text string) *Aliases {
	t.Helper()
	if err := files.MkdirAll("/home/owner/.config/nevermind", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile(aliasConfigPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return NewAliases(NewPortableAliases(portable.Open(files, aliasConfigPath)))
}

func configAliases(t *testing.T, files *fake.FS) map[string]any {
	t.Helper()
	data, err := files.ReadFile(aliasConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Aliases map[string]any `json:"aliases"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("config.json is not JSON: %v", err)
	}
	return doc.Aliases
}

// TestAliasesAreWrittenUnderTheOneXKeyAndReadBack stores an alias of Lock Screen. The key is the 1.x one, so 1.x reads
// the same alias, and a second store reads it back.
func TestAliasesAreWrittenUnderTheOneXKeyAndReadBack(t *testing.T) {
	files := fake.NewFS()
	aliases := openAliasConfig(t, files, aliasSampleConfig)
	if err := aliases.Add(CommandLockScreen, "  lock it  "); err != nil {
		t.Fatal(err)
	}
	want := []any{"lock it"}
	if got := configAliases(t, files)["extension:nevermind.system:builtin:lock-screen"]; !slices.Equal(got.([]any), want) {
		t.Fatalf("config aliases = %v, want the 1.x key with %v", got, want)
	}
	reopened := NewAliases(NewPortableAliases(portable.Open(files, aliasConfigPath)))
	if got := reopened.For(CommandLockScreen); !slices.Equal(got, []string{"lock it"}) {
		t.Fatalf("read back = %v", got)
	}
}

// TestRemovingTheLastAliasDropsItsKey checks that an action with no aliases left has no key in config.json.
func TestRemovingTheLastAliasDropsItsKey(t *testing.T) {
	files := fake.NewFS()
	aliases := openAliasConfig(t, files, aliasSampleConfig)
	if err := aliases.Add(CommandSleep, "nap"); err != nil {
		t.Fatal(err)
	}
	if err := aliases.Remove(CommandSleep, "nap"); err != nil {
		t.Fatal(err)
	}
	if _, present := configAliases(t, files)["extension:nevermind.system:builtin:sleep"]; present {
		t.Fatal("the key of a sleep alias with none left is still in config.json")
	}
}

// TestAliasKeysThisBuildDoesNotKnowSurviveAWrite checks that a 1.x alias this build does not bind is kept on write.
func TestAliasKeysThisBuildDoesNotKnowSurviveAWrite(t *testing.T) {
	files := fake.NewFS()
	aliases := openAliasConfig(t, files, aliasSampleConfig)
	if err := aliases.Add(CommandSleep, "nap"); err != nil {
		t.Fatal(err)
	}
	if got := configAliases(t, files)["extension:nevermind.unknown:thing"]; !slices.Equal(got.([]any), []any{"keep me"}) {
		t.Fatalf("unknown alias changed to %v", got)
	}
	if !strings.Contains(string(mustRead(t, files)), `"extension:nevermind.unknown:thing": [`) {
		t.Fatal("the unknown alias lost its place in the file")
	}
}

// TestAddIgnoresBlankAndRepeatedPhrases checks that a blank phrase saves nothing and a repeat keeps one copy.
func TestAddIgnoresBlankAndRepeatedPhrases(t *testing.T) {
	aliases := NewAliases(NewMemoryAliases())
	for _, phrase := range []string{"", "   ", "nap", "nap"} {
		if err := aliases.Add(CommandSleep, phrase); err != nil {
			t.Fatal(err)
		}
	}
	if got := aliases.For(CommandSleep); !slices.Equal(got, []string{"nap"}) {
		t.Fatalf("aliases = %v, want one nap", got)
	}
}

func mustRead(t *testing.T, files *fake.FS) []byte {
	t.Helper()
	data, err := files.ReadFile(aliasConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

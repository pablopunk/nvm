package fixtures

import (
	"bytes"
	"image/png"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestBuildersAreDeterministic(t *testing.T) {
	tests := []struct {
		name  string
		build func() any
	}{
		{name: "apps", build: func() any { return Apps(DefaultAppCount) }},
		{name: "files", build: func() any { return Files(0) }},
		{name: "clipboard entries", build: func() any { return ClipboardEntries(60) }},
		{name: "chats", build: func() any { return Chats(9) }},
		{name: "stream frames", build: func() any { return StreamFrames(FixtureReply, 24) }},
		{name: "malformed frames", build: func() any { return MalformedStreamFrames(FixtureReply) }},
		{name: "mid-stream error frames", build: func() any { return MidStreamErrorFrames(FixtureReply) }},
		{name: "extension sources", build: func() any { return ExtensionSources() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, second := tt.build(), tt.build()
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("two calls produced different output")
			}
		})
	}
}

func TestFilesDefaultsToFiveThousandRealisticPaths(t *testing.T) {
	paths := Files(0)
	if len(paths) != DefaultFileCount {
		t.Fatalf("Files(0) returned %d paths, want %d", len(paths), DefaultFileCount)
	}
	seen := map[string]bool{}
	var hidden, ignored, deep, withExtension int
	for _, path := range paths {
		if seen[path] {
			t.Fatalf("duplicate path %q", path)
		}
		seen[path] = true
		if !strings.HasPrefix(path, "/home/user/") {
			t.Fatalf("path %q is not under the fixture home", path)
		}
		if strings.Contains(path, "/.") {
			hidden++
		}
		if strings.Contains(path, "/node_modules/") || strings.Contains(path, "/.git/") {
			ignored++
		}
		if strings.Count(path, "/") >= 8 {
			deep++
		}
		if strings.Contains(path[strings.LastIndex(path, "/"):], ".") {
			withExtension++
		}
	}
	for label, count := range map[string]int{"hidden": hidden, "ignored": ignored, "deep": deep, "with extension": withExtension} {
		if count == 0 {
			t.Fatalf("the default tree has no %s paths", label)
		}
	}
}

func TestFilesHonorsRequestedCount(t *testing.T) {
	if got := len(Files(123)); got != 123 {
		t.Fatalf("Files(123) returned %d paths", got)
	}
}

func TestSeedFilesCreatesEveryPathInFakeFS(t *testing.T) {
	fsys := fake.NewFS()
	paths := Files(200)
	if err := SeedFiles(fsys, paths); err != nil {
		t.Fatalf("SeedFiles: %v", err)
	}
	for _, path := range paths[:10] {
		if _, err := fsys.Stat(path); err != nil {
			t.Fatalf("Stat(%q) after seeding: %v", path, err)
		}
	}
}

func TestApps(t *testing.T) {
	apps := Apps(3)
	if len(apps) != 3 {
		t.Fatalf("Apps(3) returned %d", len(apps))
	}
	for i, app := range apps {
		if app.App.Name == "" || app.App.Path == "" || app.App.ID == "" {
			t.Fatalf("app %d has an empty field: %+v", i, app.App)
		}
		decoded, err := png.Decode(bytes.NewReader(app.Icon))
		if err != nil {
			t.Fatalf("icon of %s is not PNG: %v", app.App.Name, err)
		}
		if decoded.Bounds().Dx() != appIconSize {
			t.Fatalf("icon width %d, want %d", decoded.Bounds().Dx(), appIconSize)
		}
		if i > 0 && apps[i-1].App.Name > app.App.Name {
			t.Fatalf("apps are not sorted by name: %q before %q", apps[i-1].App.Name, app.App.Name)
		}
	}
	if got := Apps(DefaultAppCount + 2); got[len(got)-1].App.Name == "" {
		t.Fatal("apps past the name list are unnamed")
	}
}

func TestClipboardEntriesIncludeImagesAndConcealedSecrets(t *testing.T) {
	entries := ClipboardEntries(77)
	var images, concealed, text int
	for i, entry := range entries {
		if !entry.At.Equal(clipboardEpoch.Add(time.Duration(i) * time.Minute)) {
			t.Fatalf("entry %d time %v is not one minute after the previous", i, entry.At)
		}
		switch {
		case entry.PNG != nil:
			images++
		case entry.Concealed:
			concealed++
		default:
			text++
		}
	}
	if images == 0 || concealed == 0 || text == 0 {
		t.Fatalf("images %d concealed %d text %d: every kind must appear", images, concealed, text)
	}
}

func TestChatsAreMostRecentFirstWithAnAnswer(t *testing.T) {
	chats := Chats(8)
	for i, chat := range chats {
		if i > 0 && !chat.Updated.Before(chats[i-1].Updated) {
			t.Fatalf("chat %d is not older than chat %d", i, i-1)
		}
		if len(chat.Messages) != 2 || chat.Messages[1].Content != FixtureReply {
			t.Fatalf("chat %s messages = %+v", chat.ID, chat.Messages)
		}
	}
}

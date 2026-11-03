package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search/usage"
)

// goldenNow is the instant every golden reads as "now", so relative times ("3m ago") never drift.
var goldenNow = time.Date(2026, time.November, 3, 10, 0, 0, 0, time.UTC)

// goldenConfig is the config directory the goldens show. Nothing is written there: no fixture saves.
const goldenConfig = "/nvm-fixture/config/nvm-native"

// goldenPalette returns the palette built from fixtures alone. It holds the stores, apps and icons in
// memory and fixes the clock and the config directory, so no golden reads or writes the disk or depends
// on the machine it runs on. It does what ready does for the view (the stack and a first search) and
// nothing else: no window, hotkey, tray, clipboard watcher or file index.
func goldenPalette(t *testing.T) *App {
	t.Helper()
	t.Setenv("NVM_THEME", "")
	t.Setenv("NVM_TOKEN", "")
	t.Setenv("HOME", t.TempDir())
	swapForGolden(t)
	p := &App{
		S: &Settings{
			Hotkey: "Alt+Shift+Space", Engine: "google", ClipLimit: 200,
			Disabled: map[string]bool{},
		},
		learn: goldenUsage(),
		clip:  goldenClipboard(t),
		snips: &SnippetStore{Items: goldenSnippets()},
		notes: &NoteStore{Items: goldenNotes()},
		chats: &ChatStore{Chats: goldenChats()},
		files: &FileIndex{},
		icons: map[string]*ui.Bitmap{},
		apps:  goldenApps(),
	}
	for _, app := range p.apps {
		p.icons[app.Path] = goldenBitmap(t, 56, 56, goldenAppColor(app.Name))
	}
	p.attachServices(fake.NewSet().Ports(), goldenConfig, time.UTC)
	p.newPalette(searchClock, true)
	p.SetStack(p.rootView())
	p.SetQuery("")
	return p
}

// goldenAppColor gives each fixture app its own icon color, so icon tiles differ in the goldens.
func goldenAppColor(name string) color.NRGBA {
	return color.NRGBA{R: uint8(40 + 7*len(name)), G: 110, B: uint8(200 - 5*len(name)), A: 255}
}

// swapForGolden points configDir and clock at their fixtures for the rest of the test.
func swapForGolden(t *testing.T) {
	t.Helper()
	previousConfig, previousClock := configDir, clock
	configDir, clock = goldenConfigDirectory, goldenClockNow
	t.Cleanup(func() { configDir, clock = previousConfig, previousClock })
}

func goldenConfigDirectory() string { return goldenConfig }

// goldenUsage is the learned state the goldens and the root table read: Firefox used three times and Terminal
// once, so the apps listed at the root are the ones used before. Nothing is written anywhere.
func goldenUsage() *usage.Learner {
	learn := usage.NewMemory(fake.NewClock())
	for range 3 {
		learn.Record("app:org.mozilla.firefox", "")
	}
	learn.Record("app:org.gnome.Terminal.desktop", "")
	return learn
}

func goldenClockNow() time.Time { return goldenNow }

// goldenBitmap is a solid image with a lighter block, decoded as the palette decodes real images.
func goldenBitmap(t *testing.T, width, height int, fill color.NRGBA) *ui.Bitmap {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	for y := height / 4; y < height/2; y++ {
		for x := width / 4; x < width/2; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 250, G: 248, B: 243, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	bm, err := ui.DecodeBitmap(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return bm
}

func goldenClipboard(t *testing.T) *ClipStore {
	t.Helper()
	shot := goldenBitmap(t, 640, 360, color.NRGBA{R: 96, G: 128, B: 160, A: 255})
	cs := &ClipStore{
		Entries: []ClipEntry{
			{ID: "e-notes", Kind: "text", Text: "Meeting notes: ship the design package by Friday and review the goldens with Ana", At: goldenNow.Add(-2 * time.Hour), Pinned: true},
			{ID: "e-rebase", Kind: "text", Text: "git rebase -i HEAD~3", At: goldenNow.Add(-3 * time.Minute)},
			{ID: "e-shot", Kind: "image", Image: "e-shot.png", W: 640, H: 360, At: goldenNow.Add(-40 * time.Minute)},
			{ID: "e-link", Kind: "text", Text: "https://nevermind.app/docs/palette", At: goldenNow.Add(-26 * time.Hour)},
		},
		thumbs: map[string]*ui.Bitmap{"e-shot": shot},
	}
	cs.full.id, cs.full.bm = "e-shot", shot
	return cs
}

func goldenApps() []mygo.InstalledApp {
	return []mygo.InstalledApp{
		{Name: "Calculator", ID: "org.gnome.Calculator.desktop", Path: "/usr/share/applications/org.gnome.Calculator.desktop"},
		{Name: "Calendar", ID: "org.gnome.Calendar.desktop", Path: "/usr/share/applications/org.gnome.Calendar.desktop"},
		{Name: "Clocks", ID: "org.gnome.clocks.desktop", Path: "/usr/share/applications/org.gnome.clocks.desktop"},
		{Name: "Firefox", ID: "org.mozilla.firefox", Path: "/usr/share/applications/firefox.desktop"},
		{Name: "Terminal", ID: "org.gnome.Terminal.desktop", Path: "/usr/share/applications/org.gnome.Terminal.desktop"},
	}
}

func goldenSnippets() []Snippet {
	return []Snippet{
		{ID: "s-sig", Name: "Email signature", Keyword: ";sig", Text: "Pablo\nNevermind"},
		{ID: "s-date", Name: "Standup update", Keyword: "", Text: "Yesterday: goldens. Today: design package. Blockers: none."},
	}
}

func goldenNotes() []Note {
	return []Note{
		{ID: "n-ideas", Text: "Ideas\nPalette chips for filters", Updated: goldenNow.Add(-90 * time.Minute)},
	}
}

func goldenChats() []*Chat {
	return []*Chat{
		{ID: "c-release", Title: "Release notes", Updated: goldenNow.Add(-5 * time.Hour)},
		{ID: "c-tokyo", Title: "Time zones", Updated: goldenNow.Add(-26 * time.Hour)},
	}
}

// goldenQuery sets the query and runs the search, as typing does.
func goldenQuery(p *App, q string) {
	p.SetQuery(q)
}

// goldenOpen shows view in a sub-level above the root, as Push does, with an empty query.
func goldenOpen(p *App, view *palette.View) {
	p.SetStack(p.rootView(), view)
	goldenQuery(p, "")
}

// goldenChatView opens a chat with a fixed transcript. When streaming, the last reply is still arriving.
func goldenChatView(p *App, streaming bool) *palette.View {
	ch := &Chat{
		ID: "c-stream", Title: "Release notes", Updated: goldenNow,
		Messages: []ChatMsg{
			{Role: "user", Content: "Summarize what changed in the palette this week."},
			{Role: "assistant", Content: "## This week\n\n- Clipboard history shows **previews** for images\n- Search ranks *apps* above files\n- `Command+K` keeps focus in the palette"},
		},
	}
	if streaming {
		ch.Messages = append(ch.Messages, ChatMsg{Role: "user", Content: "And what is next?"}, ChatMsg{Role: "assistant", Content: "Next up: **snippets** with keywords and a faster emoji grid"})
	}
	view := p.chatView(ch)
	p.runs[ch.ID].streaming = streaming
	return view
}

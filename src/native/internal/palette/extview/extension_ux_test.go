package extview

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// wallpaperPreview is the view the Online Wallpapers extension answers Preview with: the full-size image, a line of text
// and the apply actions.
func wallpaperPreview() api.View {
	return api.View{Kind: api.ViewPreview, Title: "Wallhaven · lygo7y", Actions: []api.Action{api.OpenURL("View on Wallhaven", "https://wallhaven.cc/w/lygo7y")},
		Preview: &api.PreviewSpec{Image: &api.Image{Src: "https://w.wallhaven.cc/full/ly/wallhaven-lygo7y.jpg"}, Text: "3840x1646 · Full-resolution preview from Wallhaven"}}
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	img.Set(1, 1, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestPreviewShowsItsImageAndText opens an extension's preview view in a real palette: the text is there at once, the
// image is announced as loading until the host has decoded it, and then the loading note is gone.
func TestPreviewShowsItsImageAndText(t *testing.T) {
	log := &invokeLog{}
	deps := log.deps(fake.NewFS())
	var finish func(*ui.Bitmap)
	deps.LoadImage = func(_ context.Context, _ string, done func(*ui.Bitmap)) { finish = done }
	session := NewSession(deps, "local.online-wallpapers", wallpaperPreview())
	p := palette.New(palette.Options{Root: session.previewPage(), Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	p.Show()
	tt := ui.NewTester(p.View, 900, 600)
	tt.Frame()
	if !tt.HasText("3840x1646 · Full-resolution preview from Wallhaven") {
		t.Fatalf("preview text missing; texts = %v", tt.Texts())
	}
	if !tt.HasText("Loading the image…") || finish == nil {
		t.Fatalf("a preview must start loading its image; texts = %v", tt.Texts())
	}
	bitmap, err := ui.DecodeBitmap(tinyPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	finish(bitmap)
	tt.Frame()
	if tt.HasText("Loading the image…") || tt.HasText("This image could not be loaded.") {
		t.Fatalf("the loaded image still shows a note; texts = %v", tt.Texts())
	}
}

// TestCollectionFilterChangesTheRowsOfTheOpenGrid is the Online Wallpapers filter: choosing another collection in the
// accessory menu replaces the grid's rows in place, from the keyboard as well as the pointer.
func TestCollectionFilterChangesTheRowsOfTheOpenGrid(t *testing.T) {
	feeds := []api.ViewFilterOption{{Title: "Top this month", Value: "top"}, {Title: "Minimal", Value: "minimal"}}
	view := func(value string, title string) api.View {
		return api.View{Kind: api.ViewList, Title: "Wallpapers", Items: []api.Item{{ID: value, Title: title}},
			Filter: &api.ViewFilter{Value: value, Options: feeds, OnChange: &api.ActionRef{ID: "change-collection"}}}
	}
	minimal := view("minimal", "Quiet Dunes")
	log := &invokeLog{results: map[string]api.Result{"change-collection": {View: &minimal, Navigation: api.NavReplace}}}
	session := NewSession(log.deps(fake.NewFS()), "local.online-wallpapers", view("top", "Loud Peaks"))
	page := session.listPage()
	p := palette.New(palette.Options{Root: page, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	p.Show()
	tt := ui.NewTester(p.View, 720, 470)
	tt.Frame()
	if !tt.HasText("Loud Peaks") {
		t.Fatalf("first collection missing; texts = %v", tt.Texts())
	}
	if err := tt.Click("Top this month"); err != nil {
		t.Fatalf("open the filter: %v", err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem("Minimal"); err != nil {
		t.Fatalf("choose Minimal: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Quiet Dunes") || tt.HasText("Loud Peaks") {
		t.Fatalf("the filter did not change the rows; texts = %v", tt.Texts())
	}
}

// TestEmptyListWithPageActionsShowsTheActionsHint is the Snippets extension before any snippet exists: the list is
// empty, and its only way forward is the Add Snippet page action, so the footer must say the action menu is there.
func TestEmptyListWithPageActionsShowsTheActionsHint(t *testing.T) {
	view := api.ListView("Snippets")
	view.EmptyView = &api.EmptyState{Title: "No snippets yet", Subtitle: "Add a snippet to save a piece of text."}
	view.ActionPanel = api.Panel(api.Section("Snippets", api.Run("add", "Add Snippet", nil)))
	log := &invokeLog{}
	session := NewSession(log.deps(fake.NewFS()), "local.text-snippets", view)
	root := &palette.View{Title: "Root", Search: func(string) []search.Item { return nil }}
	p := palette.New(palette.Options{Root: root, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	p.Show()
	p.Push(session.listPage())
	tt := ui.NewTester(p.View, 720, 470)
	tt.Frame()
	if !tt.HasText("No snippets yet") || !tt.HasText(palette.KeyLabel("⌃K Actions")) {
		t.Fatalf("empty list texts = %v, want the empty state and the Actions hint", tt.Texts())
	}
}

// TestFileRowsRevealOnCommandEnter is the standard of palette results that are files: Command+Enter reveals the file in
// the file manager. A row with a File gets the action added; a row that already has a reveal action takes the key for it;
// a row that binds the key itself keeps its own binding.
func TestFileRowsRevealOnCommandEnter(t *testing.T) {
	session := NewSession(Deps{RevealTitle: "Show in Finder"}, "acme.files", api.View{})
	reveals := func(row search.Item) []search.Action {
		var out []search.Action
		for _, action := range row.Actions {
			if action.Key == ui.KeyEnter && action.Mods == palette.Mod {
				out = append(out, action)
			}
		}
		return out
	}

	added := session.row(api.Item{ID: "a", Title: "photo.png", File: &api.FileRef{Path: "/Users/p/photo.png"}})
	if got := reveals(added); len(got) != 1 || got[0].Title != "Show in Finder" {
		t.Fatalf("a file row's Command+Enter actions = %+v, want one named Show in Finder", got)
	}

	own := session.row(api.Item{ID: "b", Title: "b.png", File: &api.FileRef{Path: "/b.png"}, Actions: []api.Action{api.RevealPath("Reveal in Finder", "/b.png")}})
	if got := reveals(own); len(got) != 1 || got[0].Title != "Reveal in Finder" {
		t.Fatalf("a row with its own reveal action has %+v on Command+Enter, want that action and no second one", got)
	}

	bound := api.RevealPath("Reveal", "/c.png")
	bound.Shortcut = "CommandOrControl+Enter"
	other := api.Action{Kind: api.ActionCopyText, Title: "Copy", Text: "x"}
	keep := session.row(api.Item{ID: "c", Title: "c.png", File: &api.FileRef{Path: "/c.png"}, Actions: []api.Action{other, bound}})
	if got := reveals(keep); len(got) != 1 || got[0].Title != "Reveal" {
		t.Fatalf("a row that binds the key itself has %+v on it, want only its own action", got)
	}

	plain := session.row(api.Item{ID: "d", Title: "not a file"})
	if got := reveals(plain); len(got) != 0 {
		t.Fatalf("a row with no file has %+v on Command+Enter, want none", got)
	}
}

// TestFilterAnswerThatNamesAnotherSourceLoadsItsRows is the GitHub Issues filter: each choice answers with a view whose
// rows come from its own loader. The open list must load from the new loader, not show the empty state of a view that
// carries no rows of its own.
func TestFilterAnswerThatNamesAnotherSourceLoadsItsRows(t *testing.T) {
	loaded := map[string][]api.Item{
		"issues:all":      {{ID: "a", Title: "Repo issue"}},
		"issues:authored": {{ID: "b", Title: "My issue"}},
	}
	options := []api.ViewFilterOption{{Title: "My repos", Value: "all"}, {Title: "Authored by me", Value: "authored"}}
	view := func(value string) api.View {
		list := api.ListView("Issues")
		list.Source = &api.DataSource{Loader: "issues:" + value}
		list.Filter = &api.ViewFilter{Value: value, Options: options, OnChange: &api.ActionRef{ID: "change"}}
		return list
	}
	authored := view("authored")
	log := &invokeLog{results: map[string]api.Result{"change": {View: &authored, Navigation: api.NavReplace}}}
	deps := log.deps(fake.NewFS())
	deps.Load = func(_, loader string, done func([]api.Item, *api.SnapshotEvidence, *api.Failure)) {
		done(loaded[loader], nil, nil)
	}
	session := NewSession(deps, "local.github-issues", view("all"))
	session.listPage()
	session.start()
	if got := rowTitles(session.rows("")); len(got) != 1 || got[0] != "Repo issue" {
		t.Fatalf("first rows = %v, want the repo issue", got)
	}

	session.stepCategory(1)
	if got := rowTitles(session.rows("")); len(got) != 1 || got[0] != "My issue" {
		t.Fatalf("rows after choosing Authored by me = %v, want the authored issue, not an empty list", got)
	}
}

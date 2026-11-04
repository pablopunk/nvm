package main

import (
	"errors"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.views is the dev fixture of every view type the host draws and of the view behaviours around them: data
// sources, pagination, refresh, the search accessory, selection, disabled rows, shortcuts, images and navigation. Each
// command opens one view; RunAction answers with it, so the fixture exercises the same path a real extension takes.

// tinyPNG is a 1x1 PNG as a data URL, the image the fixture's rows and details show.
const tinyPNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{
		{ID: "list", Title: "List fixture"},
		{ID: "sections", Title: "Sections fixture"},
		{ID: "grid", Title: "Grid fixture"},
		{ID: "collection", Title: "Collection fixture"},
		{ID: "preview", Title: "Preview fixture"},
		{ID: "progress", Title: "Progress fixture"},
		{ID: "file", Title: "File preview fixture"},
		{ID: "source", Title: "Source fixture"},
		{ID: "failing", Title: "Failing source fixture"},
		{ID: "empty", Title: "Empty fixture"},
		{ID: "selected", Title: "Selected fixture"},
		{ID: "shortcuts", Title: "Shortcut fixture"},
		{ID: "images", Title: "Image fixture"},
		{ID: "image-file", Title: "Image file fixture"},
		{ID: "confirm", Title: "Confirm fixture"},
		{ID: "root", Title: "Root fixture"},
		{ID: "search-results", Title: "Grid search results fixture"},
	}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "list":
		return api.Result{View: &listView}, nil
	case "sections":
		return api.Result{View: &sectionsView}, nil
	case "grid":
		return api.Result{View: &gridView}, nil
	case "search-results":
		return api.Result{View: &searchResultsView}, nil
	case "search-meaning":
		return api.Result{Patch: &api.ViewPatch{SearchResultIds: meaningMatches(in.Value)}}, nil
	case "collection":
		return api.Result{View: &collectionView}, nil
	case "preview":
		return api.Result{View: &previewView}, nil
	case "progress":
		return api.Result{View: &progressView}, nil
	case "file":
		return api.Result{View: fileView(in.Ref.Args["path"], ".txt", "")}, nil
	case "image-file":
		return api.Result{View: fileView(in.Ref.Args["path"], ".png", "image")}, nil
	case "source":
		return api.Result{View: &sourceView}, nil
	case "failing":
		return api.Result{View: &failingView}, nil
	case "empty":
		return api.Result{View: &emptyView}, nil
	case "selected":
		return api.Result{View: &selectedView}, nil
	case "shortcuts":
		return api.Result{View: &shortcutView}, nil
	case "images":
		return api.Result{View: &imagesView}, nil
	case "confirm":
		return api.Result{View: &confirmView}, nil
	case "root":
		return api.Result{View: &listView, Navigation: api.NavRoot}, nil
	case "reviewed":
		return api.Result{Patch: &api.ViewPatch{Items: []api.Item{reviewedAlpha}}}, nil
	case "more":
		return api.Result{Patch: &api.ViewPatch{Mode: api.PatchAppend, Items: moreNotes}}, nil
	case "tick":
		return api.Result{Patch: &api.ViewPatch{Items: []api.Item{refreshedNote}}}, nil
	case "filter":
		return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: filtered(in.Value)}}, nil
	case "shortcut-run":
		return api.Result{}, nil
	}
	return api.Result{}, nil
}

// LoadItems answers the data sources: "notes" loads two notes, "broken" fails.
func LoadItems(ctx api.Context, loader string) ([]api.Item, error) {
	if loader == "broken" {
		return nil, errors.New("the notes service is offline")
	}
	return []api.Item{
		{ID: "n1", Title: "Standup notes", Subtitle: "Monday", Icon: "file-text"},
		{ID: "n2", Title: "Release plan", Subtitle: "Draft", Icon: "file-text"},
	}, nil
}

var alphaDetail = api.Detail{
	Title:    "Alpha note",
	Subtitle: "Draft",
	Markdown: "# Alpha\n\nFirst paragraph with **bold** text.\n\n- one\n- two",
	Metadata: []api.Metadata{
		{Kind: api.MetadataText, Label: "Folder", Value: "/notes"},
		{Kind: api.MetadataSeparator},
		{Kind: api.MetadataLink, Label: "Source", Value: "example.com", URL: "https://example.com"},
	},
}

var alpha = api.Item{
	ID: "alpha", Title: "Alpha note", Subtitle: "Updated two days ago", Icon: "file-text",
	Accessories: []api.Accessory{{Text: "Draft", Tone: api.ToneWarning}, {Text: "2 KB"}},
	Detail:      &alphaDetail,
	Primary:     &api.Action{Kind: api.ActionRun, Title: "Open", Ref: &api.ActionRef{ID: "open"}},
	Actions: []api.Action{
		api.Run("reviewed", "Mark as reviewed", nil),
		api.CopyText("Copy title", "Alpha note", false),
	},
}

var reviewedAlpha = api.Item{
	ID: "alpha", Title: "Alpha note", Subtitle: "Reviewed", Icon: "file-text",
	Accessories: []api.Accessory{{Text: "Reviewed", Tone: api.ToneSuccess}},
}

var listView = api.View{
	Kind: api.ViewList, Title: "Notes", Placeholder: "Search notes",
	Items: []api.Item{
		alpha,
		{ID: "beta", Title: "Beta task", Subtitle: "Due Friday", Icon: "check",
			Accessories: []api.Accessory{{Text: "Done", Tone: api.ToneSuccess}}},
		{ID: "gamma", Title: "Gamma link", Subtitle: "https://example.com", Icon: "link",
			Primary: api.Primary(api.OpenURL("Open", "https://example.com"))},
	},
}

var sectionsView = api.View{
	Kind: api.ViewList, Title: "Tasks",
	Sections: []api.ItemSection{
		{Title: "Open", Items: []api.Item{{ID: "t1", Title: "Write the report", Icon: "pencil"}}},
		{Title: "Done", Items: []api.Item{{ID: "t2", Title: "Send the invoice", Icon: "check"}}},
	},
}

var gridView = api.View{
	Kind: api.ViewGrid, Title: "Emoji", Grid: &api.GridSpec{Columns: 3},
	Items: []api.Item{
		{ID: "wave", Title: "Wave", Glyph: "👋"},
		{ID: "tada", Title: "Party", Glyph: "🎉"},
		{ID: "rocket", Title: "Launch", Icon: "rocket"},
		{ID: "leaf", Title: "Leaf", Glyph: "🍃"},
	},
}

var collectionView = api.CollectionView("Snippets",
	&api.Action{Kind: api.ActionRun, Title: "New snippet", Ref: &api.ActionRef{ID: "create"}},
	api.CrudItem{
		ID: "standup", Title: "Standup notes", Subtitle: "Monday", Icon: "file-text",
		Preview: &api.Action{Kind: api.ActionRun, Title: "Preview", Ref: &api.ActionRef{ID: "preview"}},
		Edit:    &api.Action{Kind: api.ActionRun, Title: "Edit", Ref: &api.ActionRef{ID: "edit"}},
		Remove:  &api.Action{Kind: api.ActionRun, Title: "Remove", Ref: &api.ActionRef{ID: "remove"}},
	},
	api.CrudItem{ID: "email", Title: "Email signature", Subtitle: "Shared", Icon: "mail"},
)

var previewView = api.PreviewView("Release notes", "## 1.2.0\n\n- Faster search\n- Fixed the clipboard\n\n```go\nfmt.Println(\"done\")\n```")

var progressView = api.View{
	Kind: api.ViewProgress, Title: "Sync",
	Progress: &api.ProgressSpec{
		Label: "Syncing notes",
		Steps: []api.ProgressStep{
			{Title: "Fetch changes", Status: api.StepComplete},
			{Title: "Merge", Status: api.StepActive},
			{Title: "Publish", Status: api.StepPending},
		},
	},
}

func fileView(path, extension, kind string) *api.View {
	return &api.View{Kind: api.ViewFilePreview, Title: "Preview", File: &api.FilePreview{File: api.FileRef{Path: path, Name: path, Extension: extension, Kind: kind}}}
}

var moreNotes = []api.Item{
	{ID: "n3", Title: "Retro summary", Subtitle: "Friday", Icon: "file-text"},
	{ID: "n4", Title: "Hiring plan", Subtitle: "Q4", Icon: "file-text"},
}

var refreshedNote = api.Item{ID: "n1", Title: "Standup notes", Subtitle: "Refreshed just now", Icon: "file-text"}

var fruit = []api.Item{
	{ID: "apple", Title: "Apple", Glyph: "🍎"},
	{ID: "banana", Title: "Banana", Glyph: "🍌"},
	{ID: "cherry", Title: "Cherry", Glyph: "🍒"},
	{ID: "lemon", Title: "Lemon", Glyph: "🍋"},
}

// meaningMatches answers the second batch: a search for "yellow" finds the fruit whose colour is yellow, which the
// name match cannot.
func meaningMatches(text string) []string {
	if strings.Contains(strings.ToLower(text), "yellow") {
		return []string{"banana", "lemon"}
	}
	return nil
}

var searchResultsView = api.View{
	Kind: api.ViewGrid, Title: "Fruit", Placeholder: "Search fruit", Grid: &api.GridSpec{Columns: 4},
	Items:          fruit,
	OnSearchChange: &api.ActionRef{ID: "search-meaning"},
	EmptyView:      &api.EmptyState{Title: "No fruit found", Subtitle: "Try yellow."},
}

var accessoryOptions = []api.SearchAccessoryOption{
	{Title: "All", Value: "all"},
	{Title: "Done", Value: "done"},
}

// filtered answers the accessory: "done" keeps the done notes, anything else keeps them all.
func filtered(value string) []api.Item {
	if value == "done" {
		return []api.Item{{ID: "n2", Title: "Release plan", Subtitle: "Done", Icon: "check"}}
	}
	return []api.Item{
		{ID: "n1", Title: "Standup notes", Subtitle: "Monday", Icon: "file-text"},
		{ID: "n2", Title: "Release plan", Subtitle: "Done", Icon: "check"},
	}
}

var sourceView = api.View{
	Kind: api.ViewList, Title: "Remote notes", Placeholder: "Search notes",
	Source:     &api.DataSource{Loader: "notes", CacheKey: "notes", Retry: true},
	EmptyView:  &api.EmptyState{Title: "No notes yet", Subtitle: "Create one to see it here"},
	Refresh:    &api.Refresh{Immediate: true, Ref: &api.ActionRef{ID: "tick"}},
	Pagination: &api.Pagination{HasMore: true, Next: &api.ActionRef{ID: "more"}},
	SearchAccessory: &api.SearchAccessory{
		Value: "all", Options: accessoryOptions, OnChange: &api.ActionRef{ID: "filter"},
	},
}

var failingView = api.View{
	Kind: api.ViewList, Title: "Remote notes",
	Source:    &api.DataSource{Loader: "broken", Retry: true},
	EmptyView: &api.EmptyState{Title: "No notes yet"},
}

var emptyView = api.View{
	Kind: api.ViewList, Title: "Snippets", Placeholder: "Search snippets",
	EmptyView: &api.EmptyState{Title: "No snippets yet", Subtitle: "Create one with the add row"},
}

var selectedView = api.View{
	Kind: api.ViewList, Title: "Tasks", SelectedItemID: "b",
	Items: []api.Item{
		{ID: "a", Title: "Write the report", Icon: "pencil"},
		{ID: "b", Title: "Send the invoice", Icon: "check", Subtitle: "Selected by the extension"},
		{ID: "c", Title: "Archived task", Icon: "archive", Disabled: true},
		{ID: "notice", Title: "Sync is paused while offline", Contextual: true, Disabled: true},
	},
}

var shortcutView = api.View{
	Kind: api.ViewList, Title: "Paste",
	Items: []api.Item{{
		ID: "snippet", Title: "Sign-off", Subtitle: "Cmd+Y pastes it", Icon: "clipboard",
		Primary: api.Primary(api.Run("shortcut-run", "Copy", nil)),
		Actions: []api.Action{{Kind: api.ActionRun, Title: "Paste text", Shortcut: "CommandOrControl+Y", Ref: &api.ActionRef{ID: "shortcut-run"}}},
	}},
}

var imagesView = api.View{
	Kind: api.ViewList, Title: "Screens",
	Items: []api.Item{{
		ID: "shot", Title: "Dashboard", Subtitle: "1x1 sample", Icon: "image",
		Image:  &api.Image{Src: tinyPNG, Alt: "sample"},
		Detail: &api.Detail{Title: "Dashboard", Image: &api.Image{Src: tinyPNG, Alt: "sample"}, Text: "A single pixel."},
	}},
}

var confirmView = api.View{
	Kind: api.ViewList, Title: "Notes",
	Items: []api.Item{{
		ID: "alpha", Title: "Alpha note", Subtitle: "Draft",
		Primary: &api.Action{Kind: api.ActionRun, Title: "Delete", Ref: &api.ActionRef{ID: "remove"},
			Confirm: &api.Confirmation{Title: "Delete “Alpha note”?", Message: "This cannot be undone.", ConfirmLabel: "Delete", Destructive: true}},
	}},
}

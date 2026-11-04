package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// acme.views is the dev fixture of every view type the host draws. Each command opens one view; RunAction answers
// with it, so the fixture exercises the same path a real extension takes. The file view reads the path it is given.

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{
		{ID: "list", Title: "List fixture"},
		{ID: "sections", Title: "Sections fixture"},
		{ID: "grid", Title: "Grid fixture"},
		{ID: "collection", Title: "Collection fixture"},
		{ID: "preview", Title: "Preview fixture"},
		{ID: "progress", Title: "Progress fixture"},
		{ID: "file", Title: "File preview fixture"},
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
	case "collection":
		return api.Result{View: &collectionView}, nil
	case "preview":
		return api.Result{View: &previewView}, nil
	case "progress":
		return api.Result{View: &progressView}, nil
	case "file":
		return api.Result{View: fileView(in.Ref.Args["path"])}, nil
	case "reviewed":
		return api.Result{Patch: &api.ViewPatch{Items: []api.Item{reviewedAlpha}}}, nil
	}
	return api.Result{}, nil
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

func fileView(path string) *api.View {
	return &api.View{Kind: api.ViewFilePreview, Title: "Preview", File: &api.FilePreview{File: api.FileRef{Path: path, Name: path, Extension: ".txt"}}}
}

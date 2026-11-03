package api

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/golden/samples.json from the current samples")

const goldenPath = "testdata/golden/samples.json"

// namedSample pairs a readable name with a representative value of one wire type.
type namedSample struct {
	name  string
	value any
}

// wireSamples covers every major wire shape: each view kind, each action family, and each
// service result that crosses the boundary. Nested-only types are exercised through these.
func wireSamples() []namedSample {
	zero := 0.0
	ten := 10.0
	list := ListView("Clock", Item{ID: "tokyo", Title: "Tokyo 09:00", Subtitle: "Asia/Tokyo",
		Accessories: []Accessory{{Text: "JST", Tone: ToneMuted}}, Primary: Primary(CopyText("Copy", "09:00", false)),
		Actions: []Action{RevealPath("Reveal", "/tmp/x")}, Shortcut: "CommandOrControl+1"})
	list.Sections = []ItemSection{{Title: "Cities", Items: []Item{{ID: "london", Title: "London"}}}}
	list.Refresh = &Refresh{IntervalMs: 60000, Mode: PatchMerge, Immediate: true, Ref: &ActionRef{ID: "tick"}}
	list.Pagination = &Pagination{HasMore: true, PageSize: 50, Next: &ActionRef{ID: "more"}}
	list.SearchAccessory = &SearchAccessory{ID: "zone", Options: []SearchAccessoryOption{{Title: "All", Value: "all"}}}
	list.Detail = &DetailPane{Placement: "side", Visible: boolPtr(true)}
	list.ActionPanel = Panel(Section("Clock", Run("settings", "Settings", nil)))

	sourced := ListView("Remote")
	sourced.Source = &DataSource{Loader: "pull", CacheKey: "pull:v1", TTLMs: 60000, StaleTTLMs: 300000, Retry: true}
	sourced.EmptyView = &EmptyState{Title: "Nothing yet", Subtitle: "Create one with Add"}

	grid := GridView("Images", 4, Item{ID: "a.png", Title: "a.png", Image: &Image{Src: "data:image/png;base64,AAAA", Fit: FitCover, Shape: ShapeRounded}})

	form := FormView("Settings", []Field{
		{ID: "name", Type: FieldText, Label: "Name", Required: true, Rules: &Rules{MaxLength: 40}},
		{ID: "threshold", Type: FieldNumber, Rules: &Rules{Min: &zero, Max: &ten}, Value: TextValue("5")},
		{ID: "tags", Type: FieldMultiselect, Options: []Option{{Title: "Work", Value: "work"}}, Value: ListValue("work")},
		{ID: "notify", Type: FieldCheckbox, Value: FlagValue(true)},
	}, Run("save", "Save", map[string]string{"source": "form"}))

	editor := EditorView("Note", "# Title\n\nBody", Run("save-note", "Save", nil))
	editor.Editor.Format = "markdown"
	editor.Editor.Draft = &DraftOptions{Key: "note:1", Version: "2026-11-03", DebounceMs: 400}
	editor.Editor.TitleFromLine = true

	progress := ProgressView("Export", ProgressStep{Title: "Scan", Status: StepComplete}, ProgressStep{Title: "Write", Status: StepActive})
	progress.Progress.Value = 1
	progress.Progress.Total = 2

	collection := CollectionView("Snippets", &Action{Kind: ActionPushView, Title: "New", View: &View{Kind: ViewEditor, Title: "New"}},
		CrudItem{ID: "sig", Title: "Signature", Preview: &Action{Kind: ActionCopyText, Text: "Ada"}, Remove: &Action{Kind: ActionTrash, Paths: []string{"/tmp/x"}}})

	preview := PreviewView("Readme", "Hello\n\n- item")
	filePreview := View{Kind: ViewFilePreview, Title: "Shot", File: &FilePreview{File: FileRef{Path: "/tmp/shot.png", Name: "shot.png", Kind: "image"}}}
	webview := View{Kind: ViewWebview, Title: "Embed", Webview: &WebviewSpec{HTML: "<p>hi</p>", Permissions: []WebviewPermission{WebviewAutoplay}}}
	camera := View{Kind: ViewCamera, Title: "Camera", Camera: &CameraSpec{Framing: "contain", Mirrored: true}}

	menu := MenuBar{ID: "main", Icon: "timer", Title: "25:00", Tooltip: "Pomodoro",
		Sections: []MenuSection{{Title: "Timer", Rows: []MenuRow{
			{ID: "start", Title: "Start", Action: &Action{Kind: ActionRun, Ref: &ActionRef{ID: "start"}}},
			{ID: "more", Title: "More", Items: []MenuRow{{ID: "reset", Title: "Reset"}}},
		}}}}

	invocation := Invocation{Ref: ActionRef{ID: "save", Args: map[string]string{"id": "1"}}, SelectedItemID: "tokyo",
		Form: FormValues{"name": TextValue("Ada")}, Launch: LaunchContext{Reason: "manual", StartedAtMs: 1700000000000}}

	return []namedSample{
		{"manifest", Manifest{APIVersion: "1.0", Identity: Identity{ID: "acme.world-clock", Title: "World Clock"}, Capabilities: []Capability{CapabilityAI}}},
		{"command", Command{ID: "refresh", Title: "Refresh Clocks", GlobalShortcut: "CommandOrControl+Alt+R", Mode: ModeBackground,
			Triggers: []Trigger{{Type: TriggerInterval, Every: "5m"}}, Placement: []Placement{PlacementRoot}}},
		{"invocation", invocation},
		{"result-view", Result{View: &list, Navigation: NavPush}},
		{"result-patch", Result{Patch: &ViewPatch{Mode: PatchMerge, Items: []Item{{ID: "tokyo", Title: "Tokyo 09:01"}}, RemoveItemIDs: []string{"old"}}}},
		{"result-draft", Result{Draft: &DraftResolution{Key: "note:1", Resolution: "migrate", Content: "merged"}}},
		{"launch-context", LaunchContext{Trigger: &Trigger{Type: TriggerFilesChanged, Roots: []string{"~/Downloads"}, FileKind: FileKindImage}, Event: "files.changed", ChangedPaths: []string{"/home/u/Downloads/a.png"}, StartedAtMs: 1}},
		{"trigger-files", Trigger{Type: TriggerFilesChanged, Roots: []string{"~/Desktop"}, DebounceMs: 500, Ignore: []string{"*.tmp"}}},
		{"item-full", Item{ID: "x", Title: "X", Subtitle: "s", Keywords: []string{"k"}, Icon: "camera", Glyph: "⌘", Video: "https://example.com/v.mp4",
			File: &FileRef{Path: "/tmp/a.png", Name: "a.png", Size: 1}, Appearance: &Appearance{Foreground: ColorBlue, Prominence: "primary"},
			Detail: &Detail{Title: "X", Markdown: "**bold**", Metadata: []Metadata{{Kind: MetadataTag, Label: "Status", Value: "ok", Tone: ToneSuccess}},
				Link: &LinkCard{URL: "https://example.com", Status: "loading"}}, ActionPanelVisibility: PanelMenu, QueryFallback: false, Score: 0.5}},
		{"action-run", Run("open", "Open", map[string]string{"path": "/tmp"})},
		{"action-confirm", WithConfirmation(Destructive(Action{Kind: ActionTrash, Paths: []string{"/tmp/x"}}), Confirmation{Title: "Move to Trash?", ConfirmLabel: "Trash", Destructive: true})},
		{"action-shell", Action{Kind: ActionShellExec, Title: "Uptime", Shell: &ShellCommand{Command: "uptime", TimeoutMs: 2000, OutputLimit: 4096}}},
		{"action-paste", Action{Kind: ActionPaste, Title: "Paste", Content: &ClipboardContent{Type: "text", Text: "x", Concealed: true}, Paste: &PasteOptions{RestoreClipboard: true}}},
		{"view-list", list},
		{"view-source", sourced},
		{"view-grid", grid},
		{"view-form", form},
		{"view-editor", editor},
		{"view-progress", progress},
		{"view-collection", collection},
		{"view-preview", preview},
		{"view-file-preview", filePreview},
		{"view-webview", webview},
		{"view-camera", camera},
		{"menubar", menu},
		{"window-options", WindowOptions{ID: "notes", Title: "Notes", AlwaysOnTop: boolPtr(false), Size: SizeLarge, Persistent: true, RestoreKey: "notes"}},
		{"window-layout", WindowLayout{Kind: LayoutExact, Exact: &ExactFrame{X: &Dimension{Value: 10, Unit: "percent"}, Width: &Dimension{Value: 640, Unit: "points"}}}},
		{"apply-result", ApplyResult{OK: false, Reason: "no-target", Message: "Focus an app first."}},
		{"window-info", WindowInfo{Handle: "h1", AppID: "com.apple.Safari", AppName: "Safari", Title: "Docs", Frame: Rect{X: 1, Y: 2, Width: 3, Height: 4}, DisplayID: "d1"}},
		{"file-ref", FileRef{Path: "/tmp/a.png", Name: "a.png", Kind: "image", Width: 10, Height: 5, MtimeMs: 1700000000000}},
		{"failure", Failure{Code: CodePermission, Message: "Nevermind needs Screen Recording permission.", Reference: "err-42"}},
		{"form-values", FormValues{"a": TextValue("x"), "b": FlagValue(true), "c": ListValue("1", "2")}},
		{"template-result", TemplateResult{Text: "Hi Ada", Cursor: intPtr(3), Missing: []string{"name"}}},
		{"log-entry", LogEntry{TimestampMs: 1, Level: LogInfo, Source: "extension", ExtensionID: "acme.x", Message: "ran", Data: json.RawMessage(`{"n":1}`)}},
		{"clipboard-entry", ClipboardEntry{ID: "c1", Type: "text", Text: "hello", CreatedAtMs: 2}},
		{"shell-result", ShellResult{Stdout: "up 3 days", ExitCode: 0}},
		{"ocr-result", OCRResult{Text: "Invoice 42", Confidence: 0.93, Blocks: []OCRBlock{{Text: "Invoice", Box: &Rect{Width: 10, Height: 4}}}}},
		{"ai-options", AIOptions{Model: ModelFast, System: "Be brief.", Attachments: []Attachment{{Kind: "text", Text: "ctx", Title: "Selection"}}}},
		{"decision-request", DecisionRequest{Input: json.RawMessage(`"hello"`), Questions: []DecisionQuestion{{ID: "q1", Type: DecisionScore, Question: "Urgency?", Min: 1, Max: 5}}}},
		{"decision-result", DecisionResult{ID: "d1", Model: "route", Answers: map[string]DecisionAnswer{"q1": {Type: DecisionPredicate, Flag: true, Probability: 0.8}}}},
		{"chat-summary", ChatSummary{ID: "c1", Title: "Build clock", ExtensionFiles: []string{"clock.go"}}},
	}
}

func boolPtr(value bool) *bool { return &value }

func intPtr(value int) *int { return &value }

func TestWireSamplesRoundTripThroughJSON(t *testing.T) {
	for _, sample := range wireSamples() {
		t.Run(sample.name, func(t *testing.T) {
			first := mustMarshal(t, sample.value)
			decoded := reflect.New(reflect.TypeOf(sample.value))
			if err := json.Unmarshal(first, decoded.Interface()); err != nil {
				t.Fatalf("unmarshal: %v\n%s", err, first)
			}
			second := mustMarshal(t, decoded.Elem().Interface())
			if !bytes.Equal(first, second) {
				t.Fatalf("round trip changed the payload\nfirst:  %s\nsecond: %s", first, second)
			}
		})
	}
}

func TestWireSamplesMatchGoldenJSON(t *testing.T) {
	samples := map[string]json.RawMessage{}
	for _, sample := range wireSamples() {
		samples[sample.name] = mustMarshal(t, sample.value)
	}
	current := mustMarshal(t, samples)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, append(current, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update to create it): %v", err)
	}
	if !bytes.Equal(bytes.TrimSpace(want), current) {
		t.Fatalf("wire JSON changed; review the diff and rerun with -update if intended")
	}
}

func TestSampleNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, sample := range wireSamples() {
		if seen[sample.name] {
			t.Fatalf("duplicate sample name %q", sample.name)
		}
		seen[sample.name] = true
	}
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %T: %v", value, err)
	}
	return data
}

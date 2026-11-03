package api

import "testing"

func TestManifestValidationRules(t *testing.T) {
	valid := Manifest{APIVersion: "1.0", Identity: Identity{ID: "acme.world-clock", Title: "World Clock"}}
	tests := []struct {
		name    string
		mutate  func(m *Manifest)
		wantErr bool
	}{
		{name: "valid", mutate: func(m *Manifest) {}},
		{name: "id needs a dot", mutate: func(m *Manifest) { m.ID = "clock" }, wantErr: true},
		{name: "id rejects uppercase", mutate: func(m *Manifest) { m.ID = "Acme.Clock" }, wantErr: true},
		{name: "reserved built-in prefix", mutate: func(m *Manifest) { m.ID = "nevermind.clock" }, wantErr: true},
		{name: "title required", mutate: func(m *Manifest) { m.Title = " " }, wantErr: true},
		{name: "newer api is rejected", mutate: func(m *Manifest) { m.APIVersion = "1.9" }, wantErr: true},
		{name: "other major is rejected", mutate: func(m *Manifest) { m.APIVersion = "2.0" }, wantErr: true},
		{name: "unknown capability is not an error", mutate: func(m *Manifest) {
			m.Capabilities = []Capability{CapabilityAI, "future.thing"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := valid
			tt.mutate(&manifest)
			err := manifest.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	unknown := Manifest{Capabilities: []Capability{CapabilityAI, "future.thing"}}
	if got := unknown.UnknownCapabilities(); len(got) != 1 || got[0] != "future.thing" {
		t.Fatalf("UnknownCapabilities = %v, want [future.thing]", got)
	}
}

func TestActionValidationRequiresKindSpecificFields(t *testing.T) {
	tests := []struct {
		name    string
		action  Action
		wantErr bool
	}{
		{name: "run with ref", action: Run("refresh", "Refresh", nil)},
		{name: "run without ref", action: Action{Kind: ActionRun}, wantErr: true},
		{name: "open url https", action: OpenURL("Docs", "https://example.com")},
		{name: "open url javascript rejected", action: Action{Kind: ActionOpenURL, URL: "javascript:alert(1)"}, wantErr: true},
		{name: "copy needs text", action: Action{Kind: ActionCopyText}, wantErr: true},
		{name: "paste needs content", action: Action{Kind: ActionPaste}, wantErr: true},
		{name: "push needs a view", action: Action{Kind: ActionPushView}, wantErr: true},
		{name: "shell needs one of command or script", action: Action{Kind: ActionShellExec, Shell: &ShellCommand{Command: "ls", Script: "ls"}}, wantErr: true},
		{name: "shell command ok", action: Action{Kind: ActionShellExec, Shell: &ShellCommand{Command: "ls"}}},
		{name: "system op ok", action: Action{Kind: ActionSystem, Op: "lock-screen"}},
		{name: "system op unknown", action: Action{Kind: ActionSystem, Op: "reboot-the-moon"}, wantErr: true},
		{name: "window needs id or view", action: Action{Kind: ActionWindow, Op: "show", Window: &WindowRef{}}, wantErr: true},
		{name: "window show by id", action: Action{Kind: ActionWindow, Op: "show", Window: &WindowRef{ID: "notes"}}},
		{name: "unknown kind", action: Action{Kind: "teleport"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTriggerValidationChecksTypeAndFields(t *testing.T) {
	tests := []struct {
		name    string
		trigger Trigger
		wantErr bool
	}{
		{name: "startup", trigger: Trigger{Type: TriggerStartup}},
		{name: "interval 5m", trigger: Trigger{Type: TriggerInterval, Every: "5m"}},
		{name: "interval without every", trigger: Trigger{Type: TriggerInterval}, wantErr: true},
		{name: "interval below one second", trigger: Trigger{Type: TriggerInterval, Every: "500ms"}, wantErr: true},
		{name: "interval not a duration", trigger: Trigger{Type: TriggerInterval, Every: "soon"}, wantErr: true},
		{name: "files without roots", trigger: Trigger{Type: TriggerFilesChanged}, wantErr: true},
		{name: "files with roots", trigger: Trigger{Type: TriggerFilesChanged, Roots: []string{"~/Downloads"}, FileKind: FileKindImage}},
		{name: "files unknown kind", trigger: Trigger{Type: TriggerFilesChanged, Roots: []string{"~/x"}, FileKind: "spreadsheet"}, wantErr: true},
		{name: "clipboard", trigger: Trigger{Type: TriggerClipboardChanged, DebounceMs: 200}},
		{name: "unknown type", trigger: Trigger{Type: "moon-rise"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.trigger.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommandAndMenuBarValidation(t *testing.T) {
	if err := (Command{ID: "run", Title: "Run", Mode: ModeBackground,
		Triggers: []Trigger{{Type: TriggerInterval, Every: "1h"}}}).Validate(); err != nil {
		t.Fatalf("valid background command rejected: %v", err)
	}
	if err := (Command{ID: "run", Title: "Run", Mode: "daemon"}).Validate(); err == nil {
		t.Fatal("unknown command mode accepted")
	}
	if err := (Command{ID: "run", Title: "Run", Triggers: []Trigger{{Type: TriggerInterval}}}).Validate(); err == nil {
		t.Fatal("command with an invalid trigger accepted")
	}

	longTitle := MenuBar{Title: "this title is far too long for the menu bar"}
	if err := longTitle.Validate(); err == nil {
		t.Fatal("menu bar title over 24 characters accepted")
	}
	nested := MenuBar{Sections: []MenuSection{{Rows: []MenuRow{{ID: "a", Title: "A", Items: []MenuRow{
		{ID: "b", Title: "B", Items: []MenuRow{{ID: "c", Title: "C"}}},
	}}}}}}
	if err := nested.Validate(); err == nil {
		t.Fatal("two-level menu submenu accepted")
	}
	popoverAndSections := MenuBar{Sections: []MenuSection{{Rows: []MenuRow{{ID: "a", Title: "A"}}}},
		Popover: &Popover{View: PreviewView("P", "text")}}
	if err := popoverAndSections.Validate(); err == nil {
		t.Fatal("menu bar with both popover and sections accepted")
	}
	if !KnownMenuBarIcon("zap") || KnownMenuBarIcon("moonshine") {
		t.Fatal("curated menu bar icon set is wrong")
	}
}

func TestViewValidationRequiresOnlyItsOwnSpec(t *testing.T) {
	if err := ListView("Clock", Item{ID: "a", Title: "A"}, Item{ID: "a", Title: "again"}).Validate(); err == nil {
		t.Fatal("duplicate item ids accepted")
	}
	mismatched := ListView("Clock")
	mismatched.Grid = &GridSpec{Columns: 3}
	if err := mismatched.Validate(); err == nil {
		t.Fatal("list view carrying a grid spec accepted")
	}
	sourced := ListView("Remote")
	sourced.Source = &DataSource{Loader: "load"}
	if err := sourced.Validate(); err == nil {
		t.Fatal("data source without emptyView accepted")
	}
	if err := GridView("Images", 4, Item{ID: "x", Title: "X"}).Validate(); err != nil {
		t.Fatalf("valid grid rejected: %v", err)
	}
	dupRecords := CollectionView("Notes", nil, CrudItem{ID: "n1", Title: "a"}, CrudItem{ID: "n1", Title: "b"})
	if err := dupRecords.Validate(); err == nil {
		t.Fatal("duplicate collection record ids accepted")
	}
	reserved := CollectionView("Notes", nil, CrudItem{ID: "__nvm:add", Title: "a"})
	if err := reserved.Validate(); err == nil {
		t.Fatal("reserved collection id accepted")
	}
}

func TestWindowLayoutValidationStaysInsideWorkArea(t *testing.T) {
	tests := []struct {
		name    string
		layout  WindowLayout
		wantErr bool
	}{
		{name: "left half", layout: WindowLayout{Kind: LayoutFraction, Fraction: &Rect{X: 0, Y: 0, Width: 0.5, Height: 1}}},
		{name: "overflows right edge", layout: WindowLayout{Kind: LayoutFraction, Fraction: &Rect{X: 0.6, Y: 0, Width: 0.5, Height: 1}}, wantErr: true},
		{name: "zero width", layout: WindowLayout{Kind: LayoutFraction, Fraction: &Rect{X: 0, Y: 0, Width: 0, Height: 1}}, wantErr: true},
		{name: "resize step too big", layout: WindowLayout{Kind: LayoutResizeStep, Direction: "larger", Step: 0.5}, wantErr: true},
		{name: "resize step ok", layout: WindowLayout{Kind: LayoutResizeStep, Direction: "smaller", Step: 0.05}},
		{name: "bad edge", layout: WindowLayout{Kind: LayoutEdge, Edge: "middle"}, wantErr: true},
		{name: "exact needs frame", layout: WindowLayout{Kind: LayoutExact}, wantErr: true},
		{name: "center needs nothing", layout: WindowLayout{Kind: LayoutCenter}},
		{name: "unknown kind", layout: WindowLayout{Kind: "spin"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.layout.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFailureOfMapsSentinelsToCodes(t *testing.T) {
	if got := FailureOf(nil); got != (Failure{}) {
		t.Fatalf("FailureOf(nil) = %+v", got)
	}
	if got := FailureOf(ErrPermission); got.Code != CodePermission {
		t.Fatalf("permission sentinel mapped to %q", got.Code)
	}
	if got := FailureOf(ErrQuotaExceeded); got.Code != CodeQuota {
		t.Fatalf("quota sentinel mapped to %q", got.Code)
	}
}

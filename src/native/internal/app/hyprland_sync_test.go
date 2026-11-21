package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/hyprland"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/shortcuts/compositor"
)

func TestEveryShortcutSaveThatChangesTheDocumentAnnouncesOnce(t *testing.T) {
	changes := 0
	doc := notifyingDocument{Persistence: shortcuts.NewMemoryDocument(shortcuts.DefaultDocument("")), onChange: func() { changes++ }}
	set := func(accelerator string) {
		_ = doc.Update(func(d *shortcuts.Document) { d.Palette = accelerator })
	}
	set("Control+space")
	set("Control+space") // unchanged: no announcement
	set("")
	if changes != 2 {
		t.Fatalf("changes %d, want 2", changes)
	}
	_ = doc.Update(func(d *shortcuts.Document) { d.Actions["command:quick-notes"] = "Alt+N" })
	_ = doc.Update(func(d *shortcuts.Document) { delete(d.Actions, "command:quick-notes") })
	if changes != 4 {
		t.Fatalf("changes %d, want 4", changes)
	}
}

func TestNoCompositorSyncIsRequestedBeforeStartupKnowsWhichActionsRun(t *testing.T) {
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "sig")
	p := &App{} // a dispatch would panic: onUI is unset
	p.requestCompositorSync()
	if p.compositorSync.pending.Load() {
		t.Fatal("a sync was requested before startup was ready")
	}
}

func TestACollisionOrFailureIsReportedOncePerChange(t *testing.T) {
	var r problemReporter
	skipped := compositor.SyncOutcome{Skipped: []hyprland.Skipped{{What: "Alt+N", Reason: "your own bind uses it"}}}
	message := compositorProblem(skipped, nil)
	if message == "" || !r.shouldReport(message) {
		t.Fatalf("first report %q", message)
	}
	if r.shouldReport(compositorProblem(skipped, nil)) {
		t.Fatal("the same collision was reported twice")
	}
	if r.shouldReport(compositorProblem(compositor.SyncOutcome{}, nil)) {
		t.Fatal("a clean sync reported")
	}
	if !r.shouldReport(message) {
		t.Fatal("the collision after a clean sync was not reported again")
	}
}

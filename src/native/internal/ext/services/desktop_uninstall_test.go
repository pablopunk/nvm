package services

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	appfeature "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

// uninstallPort is an apps lifecycle that discovers one reviewed app and records what Trash is asked to move.
type uninstallPort struct {
	lifecycleRecorder
	trashed  map[string]bool
	snapshot appfeature.UninstallSnapshot
}

func (p *uninstallPort) DiscoverUninstall(string) appfeature.Discovery {
	return appfeature.Discovery{Ready: true, Notes: []appfeature.Note{{Message: "Some data was skipped."}}, Snapshot: appfeature.UninstallSnapshot{
		AppPath: notesApp,
		Candidates: []appfeature.Candidate{
			{ID: "app", Path: notesApp, Kind: appfeature.KindApp, Reason: "The application", SelectedByDefault: true},
			{ID: "data", Path: testpath.Abs("Users", "t", "Library", "Notes"), Kind: appfeature.KindAssociated, Reason: "Application data"},
		},
	}}
}

func (p *uninstallPort) TrashUninstall(snapshot appfeature.UninstallSnapshot, values map[string]bool) appfeature.TrashResult {
	p.trashed = values
	p.snapshot = snapshot
	return appfeature.TrashResult{Status: appfeature.StatusComplete, Moved: []string{notesApp}}
}

func (p *uninstallPort) TrashFolder() string { return testpath.Abs("Users", "t", ".Trash") }

// TestUninstallReviewListsTheCandidatesWithTheirDefaults: a ready review keeps its candidates under a token, with the
// default selection the host chose.
func TestUninstallReviewListsTheCandidatesWithTheirDefaults(t *testing.T) {
	port := &uninstallPort{}
	h, _, _ := lifecycleTestHandler(t, port, platform.CapabilityAppUninstall)
	var review api.UninstallReview
	mustDecode(t, mustOK(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpUninstallReview, protocol.PathArgs{Path: notesApp})), &review)
	if !review.Ready || review.Token == "" || len(review.Items) != 2 || review.Items[0].ID != "app" || !review.Items[0].Selected || review.Items[1].Selected {
		t.Fatalf("review = %+v", review)
	}
	if len(review.Notes) != 1 || review.Notes[0] != "Some data was skipped." {
		t.Fatalf("notes = %v", review.Notes)
	}
}

// TestUninstallTrashMovesTheReviewedSnapshot: the selected IDs go to the Trash with the snapshot that was reviewed, and
// the result names the Trash folder.
func TestUninstallTrashMovesTheReviewedSnapshot(t *testing.T) {
	port := &uninstallPort{}
	h, _, _ := lifecycleTestHandler(t, port, platform.CapabilityAppUninstall)
	var review api.UninstallReview
	mustDecode(t, mustOK(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpUninstallReview, protocol.PathArgs{Path: notesApp})), &review)
	var result api.UninstallResult
	mustDecode(t, mustOK(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpUninstallTrash, protocol.UninstallTrashArgs{Token: review.Token, IDs: []string{"app", "data"}})), &result)
	if !port.trashed["app"] || !port.trashed["data"] || port.snapshot.AppPath != notesApp {
		t.Fatalf("trash got %v over %q", port.trashed, port.snapshot.AppPath)
	}
	if result.Status != appfeature.StatusComplete || result.TrashFolder != testpath.Abs("Users", "t", ".Trash") || len(result.Moved) != 1 {
		t.Fatalf("result = %+v", result)
	}
}

// TestUninstallTrashRefusesAnUnknownReview: a token the host does not hold, or never gave, moves nothing.
func TestUninstallTrashRefusesAnUnknownReview(t *testing.T) {
	port := &uninstallPort{}
	h, _, _ := lifecycleTestHandler(t, port, platform.CapabilityAppUninstall)
	reply := serve(t, h, protocol.ServiceDesktopApps, protocol.OpUninstallTrash, protocol.UninstallTrashArgs{Token: "uninstall-404", IDs: []string{"app"}})
	if reply.Failure == nil {
		t.Fatal("an unknown review moved items")
	}
	if port.trashed != nil {
		t.Fatalf("the port was asked to trash %v", port.trashed)
	}
}

// TestUninstallIsRefusedWithoutItsCapability: a platform that cannot uninstall answers unavailable.
func TestUninstallIsRefusedWithoutItsCapability(t *testing.T) {
	h, _, _ := lifecycleTestHandler(t, &uninstallPort{})
	requireCode(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpUninstallReview, protocol.PathArgs{Path: notesApp}), api.CodeUnavailable, "not available")
}

// aliasTable is a shortcut table with two aliased actions and one that is not an app's.
type aliasTable struct{}

func (aliasTable) List() []api.ShortcutRecord { return nil }

func (aliasTable) Palette() api.ShortcutRecord { return api.ShortcutRecord{} }

func (aliasTable) Aliases(prefix string) map[string][]string {
	all := map[string][]string{"app:/Applications/Notes.app": {"notes"}, "command:quit": {"exit"}}
	out := map[string][]string{}
	for id, phrases := range all {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			out[id] = phrases
		}
	}
	return out
}

// TestAliasesReadsOnlyTheRequestedPrefix: an alias read returns the actions under its prefix and refuses an empty one.
func TestAliasesReadsOnlyTheRequestedPrefix(t *testing.T) {
	h := New(Deps{Shortcuts: aliasTable{}}, manifestFor("acme.apps", api.CapabilityShortcuts))
	t.Cleanup(func() { _ = h.Close() })
	var aliases map[string][]string
	mustDecode(t, mustOK(t, serve(t, h, protocol.ServiceShortcuts, protocol.OpAliases, protocol.PrefixArgs{Prefix: "app:"})), &aliases)
	if len(aliases) != 1 || aliases["app:/Applications/Notes.app"][0] != "notes" {
		t.Fatalf("aliases = %v", aliases)
	}
	requireCode(t, serve(t, h, protocol.ServiceShortcuts, protocol.OpAliases, protocol.PrefixArgs{}), api.CodeInvalid, "prefix")
}

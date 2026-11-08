package extview

import (
	"crypto/sha256"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestDeniedThumbnailKeepsOtherAuthorizedAndUnrelatedResults(t *testing.T) {
	dir := t.TempDir()
	denied := filepath.Join(dir, "private", "denied.png")
	filePath := filepath.ToSlash(denied)
	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}
	fileURL := (&url.URL{Scheme: "file", Path: filePath}).String()
	good := "data:image/png;base64,Z29vZA=="
	old := "data:image/png;base64,c2VjcmV0"
	other := "data:image/png;base64,b3RoZXI="
	evidence := &api.SnapshotEvidence{ThumbnailHashes: map[[32]byte]bool{sha256.Sum256([]byte(good)): true}, DeniedPaths: []string{denied}}
	action := &api.Action{Kind: api.ActionCopyText, Text: "ok"}
	result := api.Result{SnapshotEvidence: evidence, View: &api.View{Kind: api.ViewGrid, Items: []api.Item{{ID: "/ok.png", Title: "allowed", Image: &api.Image{Src: good}, Primary: action}, {ID: denied, Title: "denied", Image: &api.Image{Src: fileURL}, Primary: action}, {ID: "other", Title: "unrelated", Image: &api.Image{Src: other}, Primary: action}, {ID: "stale", Image: &api.Image{Src: old}, Primary: action}}}}
	filterSnapshotDenied(&result, map[[32]byte]bool{sha256.Sum256([]byte(old)): true})
	rows := result.View.Items
	if rows[0].Image.Src != good || rows[0].Primary == nil || rows[2].Image.Src != other || rows[2].Primary == nil {
		t.Fatal("independent results lost")
	}
	if rows[1].Image != nil || rows[1].Primary != nil || rows[3].Image != nil || rows[3].Primary != nil {
		t.Fatal("denied or stale content survived")
	}
}
func TestSnapshotOnlyRetainsHostVerifiedThumbnails(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.png")
	deniedPath := filepath.Join(dir, "denied")
	c := &CommandSnapshots{}
	id := new(byte)
	ref := api.ActionRef{ID: "grid"}
	view := api.View{Kind: api.ViewGrid, Title: "x", Refresh: &api.Refresh{Immediate: true, Ref: &ref}}
	good := "data:image/png;base64,Z29vZA=="
	unknown := "data:image/png;base64,dW5rbm93bg=="
	items := []api.Item{{ID: goodPath, Title: "unrelated secret", Subtitle: "secret", Accessories: []api.Accessory{{Text: "secret"}}, Image: &api.Image{Src: good, Alt: "secret"}}, {ID: "unknown", Image: &api.Image{Src: unknown}}}
	evidence := &api.SnapshotEvidence{ThumbnailHashes: map[[32]byte]bool{sha256.Sum256([]byte(good)): true}, ThumbnailPaths: map[[32]byte][]string{sha256.Sum256([]byte(good)): {goodPath}}}
	items = append(items, api.Item{ID: deniedPath, Image: &api.Image{Src: good}})
	c.PutVerified(id, ref, view, items, evidence)
	got, ok := c.Get(id, ref)
	if !ok || len(got.Items) != 1 || got.Items[0].Image == nil || got.Title != "Cached files" || got.Items[0].Title != "good.png" || got.Items[0].Subtitle != "" || len(got.Items[0].Accessories) != 0 || got.Items[0].Image.Alt != "" {
		t.Fatal("unverified authority data entered snapshot")
	}
	evidence.DeniedPaths = []string{deniedPath}
	c.PutVerified(id, ref, view, items, evidence)
	got, ok = c.Get(id, ref)
	if !ok || len(got.Items) != 1 || got.Items[0].ID != goodPath {
		t.Fatal("mixed run did not retain only the authorized row")
	}
}
func TestUnrelatedFileDenialDoesNotInvalidateSnapshotAuthority(t *testing.T) {
	dir := t.TempDir()
	allowed := filepath.Join(dir, "allowed.png")
	other := filepath.Join(dir, "other.png")
	paths := map[[32]byte]bool{sha256.Sum256([]byte(allowed)): true}
	if snapshotPathsCover(paths, &api.SnapshotDenial{Path: other}) {
		t.Fatal("unrelated file denial matched snapshot")
	}
	if !snapshotPathsCover(paths, &api.SnapshotDenial{Path: allowed}) || !snapshotPathsCover(paths, &api.SnapshotDenial{AuthorityDenied: true}) {
		t.Fatal("represented authority was not detected")
	}
}
func TestDeniedPathsCannotSurviveInNestedViewsOrClipboardActions(t *testing.T) {
	denied := filepath.Join(t.TempDir(), "private", "denied.png")
	old := "data:image/png;base64,c2VjcmV0"
	stale := map[[32]byte]bool{sha256.Sum256([]byte(old)): true}
	e := &api.SnapshotEvidence{DeniedPaths: []string{denied}}
	nested := &api.View{Kind: api.ViewPreview, Title: "Preview", Preview: &api.PreviewSpec{Image: &api.Image{Src: old}}}
	result := api.Result{SnapshotEvidence: e, Next: &api.Action{Kind: api.ActionCopyImage, Path: denied}, View: &api.View{Kind: api.ViewGrid, Actions: []api.Action{{Kind: api.ActionCopyImage, Path: denied}, {Kind: api.ActionCopyText, Text: "safe"}, {Kind: api.ActionPushView, View: nested}, {Kind: api.ActionPaste, Content: &api.ClipboardContent{ImageDataURL: old}}}}}
	filterSnapshotDenied(&result, stale)
	if result.Next != nil || len(result.View.Actions) != 2 || result.View.Actions[0].Text != "safe" || result.View.Actions[1].View.Preview.Image != nil {
		t.Fatalf("denied nested content survived or independent action lost: %+v", result.View.Actions)
	}
}
func TestUnprotectedCommandKeepsChildHandledFilesDenial(t *testing.T) {
	ref := api.ActionRef{ID: "open"}
	view := &api.View{Kind: api.ViewGrid, Title: "handled", Items: []api.Item{{ID: "/private.png", Title: "Child result", Image: &api.Image{Src: "file:///private.png"}, Primary: &api.Action{Kind: api.ActionCopyText, Text: "kept"}}}}
	deps := Deps{CommandSnapshots: &CommandSnapshots{}, CommandSnapshotIdentity: func(string) any { return new(byte) }, InvokeCommand: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		if in.ProtectSnapshot {
			t.Fatal("uncached command became protected")
		}
		done(api.Result{View: view, SnapshotEvidence: &api.SnapshotEvidence{FilesAuthorityDenied: true}}, nil)
	}}
	h := &fakeHost{}
	CommandActionFor(deps, "x", api.Action{Kind: api.ActionRun, Ref: &ref}).Run(h)
	if len(h.pushed) != 1 || view.Items[0].Title != "Child result" || view.Items[0].Image == nil || view.Items[0].Primary == nil {
		t.Fatal("child-handled result was changed without a snapshot")
	}
}

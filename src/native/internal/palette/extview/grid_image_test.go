package extview

import (
	"context"
	"crypto/sha256"
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func scopedTestPicture(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return thumbs.GridReference((&url.URL{Scheme: "file", Path: path}).String())
}

func TestGridReadDenialFiltersOnlyItsViewAndRejectsOldCompletions(t *testing.T) {
	denied, allowed := filepath.Join(t.TempDir(), "denied.png"), filepath.Join(t.TempDir(), "allowed.png")
	badRef, goodRef := scopedTestPicture(denied), scopedTestPicture(allowed)
	command := api.ActionRef{ID: "grid"}
	view := api.GridView("Files", 0,
		api.Item{ID: denied, Title: "denied", Image: &api.Image{Src: badRef}, Primary: api.Primary(api.OpenPath("Open", denied))},
		api.Item{ID: allowed, Title: "allowed", Image: &api.Image{Src: goodRef}, Primary: api.Primary(api.OpenPath("Open", allowed))})
	view.Refresh = &api.Refresh{Immediate: true, Ref: &command}
	identity := new(byte)
	snapshots := &CommandSnapshots{}
	evidence := &api.SnapshotEvidence{ThumbnailHashes: map[[32]byte]bool{}, ThumbnailPaths: map[[32]byte][]string{}}
	for ref, path := range map[string]string{badRef: denied, goodRef: allowed} {
		hash := sha256.Sum256([]byte(ref))
		evidence.ThumbnailHashes[hash] = true
		evidence.ThumbnailPaths[hash] = []string{path}
	}
	snapshots.PutVerified(identity, command, view, view.Items, evidence)
	otherCommand := api.ActionRef{ID: "other-grid"}
	otherView := view
	otherView.Refresh = &api.Refresh{Immediate: true, Ref: &otherCommand}
	snapshots.PutVerified(identity, otherCommand, otherView, otherView.Items, evidence)
	callbacks := make(map[string][]func(GridImageResult))
	deps := Deps{CommandSnapshots: snapshots, CommandSnapshotIdentity: func(string) any { return identity }, LoadGridImage: func(_ context.Context, request GridImageRequest, done func(GridImageResult)) {
		if request.Identity != identity || request.ExtensionID != "acme.files" || request.Command.ID != command.ID {
			t.Fatal("load lost its host scope")
		}
		if _, ok := request.Scope.Path(request.Binding(), request.Source); !ok {
			t.Fatal("load has no invocation-bound provenance")
		}
		callbacks[request.Source] = append(callbacks[request.Source], done)
	}}
	s := NewSession(deps, "acme.files", view, evidence)
	s.snapshotIdentity, s.snapshotRef = identity, &command
	s.bitmap(&api.Image{Src: badRef})
	s.bitmap(&api.Image{Src: goodRef})
	callbacks[badRef][0](GridImageResult{Denial: &api.SnapshotDenial{Path: denied, Failure: &api.Failure{Code: api.CodePermission}}})
	if s.items[0].Image != nil || s.items[0].Primary != nil {
		t.Fatal("denied row retained pixels or an action")
	}
	if s.items[1].Image == nil || s.items[1].Primary == nil {
		t.Fatal("independent authorized row was stripped")
	}
	callbacks[goodRef][0](GridImageResult{Bitmap: &ui.Bitmap{}})
	if _, exists := s.images[goodRef]; exists {
		t.Fatal("old generation restored an image")
	}
	s.bitmap(&api.Image{Src: goodRef})
	bitmap := &ui.Bitmap{}
	callbacks[goodRef][1](GridImageResult{Bitmap: bitmap, Authorized: func() bool { return true }})
	if s.images[goodRef] != bitmap {
		t.Fatal("fresh independently authorized picture did not load")
	}
	cached, ok := snapshots.Get(identity, command)
	if !ok || cached.Items[0].Image != nil || cached.Items[1].Image == nil {
		t.Fatal("retained snapshot did not filter just the denied path")
	}
	other, ok := snapshots.Get(identity, otherCommand)
	if !ok || other.Items[0].Image == nil {
		t.Fatal("denial crossed command scopes")
	}
}

func TestGridImageCallbacksCannotCrossNavigation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	ref := scopedTestPicture(path)
	var complete func(GridImageResult)
	s := NewSession(Deps{LoadGridImage: func(_ context.Context, _ GridImageRequest, done func(GridImageResult)) { complete = done }}, "one", api.GridView("Files", 0, api.Item{ID: path, Image: &api.Image{Src: ref}}))
	s.bitmap(&api.Image{Src: ref})
	s.close()
	complete(GridImageResult{Bitmap: &ui.Bitmap{}})
	if len(s.images) != 0 {
		t.Fatal("late image reached dismissed view")
	}
}

func TestPictureDenialCannotReplaceNewerOrForeignSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	ref := scopedTestPicture(path)
	command := api.ActionRef{ID: "grid"}
	identity, foreign := new(byte), new(byte)
	view := api.GridView("Files", 0, api.Item{ID: path, Image: &api.Image{Src: ref}})
	hash := sha256.Sum256([]byte(ref))
	evidence := &api.SnapshotEvidence{ThumbnailHashes: map[[32]byte]bool{hash: true}, ThumbnailPaths: map[[32]byte][]string{hash: {path}}}
	snapshots := &CommandSnapshots{}
	snapshots.PutVerified(identity, command, view, view.Items, evidence)
	version := snapshots.Version(identity, command)
	snapshots.PutVerified(identity, command, view, view.Items, evidence)
	snapshots.PutVerified(foreign, command, view, view.Items, evidence)
	snapshots.FilterPictureDenial(identity, command, version, &api.SnapshotDenial{Path: path})
	for _, owner := range []any{identity, foreign} {
		cached, ok := snapshots.Get(owner, command)
		if !ok || cached.Items[0].Image == nil {
			t.Fatal("old denial removed a newer or foreign snapshot")
		}
	}
}

func TestGridFirstPaintAndTypingDoNotWaitForPictures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	ref := scopedTestPicture(path)
	var complete func(GridImageResult)
	var work context.Context
	s := NewSession(Deps{LoadGridImage: func(ctx context.Context, _ GridImageRequest, done func(GridImageResult)) { work, complete = ctx, done }}, "files", api.GridView("Files", 0, api.Item{ID: path, Title: "picture", Image: &api.Image{Src: ref}}))
	page := s.Open(&fakeHost{})
	if page.Grid == nil || len(page.Search("pic")) != 1 || len(page.Search("other")) != 0 {
		t.Fatal("pending decoration blocked grid navigation or typing")
	}
	if complete == nil || work.Err() != nil {
		t.Fatal("picture did not remain pending")
	}
	page.OnLeave()
	if work.Err() == nil {
		t.Fatal("navigation did not cancel pending picture")
	}
	complete(GridImageResult{Bitmap: &ui.Bitmap{}})
	if len(s.images) != 0 {
		t.Fatal("canceled decoration published pixels")
	}
}

func TestPatchCannotReplayOldReferenceWithoutNewHostEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	ref := scopedTestPicture(path)
	hash := sha256.Sum256([]byte(ref))
	evidence := &api.SnapshotEvidence{ThumbnailHashes: map[[32]byte]bool{hash: true}, ThumbnailPaths: map[[32]byte][]string{hash: {path}}}
	var requests []GridImageRequest
	s := NewSession(Deps{LoadGridImage: func(_ context.Context, r GridImageRequest, _ func(GridImageResult)) { requests = append(requests, r) }}, "files", api.GridView("Files", 0, api.Item{ID: path, Image: &api.Image{Src: ref}}), evidence)
	s.bitmap(&api.Image{Src: ref})
	original := requests[0]
	s.applyPicturePatch(api.ViewPatch{Mode: api.PatchReplace, Items: []api.Item{{ID: path, Image: &api.Image{Src: ref}}}}, nil)
	s.bitmap(&api.Image{Src: ref})
	replacement := requests[1]
	if original.Scope.Current() {
		t.Fatal("replacement did not expire original read scope")
	}
	if _, ok := replacement.Scope.Path(replacement.Binding(), ref); ok {
		t.Fatal("replayed reference inherited old invocation provenance")
	}
	s.applyPicturePatch(api.ViewPatch{Mode: api.PatchReplace, Items: []api.Item{{ID: path, Image: &api.Image{Src: ref}}}}, evidence)
	s.bitmap(&api.Image{Src: ref})
	fresh := requests[2]
	if _, ok := fresh.Scope.Path(fresh.Binding(), ref); !ok {
		t.Fatal("fresh host evidence could not reauthorize reference")
	}
}

func TestPathDenialReauthorizesSurvivingLoadedPictures(t *testing.T) {
	first, second := filepath.Join(t.TempDir(), "denied.png"), filepath.Join(t.TempDir(), "allowed.png")
	bad, good := scopedTestPicture(first), scopedTestPicture(second)
	evidence := &api.SnapshotEvidence{ThumbnailHashes: make(map[[32]byte]bool), ThumbnailPaths: make(map[[32]byte][]string)}
	for ref, path := range map[string]string{bad: first, good: second} {
		hash := sha256.Sum256([]byte(ref))
		evidence.ThumbnailHashes[hash] = true
		evidence.ThumbnailPaths[hash] = []string{path}
	}
	callbacks := make(map[string]func(GridImageResult))
	scopes := make(map[string]GridImageRequest)
	s := NewSession(Deps{LoadGridImage: func(_ context.Context, r GridImageRequest, done func(GridImageResult)) {
		scopes[r.Source] = r
		callbacks[r.Source] = done
	}}, "files", api.GridView("Files", 0, api.Item{ID: first, Image: &api.Image{Src: bad}}, api.Item{ID: second, Image: &api.Image{Src: good}}), evidence)
	s.bitmap(&api.Image{Src: good})
	callbacks[good](GridImageResult{Bitmap: &ui.Bitmap{}, Authorized: scopes[good].Scope.Current})
	s.bitmap(&api.Image{Src: bad})
	callbacks[bad](GridImageResult{Denial: &api.SnapshotDenial{Path: first}})
	s.bitmap(&api.Image{Src: good})
	request := scopes[good]
	if _, ok := request.Scope.Path(request.Binding(), good); !ok {
		t.Fatal("independent row lost its provenance")
	}
	if len(s.items) != 2 || s.items[1].Image == nil || s.view.Title == "Access unavailable" {
		t.Fatal("path denial became an authority-wide denial through a stale image grant")
	}
}

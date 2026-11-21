package extview

import (
	"context"
	"crypto/sha256"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"path/filepath"
	"testing"
)

func TestQueuedMetadataDenialCannotFilterReplacementOrNewInvocation(t *testing.T) {
	for _, replacement := range []string{"view", "invocation"} {
		t.Run(replacement, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "image.png")
			ref := scopedTestPicture(path)
			identity := new(byte)
			var queue []func()
			s := NewSession(Deps{OnUI: func(fn func()) { queue = append(queue, fn) }, CommandSnapshotIdentity: func(string) any { return identity }}, "files", api.GridView("Files", 0, api.Item{ID: path, Image: &api.Image{Src: ref}}))
			paths := map[[32]byte]bool{sha256.Sum256([]byte(path)): true}
			old := s.snapshotDenialHandler(paths, 0)
			old(&api.SnapshotDenial{AuthorityDenied: true})
			if replacement == "view" {
				s.Replace(api.GridView("Replacement", 0, api.Item{ID: path, Image: &api.Image{Src: ref}}))
			} else {
				s.snapshotDenialHandler(paths, 0)
			}
			queue[0]()
			if len(s.items) != 1 || s.items[0].Image == nil || s.view.Title == "Access unavailable" {
				t.Fatal("stale metadata denial filtered replacement scope")
			}
		})
	}
}

func TestOneMetadataInvocationCanDenySeveralPaths(t *testing.T) {
	first, second := filepath.Join(t.TempDir(), "one.png"), filepath.Join(t.TempDir(), "two.png")
	s := NewSession(Deps{}, "files", api.GridView("Files", 0, api.Item{ID: first, Image: &api.Image{Src: scopedTestPicture(first)}}, api.Item{ID: second, Image: &api.Image{Src: scopedTestPicture(second)}}))
	paths := map[[32]byte]bool{sha256.Sum256([]byte(first)): true, sha256.Sum256([]byte(second)): true}
	deny := s.snapshotDenialHandler(paths, 0)
	deny(&api.SnapshotDenial{Path: first})
	deny(&api.SnapshotDenial{Path: second})
	if s.items[0].Image != nil || s.items[1].Image != nil {
		t.Fatal("first denial incorrectly expired the rest of its invocation")
	}
}

func TestRefreshMetadataDenialQueuedBeforeViewReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	identity := new(byte)
	var invocation api.Invocation
	var queued func()
	s := NewSession(Deps{CommandSnapshotIdentity: func(string) any { return identity }, OnUI: func(fn func()) { queued = fn }, InvokeRefresh: func(_ context.Context, _ string, in api.Invocation, _ func(api.Result, *api.Failure)) {
		invocation = in
	}}, "files", api.GridView("Files", 0, api.Item{ID: path, Image: &api.Image{Src: scopedTestPicture(path)}}))
	ref := api.ActionRef{ID: "grid"}
	s.snapshotRef = &ref
	s.snapshotIdentity = identity
	s.snapshotSourcePaths = map[[32]byte]bool{sha256.Sum256([]byte(path)): true}
	s.invokeRefresh(ref, func(api.Result, *api.Failure) {})
	invocation.OnSnapshotDenied(&api.SnapshotDenial{AuthorityDenied: true})
	s.Replace(api.GridView("Replacement", 0, api.Item{ID: path, Image: &api.Image{Src: scopedTestPicture(path)}}))
	queued()
	if s.view.Title != "Replacement" || len(s.items) != 1 || s.items[0].Image == nil {
		t.Fatal("old invocation erased replacement presentation")
	}
}

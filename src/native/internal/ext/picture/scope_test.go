package picture

import (
	"context"
	"crypto/sha256"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestScopeRejectsReplayedOrAlteredBindingsAndExpiredViews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	slash := filepath.ToSlash(path)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	ref := (&url.URL{Scheme: "file", Path: slash, Fragment: "nevermind-thumbnail"}).String()
	hash := sha256.Sum256([]byte(ref))
	evidence := &api.SnapshotEvidence{ThumbnailHashes: map[[32]byte]bool{hash: true}, ThumbnailPaths: map[[32]byte][]string{hash: {path}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	binding := Binding{ExtensionID: "files", Identity: new(byte), Session: new(byte), Invocation: "first-command", Generation: 1, Version: 2}
	scope := NewScope(ctx, binding, evidence)
	if got, ok := scope.Path(binding, ref); !ok || got != path {
		t.Fatal("issued reference was refused")
	}
	variants := []Binding{binding, binding, binding, binding, binding, binding}
	variants[0].ExtensionID = "other"
	variants[1].Identity = new(byte)
	variants[2].Session = new(byte)
	variants[3].Invocation = "other-command"
	variants[4].Generation++
	variants[5].Version++
	for _, other := range variants {
		if _, ok := scope.Path(other, ref); ok {
			t.Fatalf("replayed reference under another binding: %+v", other)
		}
	}
	if _, ok := NewScope(ctx, binding, nil).Path(binding, ref); ok {
		t.Fatal("installation-wide issuance replaced invocation evidence")
	}
	if _, ok := scope.Path(binding, ref+"forged"); ok {
		t.Fatal("altered reference inherited authority")
	}
	evidence.ThumbnailPaths[hash][0] = "/changed"
	if _, ok := scope.Path(binding, ref); !ok {
		t.Fatal("evidence was not copied")
	}
	cancel()
	if scope.Current() {
		t.Fatal("dismissed view kept its grant")
	}
	if _, ok := scope.Path(binding, ref); ok {
		t.Fatal("dismissed reference reopened")
	}
}

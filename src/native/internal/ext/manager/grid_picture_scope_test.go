package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/picture"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"os"
	"path/filepath"
	"testing"
)

func TestGridReadRequiresItsInvocationAndLiveSession(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.good")
	contents[bundle.ManifestFile] = bytes.ReplaceAll(contents[bundle.ManifestFile], []byte(`"desktop.apps"`), []byte(`"desktop.apps","desktop.files"`))
	if err := h.m.Install("acme.good", contents, true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	path := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(path, []byte("pixels"), 0600); err != nil {
		t.Fatal(err)
	}
	identity := h.m.SnapshotIdentity("acme.good")
	host := h.m.pictureHost("acme.good", identity)
	raw, _ := json.Marshal(protocol.PathArgs{Path: path})
	call := protocol.ServiceCall{Service: protocol.ServiceDesktopFiles, Op: protocol.OpThumbnail, Args: raw}
	reply := host.Serve(context.Background(), call)
	var thumbnail protocol.ThumbnailResult
	if err := json.Unmarshal(reply.Data, &thumbnail); err != nil || !thumbnail.Found {
		t.Fatalf("thumbnail not issued: %+v %v", reply, err)
	}
	evidence := &api.SnapshotEvidence{}
	supervisor.ObserveSnapshotService(evidence, call, reply)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	binding := picture.Binding{ExtensionID: "acme.good", Identity: identity, Session: new(byte), Invocation: "grid"}
	scope := picture.NewScope(ctx, binding, evidence)
	grant, err := h.m.OpenGridPictureSource(ctx, scope, binding, thumbnail.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer grant.Source.Close()
	other := binding
	other.Session = new(byte)
	if _, err := h.m.OpenGridPictureSource(ctx, scope, other, thumbnail.URL); err == nil {
		t.Fatal("reference replayed across sessions")
	}
	other = binding
	other.Invocation = "another-command"
	if _, err := h.m.OpenGridPictureSource(ctx, scope, other, thumbnail.URL); err == nil {
		t.Fatal("reference replayed across commands")
	}
	empty := picture.NewScope(ctx, binding, &api.SnapshotEvidence{})
	if _, err := h.m.OpenGridPictureSource(ctx, empty, binding, thumbnail.URL); err == nil {
		t.Fatal("installation-wide reference bypassed invocation provenance")
	}
	cancel()
	if grant.Current() {
		t.Fatal("dismissed session retained publication grant")
	}
	if _, err := h.m.OpenGridPictureSource(context.Background(), scope, binding, thumbnail.URL); err == nil {
		t.Fatal("closed view reopened source")
	}
}

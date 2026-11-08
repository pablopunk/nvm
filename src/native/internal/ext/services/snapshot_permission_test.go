package services

import (
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"io/fs"
	"path/filepath"
	"testing"
)

type deniedThumbnailFS struct{ platform.FS }

func (deniedThumbnailFS) ReadFilePrefix(string, int64) ([]byte, int64, error) {
	return nil, 0, fs.ErrPermission
}
func TestThumbnailOSDenialKeepsPublicFoundFalse(t *testing.T) {
	_, deps := newDesktopHost(t)
	deps.DataRoot = t.TempDir()
	deps.Files = deniedThumbnailFS{real.NewPorts().FS}
	h := New(deps, manifestFor("acme.files", api.CapabilityDesktopFiles))
	t.Cleanup(func() { _ = h.Close() })
	reply := serve(t, h, protocol.ServiceDesktopFiles, protocol.OpThumbnail, protocol.PathArgs{Path: filepath.Join(t.TempDir(), "x.png")})
	if reply.Failure != nil || reply.SnapshotPermissionFailure == nil || reply.SnapshotPermissionFailure.Code != api.CodePermission {
		t.Fatalf("reply %+v", reply)
	}
	var got protocol.ThumbnailResult
	if err := json.Unmarshal(reply.Data, &got); err != nil || got.Found || got.URL != "" {
		t.Fatalf("public result %+v %v", got, err)
	}
	raw, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip protocol.ServiceReply
	if err = json.Unmarshal(raw, &roundtrip); err != nil || roundtrip.SnapshotPermissionFailure != nil {
		t.Fatal("host-only permission marker crossed the wire")
	}
}

package supervisor

import (
	"encoding/json"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"testing"
)

func TestThumbnailEvidenceBoundsUniquePathsWithoutPromotingRepeatedDenial(t *testing.T) {
	e := &api.SnapshotEvidence{}
	args, _ := json.Marshal(protocol.PathArgs{Path: "/denied.png"})
	call := protocol.ServiceCall{Service: protocol.ServiceDesktopFiles, Op: protocol.OpThumbnail, Args: args}
	denied := protocol.ServiceReply{SnapshotPermissionFailure: &api.Failure{Code: api.CodePermission}}
	for i := 0; i < 300; i++ {
		d := observeSnapshotService(e, call, denied)
		if d == nil || d.AuthorityDenied || d.Path != "/denied.png" {
			t.Fatal("one denied file became authority-wide")
		}
	}
	if len(e.DeniedPaths) != 1 || e.FilesAuthorityDenied {
		t.Fatal("duplicate denials were not deduplicated")
	}
	success, _ := json.Marshal(protocol.ThumbnailResult{Found: true, URL: "data:image/png;base64,QQ=="})
	for i := 0; i < 300; i++ {
		observeSnapshotService(e, call, protocol.ServiceReply{Data: success})
	}
	if e.ThumbnailSamples != 1 || e.Overflow {
		t.Fatal("duplicate successful thumbnail exhausted budget")
	}
	for i := 0; i < 201; i++ {
		args, _ = json.Marshal(protocol.PathArgs{Path: fmt.Sprintf("/p%d.png", i)})
		call.Args = args
		observeSnapshotService(e, call, protocol.ServiceReply{Data: success})
	}
	if !e.Overflow || e.ThumbnailSamples != 200 {
		t.Fatalf("unbounded evidence: %+v", e)
	}
}

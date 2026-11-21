package builtin

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"path/filepath"
	"testing"
)

type pictureSample struct{ sample }

func (*pictureSample) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	ref, _ := ctx.Desktop().Files().Thumbnail(in.Ref.Args["path"])
	return api.Result{View: &api.View{Kind: api.ViewGrid, Items: []api.Item{{ID: in.Ref.Args["path"], Image: &api.Image{Src: ref}}}}}, nil
}

func TestBuiltInEvidenceBelongsToOneInvocation(t *testing.T) {
	r := newSample(t, &pictureSample{}, func(call protocol.ServiceCall) protocol.ServiceReply {
		var args protocol.PathArgs
		_ = json.Unmarshal(call.Args, &args)
		data, _ := json.Marshal(protocol.ThumbnailResult{Found: true, URL: "issued:" + args.Path})
		return protocol.ServiceReply{Data: data}
	})
	first, second := filepath.Join(t.TempDir(), "one.png"), filepath.Join(t.TempDir(), "two.png")
	results := make(chan protocol.Response, 2)
	for _, path := range []string{first, second} {
		go func() {
			response, err := r.Call(context.Background(), protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "grid", Args: map[string]string{"path": path}}})
			if err != nil {
				panic(err)
			}
			results <- response
		}()
	}
	for range 2 {
		response := <-results
		var result api.Result
		if err := json.Unmarshal(response.Data, &result); err != nil || result.View == nil {
			t.Fatalf("built-in result: %v", err)
		}
		item := result.View.Items[0]
		hash := sha256.Sum256([]byte(item.Image.Src))
		if response.SnapshotEvidence == nil || len(response.SnapshotEvidence.ThumbnailHashes) != 1 || len(response.SnapshotEvidence.ThumbnailPaths[hash]) != 1 || response.SnapshotEvidence.ThumbnailPaths[hash][0] != item.ID {
			t.Fatal("built-in calls shared or lost invocation provenance")
		}
	}
}

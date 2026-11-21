package selection

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

type recordedFiles struct {
	target platform.FocusTarget
	calls  int
}

func (r *recordedFiles) Paths(ctx context.Context, target platform.FocusTarget) ([]string, error) {
	r.target = target
	r.calls++
	return []string{"/tmp/a"}, ctx.Err()
}
func TestFilesForwardsCapturedSourceAndCancellation(t *testing.T) {
	r := &recordedFiles{}
	target := platform.FocusTarget{PID: 123}
	e := New(Deps{SelectedFiles: r, Source: func() platform.FocusTarget { return target }})
	paths, err := e.Files(context.Background())
	if err != nil || len(paths) != 1 || r.target != target {
		t.Fatalf("paths %v target %+v error %v", paths, r.target, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = e.Files(ctx)
	if !errors.Is(err, context.Canceled) || r.calls != 1 {
		t.Fatalf("cancel: %v calls %d", err, r.calls)
	}
}

package files

import (
	"context"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestWatchTargetIsTheClosestExistingAncestor ports the nested missing-root case of file-index-watcher.test.ts: a root
// under a missing folder is watched at the nearest folder that exists, and an existing root is watched itself.
func TestWatchTargetIsTheClosestExistingAncestor(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, testHome+"/Pictures/x.png")
	ix, _ := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{testHome + "/Work/Documents"}})
	l := &indexLoop{ix: ix, ctx: context.Background(), watches: map[string]context.CancelFunc{}}
	if got := l.watchTargetFor(testHome + "/Work/Documents"); got != testHome {
		t.Fatalf("watch target for a root under a missing folder = %q, want %q", got, testHome)
	}
	if got := l.watchTargetFor(testHome + "/Pictures"); got != testHome+"/Pictures" {
		t.Fatalf("watch target for an existing root = %q, want the root itself", got)
	}
}

// TestWatchesFollowRootsThatAppearAndDisappear ports the missing-root watcher cases of file-index-watcher.test.ts: a
// missing root is watched through its parent, and once it exists the parent's watch is dropped.
func TestWatchesFollowRootsThatAppearAndDisappear(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, testHome+"/Pictures/x.png")
	ix, _ := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{docs}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l := &indexLoop{ix: ix, ctx: ctx, watches: map[string]context.CancelFunc{}}
	defer func() {
		l.stopWatches()
		ix.watchers.Wait()
	}()

	l.syncWatches()
	if got := watchedTargets(l); !slices.Equal(got, []string{testHome}) {
		t.Fatalf("while Documents is missing, watches = %v, want the home folder", got)
	}

	if err := fsys.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	l.syncWatches()
	if got := watchedTargets(l); !slices.Equal(got, []string{docs}) {
		t.Fatalf("once Documents exists, watches = %v, want only Documents", got)
	}
}

// watchedTargets returns the folders the loop currently watches, sorted.
func watchedTargets(l *indexLoop) []string {
	targets := make([]string, 0, len(l.watches))
	for target := range l.watches {
		targets = append(targets, target)
	}
	slices.Sort(targets)
	return targets
}

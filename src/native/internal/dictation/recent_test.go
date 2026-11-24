package dictation

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/dictation/recordings"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestATranscribedDictationKeepsItsAudioAmongTheRecentOnes(t *testing.T) {
	files := fake.NewFS()
	unfinished := recordings.New(files, "/unfinished", time.Now)
	recent := recordings.New(files, "/recent", time.Now)
	keeper := recentKeeper{Store: unfinished, recent: recent}
	if err := unfinished.Save("dictation-a", [][]byte{{1, 2}, {3}}); err != nil {
		t.Fatal(err)
	}
	if err := keeper.Remove("dictation-a"); err != nil {
		t.Fatal(err)
	}
	if left, _ := unfinished.List(); len(left) != 0 {
		t.Fatalf("unfinished = %+v; want the dictation gone from the history", left)
	}
	if kept, err := recent.Load("dictation-a"); err != nil || len(kept) != 2 {
		t.Fatalf("recent audio = %d segments, %v; want both kept", len(kept), err)
	}
}

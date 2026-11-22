package dictation

import "testing"

func TestALongPassEndsInTheQuietestStretchBeforeTheLimit(t *testing.T) {
	packets := make([][]byte, 400)
	for i := range packets {
		packets[i] = make([]byte, 60)
	}
	for i := 200; i < 230; i++ {
		packets[i] = make([]byte, 4)
	}
	cut := quietestCut(packets, 100, 300)
	if cut < 200 || cut >= 230 {
		t.Fatalf("cut at frame %d; want it inside the quiet stretch at frames 200..229", cut)
	}
	if got := quietestCut(packets, 300, 380); got < 300 || got > 380 {
		t.Fatalf("cut at frame %d; want it inside the searched range", got)
	}
}

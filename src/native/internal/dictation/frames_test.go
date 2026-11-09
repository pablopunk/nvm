package dictation

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestPeakNormalizesSignedSamples(t *testing.T) {
	for _, tc := range []struct {
		samples []int16
		want    float64
	}{
		{nil, 0}, {[]int16{0, -16384, 100}, 0.5}, {[]int16{-32768}, 1},
	} {
		if got := Peak(tc.samples); got != tc.want {
			t.Fatalf("Peak(%v) = %v, want %v", tc.samples, got, tc.want)
		}
	}
}

func TestCaptureMeterUsesAllFramesAndDecays(t *testing.T) {
	encoder, err := fake.NewOpusCodec().NewEncoder(Format())
	if err != nil {
		t.Fatal(err)
	}
	var levels []float64
	op := operation{encoder: encoder, onLevel: func(level float64) { levels = append(levels, level) }}
	frame := make([]int16, samplesPerFrame)
	frame[0] = 8192
	if !op.addFrame(frame) {
		t.Fatal(op.failure)
	}
	clear(frame)
	for range levelEveryFrames - 1 {
		if !op.addFrame(frame) {
			t.Fatal(op.failure)
		}
	}
	for range levelEveryFrames {
		if !op.addFrame(frame) {
			t.Fatal(op.failure)
		}
	}
	if len(levels) != 2 || levels[0] != 0.5 || levels[1] != 0.3 {
		t.Fatalf("levels = %v; want peak from first frame followed by decay", levels)
	}
}

func TestLateCaptureLevelCannotReachNewOperation(t *testing.T) {
	var levels []float64
	s := NewService(Config{OnLevel: func(level float64, _ func() bool) { levels = append(levels, level) }})
	old, current := newOperation(), newOperation()
	defer old.cancel()
	defer current.cancel()
	s.phase, s.cur = PhaseRecording, current
	s.publishOperationLevel(old, 0.9)
	s.publishOperationLevel(current, 0.4)
	s.phase = PhaseTranscribing
	s.publishOperationLevel(current, 0.8)
	if len(levels) != 1 || levels[0] != 0.4 {
		t.Fatalf("levels = %v", levels)
	}
}

func TestLevelCallbackCanReenterServiceAndExpiresOnCancel(t *testing.T) {
	var current func() bool
	var s *Service
	s = NewService(Config{OnLevel: func(_ float64, valid func() bool) {
		_ = s.Snapshot()
		current = valid
		s.Cancel()
	}})
	op := newOperation()
	s.phase, s.cur = PhaseRecording, op
	finished := make(chan struct{})
	go func() {
		s.publishOperationLevel(op, 0.5)
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("level callback deadlocked while reentering the service")
	}
	if current == nil || current() {
		t.Fatal("cancelled operation still reports a current level")
	}
}

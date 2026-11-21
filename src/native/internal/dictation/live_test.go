package dictation

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestSpeechIsCutAtPausesAndSentBeforeTheUserStops(t *testing.T) {
	h := newHarness(t)
	h.transcriber.bySegment = map[int]string{0: "first sentence.", 1: "second sentence."}
	audio := slices.Concat(voice(6), make([]int16, SampleRate*6/10), voice(2))
	h.mic.Queue(audio...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	op := currentOperation(h.service)
	<-op.done
	op.live.wg.Wait()
	if sent := h.transcriber.ids(); len(sent) != 1 {
		t.Fatalf("sent %d segments while recording, want the piece before the pause sent and the last one held for Stop", len(sent))
	}
	text, err := h.service.Stop()
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if text != "first sentence. second sentence." {
		t.Fatalf("text = %q, want the pieces joined in order", text)
	}
}

func TestOnlyTheFirstLiveSegmentUsesThePreparation(t *testing.T) {
	h := newHarness(t)
	h.transcriber.bySegment = map[int]string{}
	audio := slices.Concat(voice(6), make([]int16, SampleRate*6/10), voice(6), make([]int16, SampleRate*6/10), voice(1))
	h.mic.Queue(audio...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := h.service.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	h.transcriber.mu.Lock()
	defer h.transcriber.mu.Unlock()
	if len(h.transcriber.segments) != 3 {
		t.Fatalf("sent %d segments, want 3", len(h.transcriber.segments))
	}
	for _, segment := range h.transcriber.segments {
		if segment.Prepared != (segmentIndex(segment) == 0) {
			t.Fatalf("segment %s prepared = %v, want only the first prepared", segment.OperationID, segment.Prepared)
		}
	}
}

func TestPauseDetectorWaitsForEnoughSpeechAndAPause(t *testing.T) {
	loud, quiet := make([]int16, samplesPerFrame), make([]int16, samplesPerFrame)
	for i := range loud {
		loud[i] = 9000
	}
	feed := func(p *pauseDetector, frames, start int, frame []int16) (int, bool) {
		for i := range frames {
			if p.cut(frame, start+i+1) {
				return start + i + 1, true
			}
		}
		return start + frames, false
	}
	var p pauseDetector
	frames, cut := feed(&p, 25, 0, quiet)
	frames, cut2 := feed(&p, 100, frames, loud)
	_, cut3 := feed(&p, 50, frames, quiet)
	if cut || cut2 || cut3 {
		t.Fatal("cut before the segment held enough speech")
	}
	short := minLiveSegmentMillis / FrameMillis
	var q pauseDetector
	at := 0
	for at < short {
		frame := loud
		if at%2 == 0 {
			frame = quiet
		}
		at++
		if q.cut(frame, at) {
			t.Fatalf("cut inside continuous speech at frame %d", at)
		}
	}
	at, cut = feed(&q, pauseMillis/FrameMillis, at, quiet)
	if !cut || at != short+pauseMillis/FrameMillis {
		t.Fatalf("cut = %v at frame %d, want a cut once the pause lasted %d ms", cut, at, pauseMillis)
	}
}

// segmentIndex reads the segment's position from its operation ID suffix.
func segmentIndex(segment Segment) int {
	suffix, found := strings.CutPrefix(segment.OperationID, segment.CancellationID+"-")
	if !found {
		return 0
	}
	index, _ := strconv.Atoi(suffix)
	return index
}

// voice returns seconds of a signal the pause detector hears as continuous speech: loud and quiet 20 ms frames in turn,
// like syllables, with no gap long enough to end a segment.
func voice(seconds float64) []int16 {
	samples := make([]int16, int(seconds*SampleRate))
	for i := range samples {
		if i/samplesPerFrame%2 == 0 {
			samples[i] = 9000
		} else {
			samples[i] = 300
		}
	}
	return samples
}

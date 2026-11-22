package dictation

import (
	"context"
	"slices"

	"github.com/pablopunk/nvm/native/internal/dictation/webm"
)

// maxPassMillis is the most audio a transcriber that reads the whole recording gets in one request. The on-device
// model's memory grows with the audio it reads (about 2.6 GB for a minute and a half), so a longer dictation is read in
// passes of this length, each opening with the end of the one before, and stitched like live pieces.
const maxPassMillis = 120_000

const maxPassFrames = maxPassMillis / FrameMillis

// pass is one request of a whole-recording read: its WebM file, and how many of its frames repeat the pass before.
type pass struct {
	file    []byte
	overlap int
	frames  int
}

// wholeRecordingPasses joins every segment's packets, whatever cut them, and splits them into passes of at most
// maxPassFrames, so a minute-long dictation is read in one pass and never ends a sentence at the capture's cut.
func wholeRecordingPasses(track webm.Track, segments [][][]byte) []pass {
	packets := slices.Concat(segments...)
	var passes []pass
	for start := 0; start < len(packets); start += maxPassFrames {
		end := min(start+maxPassFrames, len(packets))
		lead := 0
		if start > 0 {
			lead = min(overlapFrames, start)
		}
		passes = append(passes, pass{
			file: webm.Mux(track, packets[start-lead:end], FrameMillis), overlap: lead, frames: end - start,
		})
	}
	return passes
}

// transcribePasses reads the passes in order and stitches their texts at the overlaps. The first pass is the one a
// preparation covers when prepared is set.
func transcribePasses(ctx context.Context, transcriber Transcriber, policy RetryPolicy, operationID string, prepared bool, passes []pass, onRetry func(int, error)) (string, error) {
	var joined string
	var joinedMillis int
	for index, p := range passes {
		text, err := transcribeSegment(ctx, transcriber, policy, Segment{
			OperationID:    segmentOperationID(operationID, index),
			CancellationID: operationID,
			Prepared:       index == 0 && prepared,
			File:           p.file,
		}, onRetry)
		if err != nil {
			return "", err
		}
		if p.overlap > 0 {
			joined = stitch(joined, text, overlapWordEstimate(joined, joinedMillis))
		} else {
			joined = text
		}
		joinedMillis += p.frames * FrameMillis
	}
	return joined, nil
}

// savedPasses reads a saved dictation's segment files back into whole-recording passes.
func savedPasses(files [][]byte) ([]pass, error) {
	var track webm.Track
	segments := make([][][]byte, 0, len(files))
	for _, file := range files {
		fileTrack, packets, err := webm.Demux(file)
		if err != nil {
			return nil, err
		}
		track = fileTrack
		segments = append(segments, packets)
	}
	return wholeRecordingPasses(track, segments), nil
}

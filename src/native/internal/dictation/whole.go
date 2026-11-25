package dictation

import (
	"context"
	"math"
	"strings"

	"github.com/pablopunk/nvm/native/internal/dictation/webm"
)

// pcmReader is a transcriber that reads a dictation whole from its raw samples when it stops, as Hex hands Parakeet
// the microphone's audio: the on-device model reads any length at once with long-audio attention, so no pause or cut
// ends a sentence, and it hears the audio before compression.
type pcmReader interface {
	TranscribePCM(ctx context.Context, pcm []int16) (string, error)
}

func readsWholeRecording(t Transcriber) bool {
	_, ok := t.(pcmReader)
	return ok
}

// The end check. Parakeet sometimes leaves out the last sentence of a long recording read whole (the same audio keeps
// or drops it depending on a few hundred milliseconds of silence after it), so a recording longer than tailCheckMillis
// is read a second time from a pause about tailCheckMillis before its end, and words that reading ends with and the
// whole one lacks are added. A whole reading that already has them is left as it is.
const (
	tailCheckMillis  = 15_000
	tailSearchMillis = 5_000
	tailQuietMillis  = 300
)

// readWhole transcribes pcm in one reading and checks its end, retrying passing failures under policy.
func readWhole(ctx context.Context, reader pcmReader, policy RetryPolicy, pcm []int16, onRetry func(int, error)) (string, error) {
	text, err := readPCM(ctx, reader, policy, pcm, onRetry)
	if err != nil || len(pcm) <= (tailCheckMillis+tailSearchMillis)*SampleRate/1000 {
		return text, err
	}
	start := quietestStart(pcm, len(pcm)-(tailCheckMillis+tailSearchMillis)*SampleRate/1000, len(pcm)-tailCheckMillis*SampleRate/1000)
	tail, err := readPCM(ctx, reader, policy, pcm[start:], onRetry)
	if err != nil {
		return text, nil
	}
	tailWords := len(strings.Fields(text)) * (len(pcm) - start) / len(pcm)
	return stitch(text, tail, tailWords), nil
}

func readPCM(ctx context.Context, reader pcmReader, policy RetryPolicy, pcm []int16, onRetry func(int, error)) (string, error) {
	var text string
	err := policy.do(ctx, func(int) error {
		var err error
		text, err = reader.TranscribePCM(ctx, pcm)
		return err
	}, onRetry)
	return strings.TrimSpace(text), err
}

// quietestStart is the middle of the quietest tailQuietMillis of pcm between from and to, so the end check starts in a
// pause rather than through a word.
func quietestStart(pcm []int16, from, to int) int {
	span := tailQuietMillis * SampleRate / 1000
	step := FrameMillis * SampleRate / 1000
	best, bestEnergy := to, math.Inf(1)
	for start := max(0, from); start+span <= to; start += step {
		var energy float64
		for _, sample := range pcm[start : start+span] {
			energy += float64(sample) * float64(sample)
		}
		if energy <= bestEnergy {
			best, bestEnergy = start+span/2, energy
		}
	}
	return best
}

// decodeFiles turns saved WebM segments back into one stretch of PCM, without each file's encoder delay.
func (s *Service) decodeFiles(files [][]byte) ([]int16, error) {
	if s.cfg.Codec == nil {
		return nil, ErrCodec
	}
	decoder, err := s.cfg.Codec.NewDecoder(Format())
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	var pcm []int16
	for _, file := range files {
		track, packets, err := webm.Demux(file)
		if err != nil {
			return nil, err
		}
		var samples []int16
		for _, packet := range packets {
			frame, err := decoder.Decode(packet)
			if err != nil {
				return nil, err
			}
			samples = append(samples, frame...)
		}
		pcm = append(pcm, samples[min(track.Lookahead, len(samples)):]...)
	}
	return pcm, nil
}

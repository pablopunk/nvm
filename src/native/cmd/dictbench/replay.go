package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/dictation"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// replayMicrophone is a microphone that plays speech at real-time speed, then silence until it is closed.
type replayMicrophone struct {
	speech []int16
}

func (m *replayMicrophone) Devices() ([]platform.MicrophoneDevice, error) {
	return []platform.MicrophoneDevice{{ID: "replay", Title: "Replay", Default: true}}, nil
}

func (m *replayMicrophone) Open(ctx context.Context, _ string, format platform.AudioFormat) (platform.MicrophoneStream, error) {
	if format.SampleRate != dictation.SampleRate || format.Channels != 1 {
		return nil, fmt.Errorf("replay plays 16 kHz mono, asked for %+v", format)
	}
	stream := &replayStream{speech: m.speech, opened: time.Now(), closed: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = stream.Close()
		case <-stream.closed:
		}
	}()
	return stream, nil
}

type replayStream struct {
	speech    []int16
	opened    time.Time
	delivered int
	closed    chan struct{}
	closeOnce sync.Once
}

// Read waits until the wall clock has reached the samples it returns, as a microphone does.
func (s *replayStream) Read(pcm []int16) (int, error) {
	due := s.delivered + len(pcm)
	wait := time.Until(s.opened.Add(time.Duration(due) * time.Second / dictation.SampleRate))
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-s.closed:
			return 0, io.EOF
		}
	}
	select {
	case <-s.closed:
		return 0, io.EOF
	default:
	}
	for i := range pcm {
		position := s.delivered + i
		pcm[i] = 0
		if position < len(s.speech) {
			pcm[i] = s.speech[position]
		}
	}
	s.delivered = due
	return len(pcm), nil
}

func (s *replayStream) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

// loadSpeech reads every clip as 16 kHz mono and joins them with gap of silence in between.
func loadSpeech(paths []string, gap time.Duration) ([]int16, error) {
	var speech []int16
	silence := make([]int16, int(gap.Seconds()*dictation.SampleRate))
	for i, path := range paths {
		clip, err := readWAV(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if i > 0 {
			speech = append(speech, silence...)
		}
		speech = append(speech, clip...)
	}
	return speech, nil
}

// readWAV decodes a 16-bit PCM WAV file, mixes it to mono and resamples it to 16 kHz.
func readWAV(path string) ([]int16, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) < 12 || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return nil, errors.New("not a WAV file")
	}
	var rate, channels, bits int
	var data []byte
	for offset := 12; offset+8 <= len(raw); {
		id, size := string(raw[offset:offset+4]), int(binary.LittleEndian.Uint32(raw[offset+4:offset+8]))
		body := raw[offset+8 : min(len(raw), offset+8+size)]
		switch id {
		case "fmt ":
			if len(body) < 16 || binary.LittleEndian.Uint16(body[0:2]) != 1 {
				return nil, errors.New("only PCM WAV is supported")
			}
			channels = int(binary.LittleEndian.Uint16(body[2:4]))
			rate = int(binary.LittleEndian.Uint32(body[4:8]))
			bits = int(binary.LittleEndian.Uint16(body[14:16]))
		case "data":
			data = body
		}
		offset += 8 + size + size%2
	}
	if bits != 16 || channels < 1 || rate <= 0 || data == nil {
		return nil, fmt.Errorf("need 16-bit PCM, got %d bits, %d channels, %d Hz", bits, channels, rate)
	}
	frames := len(data) / 2 / channels
	mono := make([]float64, frames)
	for frame := range frames {
		var sum float64
		for channel := range channels {
			sum += float64(int16(binary.LittleEndian.Uint16(data[(frame*channels+channel)*2:])))
		}
		mono[frame] = sum / float64(channels)
	}
	return resample(mono, rate), nil
}

// resample converts mono samples at rate to 16 kHz by linear interpolation, which is enough for speech recognition.
func resample(mono []float64, rate int) []int16 {
	if len(mono) == 0 {
		return nil
	}
	count := len(mono) * dictation.SampleRate / rate
	out := make([]int16, count)
	step := float64(rate) / dictation.SampleRate
	for i := range out {
		position := float64(i) * step
		index := int(position)
		next := min(index+1, len(mono)-1)
		fraction := position - float64(index)
		out[i] = int16(mono[index]*(1-fraction) + mono[next]*fraction)
	}
	return out
}

func seconds(samples int) float64 {
	return float64(samples) / dictation.SampleRate
}

// writeWAV writes 16 kHz mono 16-bit PCM.
func writeWAV(path string, pcm []int16) error {
	data := make([]byte, 44+2*len(pcm))
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+2*len(pcm)))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], dictation.SampleRate)
	binary.LittleEndian.PutUint32(data[28:], dictation.SampleRate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(2*len(pcm)))
	for i, sample := range pcm {
		binary.LittleEndian.PutUint16(data[44+2*i:], uint16(sample))
	}
	return os.WriteFile(path, data, 0o644)
}

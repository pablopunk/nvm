// Package parakeet transcribes speech on the device with NVIDIA's Parakeet TDT 0.6B v3 (25 European languages, with
// punctuation), the way Hex runs it: the same Q8_0 GGUF file on transcribe.cpp, which reads a whole recording at
// once with NeMo's long-audio local attention, on the GPU where there is one. The model runs in a child process of the
// app (see child.go), so its memory is given back as soon as dictation no longer needs it.
//
// Layer: platform. The package loads the transcribe.cpp library with purego and imports the standard library.
package parakeet

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
)

// modelFile is the model's one file inside its folder, as Hex downloads it.
const modelFile = "parakeet-tdt-0.6b-v3-Q8_0.gguf"

// Files lists the files a model folder must hold.
func Files() []string {
	return []string{modelFile}
}

// statusOK is transcribe_status TRANSCRIBE_OK.
const statusOK = 0

// minimumSamples is the shortest audio a run gets, padded with silence after the speech, as Hex pads it: 1.5 s.
const minimumSamples = 24000

// library holds the transcribe.cpp functions the model uses.
type library struct {
	open     func(path string, loadParams, sessionParams uintptr, out *uintptr) int32
	run      func(session uintptr, pcm *float32, samples int32, params uintptr) int32
	fullText func(session uintptr) string
	close    func(session uintptr)
	status   func(status int32) string
}

// Model is a loaded Parakeet model. It is not safe for concurrent use.
type Model struct {
	lib     *library
	session uintptr
}

// Load opens the model in dir on the transcribe.cpp library at libraryPath.
func Load(libraryPath, dir string) (*Model, error) {
	lib, err := loadLibrary(libraryPath)
	if err != nil {
		return nil, err
	}
	var session uintptr
	if status := lib.open(filepath.Join(dir, modelFile), 0, 0, &session); status != statusOK || session == 0 {
		return nil, fmt.Errorf("load the speech model: %s", lib.status(status))
	}
	return &Model{lib: lib, session: session}, nil
}

// Close releases the model.
func (m *Model) Close() {
	if m.session != 0 {
		m.lib.close(m.session)
		m.session = 0
	}
}

// Transcribe returns the text of 16 kHz mono samples in -1..1, read whole, as Hex reads a dictation.
func (m *Model) Transcribe(ctx context.Context, samples []float32) (string, error) {
	if len(samples) == 0 {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if m.session == 0 {
		return "", errors.New("the speech model is closed")
	}
	if len(samples) < minimumSamples {
		samples = append(samples, make([]float32, minimumSamples-len(samples))...)
	}
	if status := m.lib.run(m.session, &samples[0], int32(len(samples)), 0); status != statusOK {
		return "", fmt.Errorf("transcribe: %s", m.lib.status(status))
	}
	return m.lib.fullText(m.session), nil
}

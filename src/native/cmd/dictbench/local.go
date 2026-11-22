package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/parakeet"
)

// runLocal loads the on-device model in this process and transcribes the clip whole, reporting the load time and how
// much faster than real time the transcription ran.
func runLocal(library, dir string, speech []int16, stdout io.Writer) error {
	began := time.Now()
	model, err := parakeet.Load(library, dir)
	if err != nil {
		return err
	}
	defer model.Close()
	fmt.Fprintf(stdout, "local: model loaded in %d ms\n", time.Since(began).Milliseconds())
	samples := make([]float32, len(speech))
	for i, sample := range speech {
		samples[i] = float32(sample) / 32768
	}
	for run := range 3 {
		began = time.Now()
		text, err := model.Transcribe(context.Background(), samples)
		if err != nil {
			return err
		}
		took := time.Since(began)
		fmt.Fprintf(stdout, "local run %d: %.1f s of audio in %d ms (%.0fx real time)\n  text: %s\n",
			run+1, seconds(len(speech)), took.Milliseconds(), seconds(len(speech))/took.Seconds(), text)
	}
	return nil
}

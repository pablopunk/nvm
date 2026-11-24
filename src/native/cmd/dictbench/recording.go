package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pablopunk/nvm/native/internal/dictation"
	"github.com/pablopunk/nvm/native/internal/dictation/devicemodel"
	"github.com/pablopunk/nvm/native/internal/dictation/recordings"
	"github.com/pablopunk/nvm/native/internal/dictation/webm"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// replayRecording transcribes a saved dictation on the device the way a History retry does, then each of its segments
// alone, so a transcript that lost words can be traced to the audio or to the transcription.
func replayRecording(library, modelsRoot, dir, id string, stdout io.Writer) error {
	ports := real.NewPorts()
	model := devicemodel.New(ports.FS, ports.Downloads, modelsRoot, devicemodel.Parakeet())
	device := dictation.NewDeviceTranscriber(real.NewSpeechRecognizer(library), ports.Opus, model)
	store := recordings.New(ports.FS, dir, time.Now)
	segments, err := store.Load(id)
	if err != nil {
		return err
	}
	for index, file := range segments {
		track, packets, err := webm.Demux(file)
		if err != nil {
			return fmt.Errorf("segment %d: %w", index, err)
		}
		fmt.Fprintf(stdout, "segment %d: %d packets (%.1f s), track %+v\n", index, len(packets), float64(len(packets)*dictation.FrameMillis)/1000, track)
	}
	service := dictation.NewService(dictation.Config{Recorder: store})
	began := time.Now()
	text, err := service.RetryOn(context.Background(), id, device)
	fmt.Fprintf(stdout, "\nwhole recording (%d ms, err %v):\n%s\n", time.Since(began).Milliseconds(), err, text)
	for index, file := range segments {
		text, err := device.Transcribe(context.Background(), dictation.Segment{OperationID: fmt.Sprintf("seg-%d", index), File: file})
		fmt.Fprintf(stdout, "\nsegment %d alone (err %v):\n%s\n", index, err, text)
	}
	device.Finished("")
	return nil
}

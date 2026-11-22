package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/dictation"
	"github.com/pablopunk/nvm/native/internal/dictation/devicemodel"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// runDevice measures dictations on the device through the real service: the model loads in a child process when the
// microphone opens, each piece is transcribed at its pause, and the release waits only for the last one. modelsRoot
// holds the model folder the manifest names.
func runDevice(library, modelsRoot string, speech []int16, runs int, release time.Duration, stdout, stderr io.Writer) error {
	ports := real.NewPorts()
	model := devicemodel.New(ports.FS, ports.Downloads, modelsRoot, devicemodel.Parakeet())
	if !model.Ready() {
		return fmt.Errorf("the model is not in %s", filepath.Join(modelsRoot, devicemodel.Parakeet().Name))
	}
	device := dictation.NewDeviceTranscriber(real.NewSpeechRecognizer(library), ports.Opus, model)
	var results []runResult
	for run := range runs {
		service := dictation.NewService(dictation.Config{
			Microphone: &replayMicrophone{speech: speech},
			Codec:      ports.Opus,
			Warn: func(event string, fields map[string]any) {
				fmt.Fprintln(stderr, "warn:", event, fields)
			},
		})
		if err := service.StartOn(device, "", nil); err != nil {
			return err
		}
		time.Sleep(time.Duration(seconds(len(speech))*float64(time.Second)) + release)
		released := time.Now()
		text, err := service.Stop()
		result := runResult{ReleaseToTextMs: time.Since(released).Milliseconds(), Text: text}
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
		printRun(stdout, run+1, result)
	}
	printSummary(stdout, results)
	fmt.Fprintf(stdout, "speech children running now: %d\n", countSpeechChildren())
	time.Sleep(35 * time.Second)
	fmt.Fprintf(stdout, "speech children running 35 s later: %d\n", countSpeechChildren())
	return nil
}

// countSpeechChildren counts this process's children that hold the model, from /proc where there is one.
func countSpeechChildren() int {
	entries, _ := os.ReadDir("/proc")
	count := 0
	for _, entry := range entries {
		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err == nil && containsArg(cmdline, "--nvm-speech-child") {
			count++
		}
	}
	return count
}

func containsArg(cmdline []byte, arg string) bool {
	start := 0
	for i, b := range cmdline {
		if b == 0 {
			if string(cmdline[start:i]) == arg {
				return true
			}
			start = i + 1
		}
	}
	return false
}

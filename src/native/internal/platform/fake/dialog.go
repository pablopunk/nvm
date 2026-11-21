package fake

import (
	"errors"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// FileDialog is an in-memory platform.FileDialog. Each Choose records its request and answers with the next queued
// answer; with nothing queued the user cancels. Tests queue answers with Answer and failures with FailNext.
type FileDialog struct {
	mu       sync.Mutex
	requests []platform.ChooseRequest
	answers  []dialogAnswer
}

type dialogAnswer struct {
	paths []string
	err   error
}

// NewFileDialog returns a dialog with no queued answers.
func NewFileDialog() *FileDialog {
	return &FileDialog{}
}

// Answer queues one answer: the paths the user picks.
func (d *FileDialog) Answer(paths ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.answers = append(d.answers, dialogAnswer{paths: paths})
}

// FailNext queues one failure for the next dialog.
func (d *FileDialog) FailNext(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.answers = append(d.answers, dialogAnswer{err: errors.New(err.Error())})
}

// Requests returns the dialogs shown so far, oldest first.
func (d *FileDialog) Requests() []platform.ChooseRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]platform.ChooseRequest(nil), d.requests...)
}

// Choose records the request and answers with the next queued answer.
func (d *FileDialog) Choose(req platform.ChooseRequest) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requests = append(d.requests, req)
	if len(d.answers) == 0 {
		return nil, nil
	}
	answer := d.answers[0]
	d.answers = d.answers[1:]
	return answer.paths, answer.err
}

var _ platform.FileDialog = (*FileDialog)(nil)

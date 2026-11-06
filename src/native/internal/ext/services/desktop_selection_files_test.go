package services

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

type filesSelection struct{ calls int }

func (s *filesSelection) Text() (string, bool, error)  { return "text", true, nil }
func (s *filesSelection) Replace(string) (bool, error) { return true, nil }
func (s *filesSelection) Undo() error                  { return nil }
func (s *filesSelection) Files(context.Context) ([]string, error) {
	s.calls++
	return []string{"/tmp/a"}, nil
}
func TestSelectionFilesCapabilityAndArguments(t *testing.T) {
	s := &filesSelection{}
	h := desktopHandler(t, Deps{Selection: s}, api.CapabilityDesktopFiles)
	mustOK(t, serve(t, h, protocol.ServiceDesktopSelection, protocol.OpFiles, nil))
	if s.calls != 1 {
		t.Fatal("files not called")
	}
	if reply := serve(t, h, protocol.ServiceDesktopSelection, protocol.OpFiles, map[string]int{"pid": 42}); reply.Failure == nil {
		t.Fatal("accepted target")
	}
	if reply := serve(t, h, protocol.ServiceDesktopSelection, protocol.OpText, nil); reply.Failure == nil {
		t.Fatal("text allowed without selection capability")
	}
	if reply := serve(t, h, protocol.ServiceDesktopSelection, protocol.OpReplace, map[string]string{"text": "x"}); reply.Failure == nil {
		t.Fatal("replace allowed without selection capability")
	}
	other := desktopHandler(t, Deps{Selection: s}, api.CapabilityDesktopSelection)
	if reply := serve(t, other, protocol.ServiceDesktopSelection, protocol.OpFiles, nil); reply.Failure == nil {
		t.Fatal("files allowed without files capability")
	}
}

package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

func TestMessageCopyUsesChosenTextAndRejectsChangedTranscript(t *testing.T) {
	view := chatFixtureView()
	var copied []string
	s := NewSession(Deps{CopyText: func(_ search.Host, text string) { copied = append(copied, text) }}, "acme.chat", view)
	host := &fakeHost{}
	copyMessage := s.messageCopier()
	copyMessage(host, 1)
	copyMessage(host, 2)
	if len(copied) != 2 || copied[0] != "What is in this picture?" || copied[1] != "A window." {
		t.Fatalf("copied = %q", copied)
	}
	s.view.Chat.Messages[2].Content = "Changed"
	copyMessage(host, 2)
	if len(copied) != 2 || len(host.toasts) != 1 {
		t.Fatalf("stale action copied %q, toasts %q", copied, host.toasts)
	}
}

func TestMessageCopyRejectsReplacementConversation(t *testing.T) {
	view := chatFixtureView()
	copies := 0
	s := NewSession(Deps{CopyText: func(search.Host, string) { copies++ }}, "acme.chat", view)
	copyMessage := s.messageCopier()
	s.view = chatFixtureView()
	host := &fakeHost{}
	copyMessage(host, 1)
	if copies != 0 || len(host.toasts) != 1 {
		t.Fatalf("copies = %d, toasts = %q", copies, host.toasts)
	}
}

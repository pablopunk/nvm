package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestChatPaletteCombinesMessageCopyWithExistingExtensionActions(t *testing.T) {
	view := chatFixtureView()
	view.Actions = []api.Action{api.Run("existing", "Existing Chat Action", nil)}
	var copied string
	s := NewSession(Deps{CopyText: func(_ search.Host, text string) { copied = text }}, "acme.chat", view)
	page := s.Palette()
	if page.Actions == nil {
		t.Fatal("chat palette has no action menu")
	}
	existing, copyActions := 0, 0
	for _, action := range page.Actions() {
		switch action.Title {
		case "Existing Chat Action":
			existing++
		case "Copy message 2: What is in this picture?":
			copyActions++
			action.Run(&fakeHost{})
		}
	}
	if existing != 1 || copyActions != 1 || copied != "What is in this picture?" {
		t.Fatalf("existing = %d, copy = %d, text = %q", existing, copyActions, copied)
	}
}

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

func TestCopyMetadataIsReusedAndReplacementInvalidatesEvenReusedChat(t *testing.T) {
	s := NewSession(Deps{CopyText: func(search.Host, string) { t.Error("stale action copied") }}, "acme.chat", chatFixtureView())
	first := s.chatSpec()
	state := s.chatCopies
	second := s.chatSpec()
	if state != s.chatCopies || first.CopyRevision != second.CopyRevision {
		t.Fatal("unchanged render rebuilt copy metadata")
	}
	s.Replace(s.view)
	current := s.chatSpec()
	if current.CopyRevision == first.CopyRevision {
		t.Fatal("replacement retained old action revision")
	}
	host := &fakeHost{}
	first.CopyMessage(host, 1)
	if len(host.toasts) != 1 {
		t.Fatal("stale revision was not rejected")
	}
}

func TestCopyPreservesExactTextAndRejectsDismissedOrMutatedOtherMessage(t *testing.T) {
	view := chatFixtureView()
	view.Chat.Messages[1].Content = "\tα \n beta  "
	var copied []string
	s := NewSession(Deps{CopyText: func(_ search.Host, text string) { copied = append(copied, text) }}, "acme.chat", view)
	copyMessage := s.messageCopier()
	host := &fakeHost{}
	copyMessage(host, 1)
	if len(copied) != 1 || copied[0] != "\tα \n beta  " {
		t.Fatalf("copied text = %q", copied)
	}
	s.view.Chat.Messages[2].Content = "other message changed"
	copyMessage(host, 1)
	s.dismissed = true
	copyMessage(host, 1)
	if len(copied) != 1 || len(host.toasts) != 2 {
		t.Fatalf("stale or dismissed action copied: %q, %q", copied, host.toasts)
	}
}

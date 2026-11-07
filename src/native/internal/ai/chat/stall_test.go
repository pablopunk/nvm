package chat

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// TestSilentReplySettlesWithANotice is the owner's report: the builder sends on Smart, the reply opens and then no
// byte arrives. Once the backend's idle window passes, the turn ends with the timeout notice instead of a spinner that
// never stops, and the empty reply is gone.
func TestSilentReplySettlesWithANotice(t *testing.T) {
	f, transport := pipeFixture(t)
	record, err := f.svc.NewChat(KindBuilder, TierSmart)
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}
	mustSend(t, f, record.ID, "toggle system theme")
	defer func() { _ = nextReply(t, transport).Close() }()
	f.set.Clock.AwaitTimerDueIn(backend.StreamIdleTimeout)
	f.set.Clock.Advance(backend.StreamIdleTimeout)
	f.awaitTurn(record.ID)
	if notice := lastNotice(t, f, record.ID); notice.Kind != NoticeTimeout {
		t.Fatalf("notice = %+v, want the timeout notice", notice)
	}
	if messages := f.messages(record.ID); len(messages) != 2 {
		t.Fatalf("transcript = %+v, want the question and the notice", messages)
	}
	assertTurnLogged(t, f, "timeout")
}

// assertTurnLogged finds the warn line of a failed turn whose backend failure kind is kind.
func assertTurnLogged(t *testing.T, f *fixture, kind string) {
	t.Helper()
	for _, entry := range f.logger.Recent(0) {
		if entry.Level == logx.Warn && entry.Message == "chat turn failed" && entry.Fields["kind"] == kind {
			return
		}
	}
	t.Fatalf("no failed-turn line of kind %s in %+v", kind, f.logger.Recent(0))
}

// TestRestartSettlesATurnAQuitCutOff opens the service over a chat whose reply was still streaming when the app
// quit. No turn runs it any more, so the service ends it as failed instead of showing a spinner forever.
func TestRestartSettlesATurnAQuitCutOff(t *testing.T) {
	f := newFixture(t)
	record := Chat{ID: "chat-cut-off", Kind: KindConversation, Tier: TierFast, Messages: []Message{
		{Author: AuthorUser, Text: "question", Status: StatusComplete},
		{Author: AuthorAssistant, Status: StatusStreaming},
	}}
	if err := f.store.Insert(record); err != nil {
		t.Fatalf("seed: %v", err)
	}
	restarted := New(f.svc.opts)
	chat, _ := restarted.Chat(record.ID)
	if Busy(chat) {
		t.Fatalf("transcript = %+v, want no streaming reply after a restart", chat.Messages)
	}
	if last := chat.Messages[len(chat.Messages)-1]; last.Author != AuthorNotice || last.Notice.Kind != NoticeFailed {
		t.Fatalf("transcript ends with %+v, want the failed notice", last)
	}
}

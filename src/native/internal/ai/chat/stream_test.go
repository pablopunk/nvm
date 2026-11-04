package chat

import (
	"io"
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// pipeFixture is a fixture whose chat replies arrive frame by frame from the test.
func pipeFixture(t *testing.T) (*fixture, *pipeTransport) {
	t.Helper()
	transport := newPipeTransport()
	f := &fixture{t: t, set: fake.NewSet(), connected: true}
	f.build(transport)
	return f, transport
}

// nextReply returns the writer of the next chat reply the service opened.
func nextReply(t *testing.T, transport *pipeTransport) io.WriteCloser {
	t.Helper()
	select {
	case writer := <-transport.bodies:
		return writer
	case <-t.Context().Done():
		t.Fatal("no reply was requested")
	}
	return nil
}

// writeFrames sends frames on a reply as a backend would, one at a time.
func writeFrames(t *testing.T, writer io.Writer, frames []string) {
	t.Helper()
	for _, frame := range frames {
		if _, err := io.WriteString(writer, frame); err != nil {
			t.Fatalf("write frame: %v", err)
		}
	}
}

// TestBusyChatRefusesASecondMessage checks that a second message waits for the reply in flight.
func TestBusyChatRefusesASecondMessage(t *testing.T) {
	f, transport := pipeFixture(t)
	chat := f.newChat()
	mustSend(t, f, chat.ID, "first")
	reply := nextReply(t, transport)
	if err := f.svc.Send(chat.ID, "second"); err != ErrBusy {
		t.Fatalf("Send while answering = %v, want ErrBusy", err)
	}
	writeFrames(t, reply, append(fixtures.StreamFrames("Done.", 24), fixtures.DoneFrame))
	f.awaitTurn(chat.ID)
}

// TestStopKeepsTheTextAndResumeContinuesTheChat is the stop and resume journey: a reply is stopped while it is
// still arriving, the text it had stays marked stopped, and the next message sends that text back as history.
func TestStopKeepsTheTextAndResumeContinuesTheChat(t *testing.T) {
	f, transport := pipeFixture(t)
	chat := f.newChat()
	mustSend(t, f, chat.ID, "first")
	reply := nextReply(t, transport)
	writeFrames(t, reply, fixtures.StreamFrames("Hello", 24))
	f.awaitText(chat.ID, "Hello")
	f.svc.Stop(chat.ID)
	f.awaitTurn(chat.ID)
	messages := f.messages(chat.ID)
	if len(messages) != 2 || messages[1].Text != "Hello" || messages[1].Status != StatusStopped {
		t.Fatalf("transcript after stop = %+v, want the partial reply marked stopped", messages)
	}
	if len(f.settled) != 1 || f.settled[0].Ending != EndStopped {
		t.Fatalf("settled = %+v, want one stopped turn", f.settled)
	}

	mustSend(t, f, chat.ID, "second")
	resumed := nextReply(t, transport)
	writeFrames(t, resumed, append(fixtures.StreamFrames("Back again.", 24), fixtures.DoneFrame))
	f.awaitTurn(chat.ID)
	if got := roles(t, transport.body(1)); len(got) != 4 || got[2] != "assistant" {
		t.Fatalf("resumed request roles = %v, want the stopped reply sent back", got)
	}
	messages = f.messages(chat.ID)
	if len(messages) != 4 || messages[3].Text != "Back again." || messages[3].Status != StatusComplete {
		t.Fatalf("transcript after resume = %+v, want the new reply complete", messages)
	}
}

// TestStoppingBeforeTheFirstWordRemovesTheEmptyReply checks a stop with no text leaves no empty bubble.
func TestStoppingBeforeTheFirstWordRemovesTheEmptyReply(t *testing.T) {
	f, transport := pipeFixture(t)
	chat := f.newChat()
	mustSend(t, f, chat.ID, "first")
	nextReply(t, transport)
	f.svc.Stop(chat.ID)
	f.awaitTurn(chat.ID)
	if messages := f.messages(chat.ID); len(messages) != 1 || messages[0].Author != AuthorUser {
		t.Fatalf("transcript = %+v, want the question alone", messages)
	}
}

// TestFinishedReplyWithoutTextIsAFailure checks a reply that ends with no text says so, rather than showing an
// empty answer.
func TestFinishedReplyWithoutTextIsAFailure(t *testing.T) {
	f, transport := pipeFixture(t)
	chat := f.newChat()
	mustSend(t, f, chat.ID, "first")
	writeFrames(t, nextReply(t, transport), []string{fixtures.DoneFrame})
	f.awaitTurn(chat.ID)
	if notice := lastNotice(t, f, chat.ID); notice.Title != "No reply" {
		t.Fatalf("notice = %+v, want the no-reply notice", notice)
	}
}

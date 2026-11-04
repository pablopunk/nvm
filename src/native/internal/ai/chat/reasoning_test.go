package chat

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// TestLongReasoningStreamsBeforeTheReplyAndIsKept checks a reasoning model that thinks for minutes before it answers:
// the reasoning text lands on the reply as it streams, the reply stays empty meanwhile, the turn is not ended by the
// quiet of the reply text, and the reasoning is kept once the answer arrives. The start and end of the reasoning and
// the end of the turn are logged.
func TestLongReasoningStreamsBeforeTheReplyAndIsKept(t *testing.T) {
	f, transport := pipeFixture(t)
	record := f.newChat()
	mustSend(t, f, record.ID, "toggle system theme")
	writer := nextReply(t, transport)
	defer func() { _ = writer.Close() }()

	thinking := strings.Repeat("weighing the theme options. ", 40)
	for minute := 0; minute < 6; minute++ {
		writeFrames(t, writer, fixtures.ReasoningFrames(thinking, 256))
		f.set.Clock.Advance(30 * time.Second)
		f.q.drain()
	}
	midway := f.messages(record.ID)
	if got := midway[len(midway)-1]; got.Text != "" || got.Reasoning == "" || got.Status != StatusStreaming {
		t.Fatalf("reply while reasoning = %+v, want no text yet and the reasoning streaming", got)
	}

	writeFrames(t, writer, fixtures.StreamFrames("Done: the theme follows the system.", 16))
	writeFrames(t, writer, []string{fixtures.DoneFrame})
	f.awaitTurn(record.ID)
	reply := f.messages(record.ID)[1]
	if reply.Text != "Done: the theme follows the system." || reply.Status != StatusComplete {
		t.Fatalf("reply = %+v, want the answer, complete", reply)
	}
	if !strings.HasPrefix(reply.Reasoning, "weighing the theme options.") {
		t.Fatalf("reasoning = %q..., want the streamed reasoning kept", truncateBytes(reply.Reasoning, 40))
	}
	assertLogged(t, f, "chat reasoning started")
	assertLogged(t, f, "chat reasoning ended")
	assertTurnEndLogged(t, f, "finished")
}

// TestReasoningIsCappedAtTheStoredLimit checks a reasoning longer than MaxReasoningBytes keeps only the limit, so a
// long thought cannot grow the transcript without bound.
func TestReasoningIsCappedAtTheStoredLimit(t *testing.T) {
	f, transport := pipeFixture(t)
	record := f.newChat()
	mustSend(t, f, record.ID, "think hard")
	writer := nextReply(t, transport)
	defer func() { _ = writer.Close() }()

	writeFrames(t, writer, fixtures.ReasoningFrames(strings.Repeat("x", MaxReasoningBytes+4096), 4096))
	writeFrames(t, writer, append(fixtures.StreamFrames("Answer.", 16), fixtures.DoneFrame))
	f.awaitTurn(record.ID)
	if got := len(f.messages(record.ID)[1].Reasoning); got != MaxReasoningBytes {
		t.Fatalf("kept %d bytes of reasoning, want %d; transcript = %+v", got, MaxReasoningBytes, f.messages(record.ID))
	}
}

// TestTurnPastItsDeadlineEndsWithTheDeadlineNotice checks a turn that keeps reasoning past the turn limit ends with a
// clear notice that the reply took too long, and the failure is logged. Time spent waiting for the user does not count.
func TestTurnPastItsDeadlineEndsWithTheDeadlineNotice(t *testing.T) {
	f, transport := pipeFixture(t)
	record := f.newChat()
	mustSend(t, f, record.ID, "a very long think")
	writer := nextReply(t, transport)
	defer func() { _ = writer.Close() }()

	for step := 0; step < 24 && f.svc.Busy(record.ID); step++ {
		if !writeFramesUnlessClosed(writer, fixtures.ReasoningFrames("still thinking", 64)) {
			break
		}
		f.set.Clock.Advance(30 * time.Second)
		f.q.drain()
	}
	f.awaitTurn(record.ID)
	notice := lastNotice(t, f, record.ID)
	if notice.Kind != NoticeTimeout || notice.Title != "Reply took too long" {
		t.Fatalf("notice = %+v, want the deadline notice", notice)
	}
	assertTurnEndLogged(t, f, "failed")
}

// assertLogged fails unless the app log holds an info line with message.
func assertLogged(t *testing.T, f *fixture, message string) {
	t.Helper()
	for _, entry := range f.logger.Recent(0) {
		if entry.Level == logx.Info && entry.Message == message {
			return
		}
	}
	t.Fatalf("no %q line in the log", message)
}

// assertTurnEndLogged fails unless the app log holds the end of a turn with the given ending.
func assertTurnEndLogged(t *testing.T, f *fixture, ending string) {
	t.Helper()
	for _, entry := range f.logger.Recent(0) {
		if entry.Level == logx.Info && entry.Message == "chat turn ended" && entry.Fields["ending"] == ending {
			return
		}
	}
	t.Fatalf("no chat turn ended line with ending %q in %+v", ending, f.logger.Recent(0))
}

// writeFramesUnlessClosed writes frames to a reply and reports false once the reply is closed, as a cancelled or
// timed-out turn closes it.
func writeFramesUnlessClosed(writer io.Writer, frames []string) bool {
	for _, frame := range frames {
		if _, err := io.WriteString(writer, frame); err != nil {
			return false
		}
	}
	return true
}

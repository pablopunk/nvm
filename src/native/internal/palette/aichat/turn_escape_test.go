package aichat

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// refusingTransport answers the active-model route as usual and every chat request with the 402 insufficient-credits
// body, so the turn is refused before any reply streams.
type refusingTransport struct{}

func (refusingTransport) Do(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, fixtures.ActiveModelPath) {
		return respond(req, http.StatusOK, fixtures.ActiveModelBody), nil
	}
	return respond(req, http.StatusPaymentRequired, fixtures.InsufficientCreditsBody), nil
}

// respond builds a JSON response with status and body for req.
func respond(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}

// TestEscapeAfterACompletedTurnGoesBack finishes a reply and checks the turn has ended, so Escape is no longer a stop
// request: it goes back to the root.
func TestEscapeAfterACompletedTurnGoesBack(t *testing.T) {
	transport := fixtures.NewStreamingTransport()
	r := newRig(t, streamingClient(t, transport))
	record := r.chatWith("", chat.TierFast)
	r.palette.Show()
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Type("say done")
	tt.Key(0, ui.KeyEnter)
	reply := awaitReply(t, transport)
	for _, frame := range append(fixtures.StreamFrames("All done.", 24), fixtures.DoneFrame) {
		writeFrame(t, reply, frame)
	}
	_ = reply.Close()
	r.awaitIdle(record.ID)
	if lastReply(mustChat(t, r, record.ID)) != "All done." {
		t.Fatal("the reply did not complete")
	}

	tt.Key(0, ui.KeyEscape)
	if r.palette.Top() != r.root {
		t.Fatal("Escape after a completed reply should go back to the root")
	}
}

// TestEscapeAfterARefusedTurnGoesBack refuses a turn with 402 and checks the refusal is in the transcript and the
// turn has ended, so Escape goes back to the root.
func TestEscapeAfterARefusedTurnGoesBack(t *testing.T) {
	r := newRig(t, streamingClient(t, refusingTransport{}))
	record := r.chatWith("", chat.TierFast)
	r.palette.Show()
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Type("refused")
	tt.Key(0, ui.KeyEnter)
	r.awaitIdle(record.ID)
	messages := mustChat(t, r, record.ID).Messages
	if last := messages[len(messages)-1]; last.Author != chat.AuthorNotice {
		t.Fatalf("the refusal should end the transcript as a notice, got %+v", last)
	}

	tt.Key(0, ui.KeyEscape)
	if r.palette.Top() != r.root {
		t.Fatal("Escape after a refused turn should go back to the root")
	}
}

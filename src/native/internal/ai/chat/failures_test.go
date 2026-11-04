package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// lastNotice returns the notice at the end of the chat's transcript, failing the test when there is none.
func lastNotice(t *testing.T, f *fixture, id string) Notice {
	t.Helper()
	messages := f.messages(id)
	last := messages[len(messages)-1]
	if last.Author != AuthorNotice || last.Notice == nil {
		t.Fatalf("the transcript ends with %+v, want a notice", last)
	}
	return *last.Notice
}

// sendFailing sends one message to a fixture whose backend has been switched to a failure mode, and waits for the
// turn to settle.
func sendFailing(t *testing.T, f *fixture) string {
	t.Helper()
	chat := f.newChat()
	mustSend(t, f, chat.ID, "question")
	f.awaitTurn(chat.ID)
	return chat.ID
}

// TestFailureModesEndWithTheirOwnNotice runs the backend's HTTP failure modes and checks each one ends the turn
// with a notice that names it, leaves no empty reply behind, and reports the error to the account.
func TestFailureModesEndWithTheirOwnNotice(t *testing.T) {
	cases := []struct {
		name string
		mode fixtures.FailureMode
		kind NoticeKind
		want string
	}{
		{"unauthorized", fixtures.ModeUnauthorized, NoticeSignIn, "Sign in again"},
		{"insufficient credits", fixtures.ModeInsufficientCredits, NoticeCredits, "Credits needed"},
		{"rate limited", fixtures.ModeRateLimited, NoticeRateLimited, "Too many requests"},
		{"server error", fixtures.ModeServerError, NoticeFailed, "Reply failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.backend.SetMode(tc.mode)
			id := sendFailing(t, f)
			notice := lastNotice(t, f, id)
			if notice.Kind != tc.kind || notice.Title != tc.want {
				t.Fatalf("notice = %+v, want kind %s titled %q", notice, tc.kind, tc.want)
			}
			if got := f.messages(id); len(got) != 2 || got[0].Author != AuthorUser {
				t.Fatalf("transcript = %+v, want the question and the notice only", got)
			}
			if len(f.noted) != 1 {
				t.Fatalf("the account saw %d errors, want 1", len(f.noted))
			}
		})
	}
}

// TestRateLimitedNoticeNamesTheWait checks the wait the backend asked for reaches the copy.
func TestRateLimitedNoticeNamesTheWait(t *testing.T) {
	f := newFixture(t)
	f.backend.SetMode(fixtures.ModeRateLimited)
	id := sendFailing(t, f)
	if msg := lastNotice(t, f, id).Message; !strings.Contains(msg, "Try again in 30 seconds") {
		t.Fatalf("message = %q, want the 30 second wait", msg)
	}
}

// TestSpendCapBlockedRouteIsRefusedBeforeTheReply checks a route the backend has paused: no reply request is sent,
// the spend-cap notice shows, and the account notes the refusal.
func TestSpendCapBlockedRouteIsRefusedBeforeTheReply(t *testing.T) {
	f := newFixture(t)
	f.backend.SetActiveModel(strings.Replace(fixtures.ActiveModelBody,
		`"notice":"ok","noticeReason":""`, `"notice":"blocked","noticeReason":"free_spend_cap_reached"`, 1))
	id := sendFailing(t, f)
	if notice := lastNotice(t, f, id); notice.Kind != NoticeSpendCap {
		t.Fatalf("notice = %+v, want the spend-cap notice", notice)
	}
	if n := countRequests(f, fixtures.ChatCompletionsPath); n != 0 {
		t.Fatalf("%d chat requests were sent on a paused route", n)
	}
	if len(f.noted) != 1 {
		t.Fatalf("the account saw %d errors, want the spend-cap refusal", len(f.noted))
	}
}

// TestUnsupportedWireFormatIsRefusedWithAnUpdateNotice checks a route this version cannot read is never streamed.
func TestUnsupportedWireFormatIsRefusedWithAnUpdateNotice(t *testing.T) {
	f := newFixture(t)
	f.backend.SetActiveModel(strings.Replace(fixtures.ActiveModelBody, `"api":"openai-completions"`, `"api":"openai-responses"`, 1))
	id := sendFailing(t, f)
	if notice := lastNotice(t, f, id); notice.Kind != NoticeUpdate {
		t.Fatalf("notice = %+v, want the update notice", notice)
	}
	if n := countRequests(f, fixtures.ChatCompletionsPath); n != 0 {
		t.Fatalf("%d chat requests were sent on a route this version cannot read", n)
	}
}

// TestServerTimeoutNamesTheTimeout advances the fake clock past the route lookup's deadline, so the call fails as
// a timeout rather than hanging.
func TestServerTimeoutNamesTheTimeout(t *testing.T) {
	f := newFixture(t)
	f.backend.SetMode(fixtures.ModeTimeout)
	chat := f.newChat()
	mustSend(t, f, chat.ID, "question")
	<-f.backend.Hanging()
	f.set.Clock.AwaitTimerDueIn(10 * time.Second)
	f.set.Clock.Advance(10 * time.Second)
	f.awaitTurn(chat.ID)
	if notice := lastNotice(t, f, chat.ID); notice.Kind != NoticeTimeout {
		t.Fatalf("notice = %+v, want the timeout notice", notice)
	}
}

// TestBrokenStreamsKeepTheTextAndMarkItCutOff runs the two stream failures the backend can produce after a reply
// has started. The text that arrived stays, marked stopped, and a notice says the reply was cut off.
func TestBrokenStreamsKeepTheTextAndMarkItCutOff(t *testing.T) {
	for _, mode := range []fixtures.FailureMode{fixtures.ModeMalformedSSE, fixtures.ModeMidStreamError} {
		t.Run(string(mode), func(t *testing.T) {
			f := newFixture(t)
			f.backend.SetMode(mode)
			chat := f.newChat()
			mustSend(t, f, chat.ID, "question")
			f.awaitTurn(chat.ID)
			messages := f.messages(chat.ID)
			if len(messages) != 3 || messages[1].Text != fixtures.FixtureReply || messages[1].Status != StatusStopped {
				t.Fatalf("transcript = %+v, want the partial reply marked stopped", messages)
			}
			if notice := lastNotice(t, f, chat.ID); notice.Kind != NoticeCutOff {
				t.Fatalf("notice = %+v, want the cut-off notice", notice)
			}
			if len(f.settled) != 1 || f.settled[0].Ending != EndFailed {
				t.Fatalf("settled = %+v, want one failed turn", f.settled)
			}
		})
	}
}

// countRequests counts the recorded requests whose URL ends with path.
func countRequests(f *fixture, path string) int {
	n := 0
	for _, request := range f.set.HTTP.Requests() {
		if strings.HasSuffix(request.URL, path) {
			n++
		}
	}
	return n
}

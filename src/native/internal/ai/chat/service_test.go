package chat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestSendStreamsTheReplyAndSettlesComplete checks the happy path: the user's text is trimmed and kept, the reply
// streams in whole, the chat takes its title from the message, and the request names the chat's tier.
func TestSendStreamsTheReplyAndSettlesComplete(t *testing.T) {
	f := newFixture(t)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "  Hello there  "); err != nil {
		t.Fatalf("Send: %v", err)
	}
	f.awaitTurn(chat.ID)
	messages := f.messages(chat.ID)
	if len(messages) != 2 || messages[0].Text != "Hello there" || messages[1].Text != fixtures.FixtureReply {
		t.Fatalf("transcript = %+v, want the user line and the whole reply", messages)
	}
	if messages[1].Status != StatusComplete {
		t.Fatalf("reply status = %q, want complete", messages[1].Status)
	}
	if got, _ := f.svc.Chat(chat.ID); got.Title != "Hello there" {
		t.Fatalf("title = %q, want the first message", got.Title)
	}
	if len(f.noted) != 0 || len(f.settled) != 1 || f.settled[0].Ending != EndFinished {
		t.Fatalf("noted=%v settled=%+v, want one finished turn and no error", f.noted, f.settled)
	}
	if got := recordedChatRequest(t, f).Header.Get(backend.RoleHeader); got != backend.RoleFast {
		t.Fatalf("%s = %q, want the fast tier", backend.RoleHeader, got)
	}
}

// TestRequestCarriesTheSettledHistoryOnly checks what the second turn sends: the system prompt, then each earlier
// user and assistant line, and the new message last.
func TestRequestCarriesTheSettledHistoryOnly(t *testing.T) {
	f := newFixture(t)
	chat := f.newChat()
	mustSend(t, f, chat.ID, "first")
	f.awaitTurn(chat.ID)
	mustSend(t, f, chat.ID, "second")
	f.awaitTurn(chat.ID)
	if got := roles(t, recordedChatRequest(t, f).Body); strings.Join(got, ",") != "system,user,assistant,user" {
		t.Fatalf("roles = %v, want system, user, assistant, user", got)
	}
}

// TestRegenerateReplacesTheLastReply drops the reply after the last question and asks again. An empty chat has
// nothing to ask again.
func TestRegenerateReplacesTheLastReply(t *testing.T) {
	f := newFixture(t)
	chat := f.newChat()
	if err := f.svc.Regenerate(chat.ID); err != ErrNothingToAsk {
		t.Fatalf("Regenerate on an empty chat = %v, want ErrNothingToAsk", err)
	}
	mustSend(t, f, chat.ID, "again please")
	f.awaitTurn(chat.ID)
	if err := f.svc.Regenerate(chat.ID); err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
	f.awaitTurn(chat.ID)
	messages := f.messages(chat.ID)
	if len(messages) != 2 || messages[0].Author != AuthorUser || messages[1].Text != fixtures.FixtureReply {
		t.Fatalf("transcript = %+v, want one question and one reply", messages)
	}
}

// TestTierFollowsTheChat checks that a tier change reaches both the route lookup and the chat request.
func TestTierFollowsTheChat(t *testing.T) {
	f := newFixture(t)
	chat := f.newChat()
	if err := f.svc.SetTier(chat.ID, TierSmart); err != nil {
		t.Fatalf("SetTier: %v", err)
	}
	mustSend(t, f, chat.ID, "think harder")
	f.awaitTurn(chat.ID)
	if got := recordedChatRequest(t, f).Header.Get(backend.RoleHeader); got != backend.RoleSmart {
		t.Fatalf("%s = %q, want the smart tier", backend.RoleHeader, got)
	}
	if _, ok := f.svc.Model(TierSmart); !ok {
		t.Fatal("the resolved smart route was not remembered")
	}
}

// TestRefreshModelsResolvesBothTiers checks the palette can name the route of each tier.
func TestRefreshModelsResolvesBothTiers(t *testing.T) {
	f := newFixture(t)
	f.svc.RefreshModels()
	deadline := time.After(watchdog)
	for {
		f.q.drain()
		if _, ok := f.svc.Model(TierSmart); ok {
			break
		}
		select {
		case <-f.q.notify:
		case <-deadline:
			t.Fatal("the tiers were never resolved")
		}
	}
	if model, _ := f.svc.Model(TierFast); model.Name != "Fixture Route" {
		t.Fatalf("fast route = %q, want the fixture route", model.Name)
	}
}

// TestSignedOutKeepsTheMessageAndAddsASignInNotice checks that no request leaves the app without an account.
func TestSignedOutKeepsTheMessageAndAddsASignInNotice(t *testing.T) {
	f := newFixture(t)
	f.connected = false
	chat := f.newChat()
	mustSend(t, f, chat.ID, "hello")
	messages := f.messages(chat.ID)
	if len(messages) != 2 || messages[1].Author != AuthorNotice || messages[1].Notice.Kind != NoticeSignIn {
		t.Fatalf("transcript = %+v, want the message and a sign-in notice", messages)
	}
	if n := len(f.set.HTTP.Requests()); n != 0 {
		t.Fatalf("%d requests were sent without an account", n)
	}
}

// TestSendRefusesEmptyBusyAndMissingChats checks the refusals that never reach the backend.
func TestSendRefusesEmptyBusyAndMissingChats(t *testing.T) {
	f := newFixture(t)
	if err := f.svc.Send("chat-missing", "hi"); err != ErrNotFound {
		t.Fatalf("Send to a missing chat = %v, want ErrNotFound", err)
	}
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, " \n "); err != ErrEmpty {
		t.Fatalf("Send of blank text = %v, want ErrEmpty", err)
	}
	mustSend(t, f, chat.ID, "hi")
	f.awaitTurn(chat.ID)
	if err := f.svc.Delete(chat.ID); err != nil {
		t.Fatalf("Delete of an idle chat: %v", err)
	}
	if _, ok := f.svc.Chat(chat.ID); ok {
		t.Fatal("the deleted chat is still stored")
	}
}

// mustSend sends text to the chat and fails the test on an error.
func mustSend(t *testing.T, f *fixture, id, text string) {
	t.Helper()
	if err := f.svc.Send(id, text); err != nil {
		t.Fatalf("Send(%q): %v", text, err)
	}
}

// recordedChatRequest returns the most recent request to the chat route.
func recordedChatRequest(t *testing.T, f *fixture) fake.RecordedRequest {
	t.Helper()
	requests := f.set.HTTP.Requests()
	for i := len(requests) - 1; i >= 0; i-- {
		if strings.HasSuffix(requests[i].URL, fixtures.ChatCompletionsPath) {
			return requests[i]
		}
	}
	t.Fatal("no chat request was recorded")
	return fake.RecordedRequest{}
}

// roles returns the role of each message in a chat request body, in order.
func roles(t *testing.T, body []byte) []string {
	t.Helper()
	var wire struct {
		Messages []struct {
			Role string `json:"role"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("chat request is not JSON: %v", err)
	}
	out := make([]string, 0, len(wire.Messages))
	for _, message := range wire.Messages {
		out = append(out, message.Role)
	}
	return out
}

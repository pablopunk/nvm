package aichats

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func retryDetail(kind string) api.ChatDetail {
	return api.ChatDetail{ChatSummary: api.ChatSummary{ID: "c1", Kind: kind}, Messages: []api.ChatMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "answer one"},
		{Role: "user", Content: "second"},
		{Role: "assistant", Tool: &api.ChatTool{Name: "read_file", Status: "done"}},
		{Role: "system", Title: "Reply failed", Content: "The reply failed."},
	}}
}

func TestRetryRowsListEachAnswerAndFailedTurnWithNoShortcut(t *testing.T) {
	actions := retryActions(retryDetail(kindConversation), false)
	if len(actions) != 2 {
		t.Fatalf("rows = %+v, want the answer and the failed turn", actions)
	}
	for i, want := range []int{1, 4} {
		a := actions[i]
		if a.Kind != api.ActionChat || a.Op != api.ChatRetry || a.Chat.ChatID != "c1" || a.Chat.Index != want || a.Shortcut != "" {
			t.Errorf("row %d = %+v", i, a)
		}
		if err := a.Validate(); err != nil {
			t.Errorf("row %d invalid: %v", i, err)
		}
	}
	if !strings.HasPrefix(actions[0].Title, "Retry message 2: answer one") || !strings.HasPrefix(actions[1].Title, "Retry message 5: The reply failed.") {
		t.Errorf("titles = %q, %q", actions[0].Title, actions[1].Title)
	}
}

func TestRetryRowsAreAbsentForBuilderChatsAndWhileStreaming(t *testing.T) {
	if got := retryActions(retryDetail(kindBuilder), false); len(got) != 0 {
		t.Fatalf("a builder chat owns its extension, but lists %+v", got)
	}
	if got := retryActions(retryDetail(kindConversation), true); len(got) != 0 {
		t.Fatalf("a streaming chat lists %+v", got)
	}
	detail := retryDetail(kindConversation)
	detail.Messages[1].Streaming = true
	for _, a := range retryActions(detail, false) {
		if a.Chat.Index == 1 {
			t.Fatalf("a streaming answer is listed: %+v", a)
		}
	}
}

func TestRetryNeedsAChatAndAnAnswerPosition(t *testing.T) {
	bad := api.Action{Kind: api.ActionChat, Op: api.ChatRetry, Title: "Retry", Chat: &api.ChatAction{ChatID: "c1"}}
	if bad.Validate() == nil {
		t.Fatal("retry of index 0 validated")
	}
}

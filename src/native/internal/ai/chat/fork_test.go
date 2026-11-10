package chat

import (
	"errors"
	"reflect"
	"testing"
)

func TestForkReplyKeepsSourceAndStopsAtSelectedRequest(t *testing.T) {
	f := newFixture(t)
	f.connected = false
	source, err := f.svc.NewChat(KindConversation, TierSmart)
	if err != nil {
		t.Fatal(err)
	}
	err = f.svc.mutate(source.ID, func(c *Chat) {
		c.Messages = []Message{
			{Author: AuthorUser, Text: "first", Status: StatusComplete},
			{Author: AuthorAssistant, Text: "answer", Status: StatusStopped},
			{Author: AuthorUser, Text: "later", Status: StatusComplete},
			{Author: AuthorAssistant, Text: "later answer", Status: StatusComplete},
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := f.svc.Chat(source.ID)
	fork, err := f.svc.ForkReply(before, 1)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := f.svc.Chat(source.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("source changed")
	}
	if fork.ID == source.ID || fork.Kind != KindConversation || fork.Tier != TierSmart || len(fork.Messages) != 2 || fork.Messages[0].Text != "first" || fork.Messages[1].Author != AuthorNotice {
		t.Fatalf("fork = %+v", fork)
	}
}

func TestForkReplyRejectsStaleBusyAndBuilder(t *testing.T) {
	f := newFixture(t)
	source, _ := f.svc.NewChat(KindConversation, TierFast)
	_ = f.svc.mutate(source.ID, func(c *Chat) { c.Messages = []Message{{Author: AuthorUser, Text: "q"}, {Author: AuthorAssistant, Text: "a"}} })
	snapshot, _ := f.svc.Chat(source.ID)
	_ = f.svc.Retitle(source.ID, "changed")
	if _, err := f.svc.ForkReply(snapshot, 1); !errors.Is(err, ErrStaleFork) {
		t.Fatalf("stale: %v", err)
	}
	current, _ := f.svc.Chat(source.ID)
	f.svc.runs[source.ID] = &run{}
	if _, err := f.svc.ForkReply(current, 1); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy: %v", err)
	}
	delete(f.svc.runs, source.ID)
	builder, _ := f.svc.NewChat(KindBuilder, TierSmart)
	_ = f.svc.mutate(builder.ID, func(c *Chat) { c.Messages = current.Messages })
	builder, _ = f.svc.Chat(builder.ID)
	if _, err := f.svc.ForkReply(builder, 1); !errors.Is(err, ErrNothingToAsk) {
		t.Fatalf("builder: %v", err)
	}
}

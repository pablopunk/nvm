package builder

import (
	"context"
	"errors"
	"testing"
)

type publishedWithError struct{ *fakeExtensions }

func (e publishedWithError) InstallEdit(ctx context.Context, id string, files map[string][]byte, enable bool, revision any) (bool, error) {
	published, err := e.fakeExtensions.InstallEdit(ctx, id, files, enable, revision)
	if err != nil {
		return published, err
	}
	return published, errors.New("injected post-publication reload failure")
}

func TestPublishedWriteRetainsOwnershipDespiteLaterError(t *testing.T) {
	conversation := newFakeConversation(builderChat("chat-1"))
	extensions := publishedWithError{newFakeExtensions()}
	w := New(conversation, extensions, synchronous)
	if _, err := writeForTest(w, context.Background(), "chat-1", clockArgs(t)); err == nil {
		t.Fatal("post-publication error was hidden")
	}
	w.Settled("chat-1", finished())
	if conversation.bound["chat-1"] != "user.clock" || len(extensions.installs) != 1 {
		t.Fatal("published extension lost its chat ownership")
	}
}

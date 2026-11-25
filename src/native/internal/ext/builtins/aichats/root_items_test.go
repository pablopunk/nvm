package aichats

import (
	"testing"
	"time"
)

func TestRootListsNoChatsWhenTheSettingHidesThem(t *testing.T) {
	ctx := contextWith(chat("1", "conversation", "Alpha"))
	ctx.rootHidden = true
	items, err := Extension{}.RootItems(ctx)
	if err != nil || len(items) != 0 {
		t.Fatalf("RootItems = %v, %v; want no rows", items, err)
	}
	if items, _ := (Extension{}).SearchItems(ctx, "Alpha"); len(items) == 0 {
		t.Fatal("search lost the chat; the setting hides root rows only")
	}
}

func TestRootListsOnlyChatsUsedInTheLastHour(t *testing.T) {
	recent, old := chat("1", "conversation", "Recent"), chat("2", "conversation", "Old")
	old.UpdatedAtMs = testNow.Add(-rootChatWindow - time.Minute).UnixMilli()
	old.CreatedAtMs = old.UpdatedAtMs
	ctx := contextWith(recent, old)
	items, err := Extension{}.RootItems(ctx)
	if err != nil || len(items) != 1 || items[0].ID != "chat:1" {
		t.Fatalf("RootItems = %v, %v; want only the recent chat", items, err)
	}
	if found, _ := (Extension{}).SearchItems(ctx, "Old"); len(found) == 0 || found[0].ID != "chat:2" {
		t.Fatalf("search = %v; want the old chat found", found)
	}
}

// TestRootRanksBuilderChatsByRecencyLikeConversations checks that a builder chat at the empty root ties with the recent
// copies on recency alone, while a typed query still puts it after an equal conversation.
func TestRootRanksBuilderChatsByRecencyLikeConversations(t *testing.T) {
	ctx := contextWith(chat("1", "builder", "Weather"))
	root, _ := Extension{}.RootItems(ctx)
	if len(root) != 1 || root[0].TiePriority != 0 || root[0].Score != scoreRootConversation {
		t.Fatalf("root row = %+v; want the recent score and no tie priority", root)
	}
	found, _ := Extension{}.SearchItems(ctx, "Weather")
	if len(found) == 0 || found[0].TiePriority != chatTiePriority(kindBuilder) {
		t.Fatalf("search row = %+v; want the builder tie priority", found)
	}
}

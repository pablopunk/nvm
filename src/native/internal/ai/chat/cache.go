package chat

import (
	"slices"
	"time"
)

// cache is the service's copy of the chats, read on every frame by the views. It is copy-on-write: a change puts a
// new Chat in place of the one it touches, and never edits a Chat a view may hold, so a reader keeps a transcript
// that is consistent for the whole frame. Only the UI thread reads or writes the cache.
//
// drafts are the chats opened but not yet recorded: each has no message, and it joins chats with its first one.
type cache struct {
	chats   []Chat
	drafts  map[string]Chat
	version uint64
}

// load replaces the recorded chats with all. Drafts stay.
func (c *cache) load(all []Chat) {
	c.chats = all
	c.version++
}

// indexOf returns the position of the recorded chat with id, or -1.
func (c *cache) indexOf(id string) int {
	for i := range c.chats {
		if c.chats[i].ID == id {
			return i
		}
	}
	return -1
}

// get returns the chat with id, recorded or a draft. The Messages slice is shared with the cache and must not be
// modified.
func (c *cache) get(id string) (Chat, bool) {
	if index := c.indexOf(id); index >= 0 {
		return c.chats[index], true
	}
	return c.draft(id)
}

// draft returns the unrecorded chat with id.
func (c *cache) draft(id string) (Chat, bool) {
	chat, ok := c.drafts[id]
	return chat, ok
}

// put adds a recorded chat, in a new slice.
func (c *cache) put(chat Chat) {
	c.chats = append(slices.Clone(c.chats), chat)
	c.version++
}

// putDraft holds chat as an unrecorded chat, replacing any draft with its id.
func (c *cache) putDraft(chat Chat) {
	if c.drafts == nil {
		c.drafts = map[string]Chat{}
	}
	c.drafts[chat.ID] = chat
	c.version++
}

// commitDraft records the draft chat, which now has a message, in place of its draft.
func (c *cache) commitDraft(chat Chat) {
	delete(c.drafts, chat.ID)
	c.put(chat)
}

// dropDraft forgets the draft with id.
func (c *cache) dropDraft(id string) {
	delete(c.drafts, id)
	c.version++
}

// dropDraftsCreatedBefore forgets the drafts opened at or before cutoff, so an opened chat that was never used does not
// outlive the retention window.
func (c *cache) dropDraftsCreatedBefore(cutoff time.Time) {
	for id, chat := range c.drafts {
		if !chat.CreatedAt.After(cutoff) {
			delete(c.drafts, id)
		}
	}
	c.version++
}

// replace puts chat in place of the chat at index, in a new slice.
func (c *cache) replace(index int, chat Chat) {
	next := slices.Clone(c.chats)
	next[index] = chat
	c.chats = next
	c.version++
}

// drop removes the recorded chat with id, in a new slice.
func (c *cache) drop(id string) {
	c.chats = slices.DeleteFunc(slices.Clone(c.chats), func(chat Chat) bool { return chat.ID == id })
	c.version++
}

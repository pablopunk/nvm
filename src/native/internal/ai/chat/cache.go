package chat

import (
	"slices"
)

// cache is the service's copy of the chats, read on every frame by the views. It is copy-on-write: a change puts a
// new Chat in place of the one it touches, and never edits a Chat a view may hold, so a reader keeps a transcript
// that is consistent for the whole frame. Only the UI thread reads or writes the cache.
type cache struct {
	chats   []Chat
	version uint64
}

// load replaces the cache with all.
func (c *cache) load(all []Chat) {
	c.chats = all
	c.version++
}

// indexOf returns the position of the chat with id, or -1.
func (c *cache) indexOf(id string) int {
	for i := range c.chats {
		if c.chats[i].ID == id {
			return i
		}
	}
	return -1
}

// get returns the chat with id. The Messages slice is shared with the cache and must not be modified.
func (c *cache) get(id string) (Chat, bool) {
	if index := c.indexOf(id); index >= 0 {
		return c.chats[index], true
	}
	return Chat{}, false
}

// put adds a chat, in a new slice.
func (c *cache) put(chat Chat) {
	c.chats = append(slices.Clone(c.chats), chat)
	c.version++
}

// replace puts chat in place of the chat at index, in a new slice.
func (c *cache) replace(index int, chat Chat) {
	next := slices.Clone(c.chats)
	next[index] = chat
	c.chats = next
	c.version++
}

// drop removes the chat with id, in a new slice.
func (c *cache) drop(id string) {
	c.chats = slices.DeleteFunc(slices.Clone(c.chats), func(chat Chat) bool { return chat.ID == id })
	c.version++
}

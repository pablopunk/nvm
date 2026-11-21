package services

import (
	"encoding/json"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// SharedDisplays holds the display text each built-in publishes for its settings, one store for the whole host, so
// another built-in's settings row can show it.
type SharedDisplays struct {
	mu    sync.Mutex
	texts map[api.SettingRef]string
}

// NewSharedDisplays returns an empty store.
func NewSharedDisplays() *SharedDisplays {
	return &SharedDisplays{texts: map[api.SettingRef]string{}}
}

// Publish stores text as the display of setting id of owner.
func (d *SharedDisplays) Publish(owner, id, text string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.texts[api.SettingRef{Extension: owner, Setting: id}] = text
}

// Read returns the display text published for ref, and whether one was published.
func (d *SharedDisplays) Read(ref api.SettingRef) (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	text, ok := d.texts[ref]
	return text, ok
}

// publishDisplay stores the display text of a setting this built-in publishes. A setting the manifest does not list in
// Publishes is refused, so a built-in cannot speak for another's settings.
func (h *Handler) publishDisplay(args json.RawMessage) error {
	var request protocol.KeyValueArgs
	if err := decodeArgs(args, &request); err != nil {
		return err
	}
	if !h.publishes(request.Key) {
		return notFound("the extension publishes no display for setting %q", request.Key)
	}
	var text string
	if err := json.Unmarshal(request.Value, &text); err != nil {
		return invalid("the display of setting %q is not text", request.Key)
	}
	if h.deps.Displays == nil {
		return unavailable("shared setting displays are not kept here")
	}
	h.deps.Displays.Publish(h.manifest.ID, request.Key, text)
	return nil
}

func (h *Handler) publishes(id string) bool {
	for _, published := range h.manifest.Publishes {
		if published == id {
			return true
		}
	}
	return false
}

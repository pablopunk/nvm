package extview

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

func (f *formState) submitHint() *design.KeyHint {
	return &design.KeyHint{Title: f.saveTitle(), Key: palette.KeyLabel("⌃↵")}
}

// applyDraft commits a text field before the form shortcut submits. Choices and paths keep their explicit selection.
func (f *formState) applyDraft(h search.Host) {
	if f.editing == "" {
		return
	}
	field := f.fieldByID(f.editing)
	if isChoice(field.Type) || isPath(field.Type) {
		return
	}
	text := h.Query()
	if field.Type != api.FieldPassword {
		text = strings.TrimSpace(text)
	}
	f.commit(h, field, api.TextValue(text))
}

package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/search"
)

// stillDeciding is a second batch whose call has started and never answers, as the picker's meaning batch is while the
// model thinks. It is appended, as that batch is.
type stillDeciding struct{}

func (stillDeciding) ID() string                         { return "still-deciding" }
func (stillDeciding) Root(context.Context) []search.Item { return nil }
func (stillDeciding) Appended() bool                     { return true }

func (stillDeciding) Search(_ context.Context, _ string, emit func(search.Batch)) {
	emit(search.Batch{Pending: true})
}

// TestEmojiPickerShowsItsMeaningBatchLoadingAfterTheCatalogMatches is the Emoji & Symbols grid while the meaning batch
// is in flight: the catalog's matches, then placeholder cells where the model's characters will land.
func TestEmojiPickerShowsItsMeaningBatchLoadingAfterTheCatalogMatches(t *testing.T) {
	p := goldenPalette(t)
	view := p.emojiView()
	catalog := view.Providers
	view.Providers = func() []search.Provider { return append(catalog(), stillDeciding{}) }
	goldenOpen(p, view)
	goldenQuery(p, "crying")
	golden.AssertMatrix(t, "emoji-loading", p.View, paletteGoldens)
}

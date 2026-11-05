package app

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

type pasteUIQueue chan func()

func (q pasteUIQueue) Update(fn func()) { q <- fn }

func TestHiddenPaletteAllowsRepeatedBackgroundPastes(t *testing.T) {
	p := goldenPalette(t)
	p.Attach(&testWindow{})
	for i := 0; i < 3; i++ {
		if err := p.waitPaletteHidden(context.Background()); err != nil {
			t.Fatalf("paste %d with palette already hidden: %v", i+1, err)
		}
	}
}

func TestPasteChecksCurrentVisibilityAfterQueuedDismissal(t *testing.T) {
	p := goldenPalette(t)
	window := &testWindow{visible: true}
	p.Attach(window)
	queue := make(pasteUIQueue, 1)
	p.uiQueue = queue
	result := make(chan error, 1)
	go func() { result <- p.waitPaletteHidden(context.Background()) }()
	check := <-queue
	window.Hide()
	check()
	if err := <-result; err != nil {
		t.Fatalf("dismissed palette refused paste: %v", err)
	}
	go func() { result <- p.waitPaletteHidden(context.Background()) }()
	check = <-queue
	window.Show()
	check()
	if err := <-result; !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("resummoned palette allowed paste: %v", err)
	}
}

func TestPasteVisibilityWaitCanBeCancelledBeforeUIResponds(t *testing.T) {
	p := goldenPalette(t)
	queue := make(pasteUIQueue, 1)
	p.uiQueue = queue
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- p.waitPaletteHidden(ctx) }()
	check := <-queue
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled UI wait returned %v", err)
	}
	check()
}

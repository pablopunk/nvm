package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func itemsLoader(calls *int, titles ...string) func(context.Context) ([]api.Item, error) {
	return func(context.Context) ([]api.Item, error) {
		*calls++
		var items []api.Item
		for _, title := range titles {
			items = append(items, api.Item{ID: title, Title: title})
		}
		return items, nil
	}
}

func failingLoader(calls *int) func(context.Context) ([]api.Item, error) {
	return func(context.Context) ([]api.Item, error) {
		*calls++
		return nil, errors.New("upstream is down")
	}
}

func TestSourceWithoutCacheKeyAlwaysLoads(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.one"))
	calls := 0
	for i := 0; i < 2; i++ {
		if _, err := h.LoadSource(context.Background(), api.DataSource{Loader: "feed"}, itemsLoader(&calls, "a")); err != nil {
			t.Fatalf("load: %v", err)
		}
	}
	if calls != 2 {
		t.Fatalf("loader ran %d times; want 2", calls)
	}
}

func TestSavedItemsAreServedWithinTTLAndReloadedAfter(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, manifestFor("acme.one"))
	source := api.DataSource{Loader: "feed", CacheKey: "feed", TTLMs: 60_000, StaleTTLMs: 300_000}
	calls := 0
	first, _ := h.LoadSource(context.Background(), source, itemsLoader(&calls, "fresh"))
	again, _ := h.LoadSource(context.Background(), source, itemsLoader(&calls, "newer"))
	if calls != 1 || again[0].Title != "fresh" || first[0].Title != "fresh" {
		t.Fatalf("within TTL: calls %d, got %v; want one load and the saved items", calls, again)
	}
	r.clock.Advance(90 * time.Second)
	after, _ := h.LoadSource(context.Background(), source, itemsLoader(&calls, "newer"))
	if calls != 2 || after[0].Title != "newer" {
		t.Fatalf("after TTL: calls %d, got %v; want a reload", calls, after)
	}
}

func TestStaleItemsCoverAFailingLoaderWithinTheStaleWindow(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, manifestFor("acme.one"))
	source := api.DataSource{Loader: "feed", CacheKey: "feed", TTLMs: 60_000, StaleTTLMs: 300_000}
	calls := 0
	if _, err := h.LoadSource(context.Background(), source, itemsLoader(&calls, "kept")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r.clock.Advance(2 * time.Minute)
	items, err := h.LoadSource(context.Background(), source, failingLoader(&calls))
	if err != nil || len(items) != 1 || items[0].Title != "kept" {
		t.Fatalf("stale fallback = %v (%v); want the saved items", items, err)
	}
	r.clock.Advance(10 * time.Minute)
	if _, err := h.LoadSource(context.Background(), source, failingLoader(&calls)); err == nil {
		t.Fatal("items past the stale window were served")
	}
}

func TestDataCacheIsBounded(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, manifestFor("acme.one"))
	calls := 0
	for i := 0; i < maxDataCacheEntries+5; i++ {
		source := api.DataSource{Loader: "feed", CacheKey: fmt.Sprintf("feed-%d", i)}
		r.clock.Advance(time.Second)
		if _, err := h.LoadSource(context.Background(), source, itemsLoader(&calls, "x")); err != nil {
			t.Fatalf("load %d: %v", i, err)
		}
	}
	doc, err := h.openedDocumentsForTest()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cache, _ := doc.data.Load()
	if len(cache.Entries) != maxDataCacheEntries {
		t.Fatalf("cache holds %d entries; want %d", len(cache.Entries), maxDataCacheEntries)
	}
	if _, kept := cache.Entries["feed-0"]; kept {
		t.Fatal("the oldest entry was not evicted")
	}
}

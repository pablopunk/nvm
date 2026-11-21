package manager

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestCommandSnapshotRefreshIsBoundedAndInvalidationRetainsRows(t *testing.T) {
	var cache commandCache
	now := time.Unix(100, 0)
	generation, ok := cache.begin("timer", now)
	if !ok {
		t.Fatal("cold discovery was not started")
	}
	result := CommandResult{ExtensionID: "timer", Commands: []api.Command{{ID: "start", Title: "Start Timer"}}}
	if !cache.finish("timer", generation, result, now) {
		t.Fatal("first snapshot did not report a change")
	}
	if _, ok := cache.begin("timer", now); ok {
		t.Fatal("the change notification immediately started another refresh")
	}
	cache.invalidate("timer")
	if got, ok := cache.lookup("timer"); !ok || got.Commands[0].ID != "start" {
		t.Fatal("invalidation discarded the useful snapshot")
	}
	fresh, ok := cache.begin("timer", now)
	if !ok || fresh == generation {
		t.Fatal("explicit invalidation did not immediately start a new generation")
	}
	if cache.finish("timer", generation, CommandResult{ExtensionID: "timer"}, now) {
		t.Fatal("an invalidated call replaced the newer snapshot")
	}
	if cache.finish("timer", fresh, result, now) {
		t.Fatal("unchanged commands requested another redraw")
	}
	if _, ok := cache.begin("timer", now.Add(commandRetryInterval)); !ok {
		t.Fatal("successful dynamic commands were cached forever")
	}
}

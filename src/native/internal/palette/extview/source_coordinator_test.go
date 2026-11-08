package extview

import (
	"fmt"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestCoordinatorEvictionPreservesLiveEpochAndRejectsStaleWrites(t *testing.T) {
	c := newSourceCoordinator(1, 1024)
	id := new(int)
	old := c.acquire("ext", id)
	epoch := old.version()
	if !old.put("source", epoch, CacheEntry{Items: []api.Item{{ID: "old"}}}) {
		t.Fatal("initial write refused")
	}
	other := c.acquire("other", new(int))
	other.put("source", other.version(), CacheEntry{Items: []api.Item{{ID: "other"}}})
	if old.version() != epoch {
		t.Fatal("snapshot eviction discarded live epoch")
	}
	c.invalidate("ext", id)
	if old.put("source", epoch, CacheEntry{}) {
		t.Fatal("accepted pre-invalidation write")
	}
	if _, _, ok := other.get("source"); !ok {
		t.Fatal("invalidation touched unrelated installation")
	}
	old.release()
	other.release()
}

func TestCoordinatorReplacementsAndReleasedLeaseAreFenced(t *testing.T) {
	c := newSourceCoordinator(4, 1024)
	first, second := new(int), new(int)
	old := c.acquire("ext", first)
	epoch := old.version()
	replacement := c.acquire("ext", second)
	replacement.put("source", replacement.version(), CacheEntry{Items: []api.Item{{ID: "new"}}})
	c.invalidate("ext", first)
	if _, _, ok := replacement.get("source"); !ok {
		t.Fatal("old invalidation removed replacement")
	}
	old.release()
	if old.put("source", epoch, CacheEntry{}) {
		t.Fatal("released lease wrote")
	}
	replacement.release()
}

func TestCoordinatorBoundsSnapshotsAcrossLifecycles(t *testing.T) {
	c := newSourceCoordinator(3, 160)
	for i := 0; i < 50; i++ {
		lease := c.acquire("ext", new(int))
		lease.put(fmt.Sprint(i), lease.version(), CacheEntry{Items: []api.Item{{ID: "123456789"}}})
		lease.release()
		if c.entries > c.maxEntries || c.bytes > c.maxBytes || len(c.records) > c.maxEntries {
			t.Fatalf("entries=%d bytes=%d records=%d", c.entries, c.bytes, len(c.records))
		}
	}
}

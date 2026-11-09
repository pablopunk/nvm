package extview

import "testing"

// TestInvalidateDropsOnlyThatExtensionsSnapshots checks that Views.Invalidate clears the extension's cached
// data-source rows, so its next open loads fresh ones, and leaves every other extension's rows, including those of an
// extension whose id starts with the same text.
func TestInvalidateDropsOnlyThatExtensionsSnapshots(t *testing.T) {
	cache := NewMemoryCache()
	for _, key := range []string{"local.downloads/big-files", "local.downloads/other", "local.downloads-big-files/big-files", "acme.good/list"} {
		cache.Put(key, CacheEntry{})
	}
	cache.Invalidate("local.downloads", nil)
	for key, want := range map[string]bool{"local.downloads/big-files": false, "local.downloads/other": false, "local.downloads-big-files/big-files": true, "acme.good/list": true} {
		if _, ok := cache.Get(key); ok != want {
			t.Errorf("snapshot %q present = %v, want %v", key, ok, want)
		}
	}
}

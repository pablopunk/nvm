package thumbs

import (
	"container/list"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// CacheBytes bounds the thumbnails kept on disk. Past it the least recently used files are removed.
const CacheBytes = 600 << 20

// Cache keeps thumbnails under one folder of the app's cache directory, one PNG file per source file version. The key
// is a hash of the path, the modification time and the size, so an edited picture gets a new thumbnail and the old one
// ages out. Writes are atomic: a temporary file is flushed and renamed into place. The cache is safe for concurrent use.
type Cache struct {
	fsys  platform.FS
	dir   string
	limit int64

	mu      sync.Mutex
	entries map[string]*list.Element
	lru     *list.List // front: most recently used
	total   int64
}

// cached is one stored thumbnail.
type cached struct {
	key  string
	size int64
}

// Open returns the cache in dir, creating the folder if needed. Files already there count toward the bound, oldest first.
// limit is the byte bound; zero means CacheBytes.
func Open(fsys platform.FS, dir string, limit int64) (*Cache, error) {
	if limit <= 0 {
		limit = CacheBytes
	}
	if err := fsys.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	c := &Cache{fsys: fsys, dir: dir, limit: limit, entries: map[string]*list.Element{}, lru: list.New()}
	if err := c.load(); err != nil {
		return nil, err
	}
	return c, nil
}

// load adopts the thumbnails already in the folder: the newest are the most recently used.
func (c *Cache) load() error {
	des, err := c.fsys.ReadDir(c.dir)
	if err != nil {
		return err
	}
	type found struct {
		key     string
		size    int64
		modTime int64
	}
	var all []found
	for _, de := range des {
		info, err := de.Info()
		if err != nil || !info.Mode().IsRegular() || filepath.Ext(de.Name()) != ".png" {
			continue
		}
		all = append(all, found{key: de.Name()[:len(de.Name())-len(".png")], size: info.Size(), modTime: info.ModTime().UnixNano()})
	}
	slices.SortFunc(all, func(a, b found) int {
		switch {
		case a.modTime < b.modTime:
			return -1
		case a.modTime > b.modTime:
			return 1
		default:
			return 0
		}
	})
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range all {
		c.pushFrontLocked(f.key, f.size)
	}
	c.evictLocked()
	return nil
}

// Thumbnail returns the PNG thumbnail of the image at path, from disk when it is there and rendered otherwise. mtimeMs and
// size identify the version of the file the thumbnail is for.
func (c *Cache) Thumbnail(path string, mtimeMs, size int64) ([]byte, error) {
	return c.Rendered(path, mtimeMs, size, Edge)
}

// Rendered returns the PNG of the image at path scaled to edge, from disk when it is there and rendered otherwise, so a
// picture is only ever scaled once per version of the file.
func (c *Cache) Rendered(path string, mtimeMs, size int64, edge int) ([]byte, error) {
	key := keyFor(path, mtimeMs, size)
	if edge != Edge {
		key = keyFor(path+"\x00"+strconv.Itoa(edge), mtimeMs, size)
	}
	if data, ok := c.read(key); ok {
		return data, nil
	}
	data, err := Render(c.fsys, path, edge)
	if err != nil {
		return nil, err
	}
	if err := c.write(key, data); err != nil {
		return data, nil
	}
	return data, nil
}

// Has reports whether the thumbnail for a file version is stored, without reading it or changing its recency.
func (c *Cache) Has(path string, mtimeMs, size int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.entries[keyFor(path, mtimeMs, size)]
	return ok
}

// Total returns the bytes of the thumbnails kept.
func (c *Cache) Total() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}

// read returns the stored bytes for key and marks it used. A stored file that cannot be read is forgotten.
func (c *Cache) read(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	data, err := c.fsys.ReadFile(c.pathOf(key))
	if err != nil {
		c.total -= el.Value.(*cached).size
		c.lru.Remove(el)
		delete(c.entries, key)
		return nil, false
	}
	c.lru.MoveToFront(el)
	return data, true
}

// write stores data under key atomically and then enforces the bound.
func (c *Cache) write(key string, data []byte) error {
	target := c.pathOf(key)
	var tmp string
	for range 10 {
		var suffix [16]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return err
		}
		tmp = target + "." + hex.EncodeToString(suffix[:]) + ".tmp"
		err := c.fsys.CreateExclusive(tmp, data, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		defer c.fsys.Remove(tmp)
		if err := c.fsys.Sync(tmp); err != nil {
			return err
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if err := c.fsys.Rename(tmp, target); err != nil {
			return err
		}
		if el, ok := c.entries[key]; ok {
			c.total -= el.Value.(*cached).size
			c.lru.Remove(el)
			delete(c.entries, key)
		}
		c.pushFrontLocked(key, int64(len(data)))
		c.evictLocked()
		return nil
	}
	return fs.ErrExist
}

// pushFrontLocked records key as the most recently used entry. The caller holds mu.
func (c *Cache) pushFrontLocked(key string, size int64) {
	if el, ok := c.entries[key]; ok {
		c.total -= el.Value.(*cached).size
		c.lru.Remove(el)
	}
	c.entries[key] = c.lru.PushFront(&cached{key: key, size: size})
	c.total += size
}

// evictLocked removes the least recently used thumbnails until the total fits the bound. The caller holds mu; removing
// the files happens under the lock too, which keeps the index and the folder in step.
func (c *Cache) evictLocked() {
	for c.total > c.limit {
		el := c.lru.Back()
		if el == nil {
			return
		}
		victim := el.Value.(*cached)
		c.lru.Remove(el)
		delete(c.entries, victim.key)
		c.total -= victim.size
		_ = c.fsys.Remove(c.pathOf(victim.key))
	}
}

// pathOf returns the file that stores key.
func (c *Cache) pathOf(key string) string {
	return filepath.Join(c.dir, key+".png")
}

// keyFor hashes the identity of one version of a file.
func keyFor(path string, mtimeMs, size int64) string {
	sum := sha256.Sum256([]byte(path + "\x00" + strconv.FormatInt(mtimeMs, 10) + "\x00" + strconv.FormatInt(size, 10)))
	return hex.EncodeToString(sum[:])
}

package fake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Downloads is a platform.FileDownloader that serves scripted content by URL and writes it to the real disk, since
// the files it fetches are read by paths. Block, when set, holds every download until it is closed.
type Downloads struct {
	mu      sync.Mutex
	content map[string][]byte
	fetched []string
	Block   chan struct{}
}

var _ platform.FileDownloader = (*Downloads)(nil)

// NewDownloads returns a downloader that serves nothing yet.
func NewDownloads() *Downloads {
	return &Downloads{content: map[string][]byte{}}
}

// Serve makes url answer content.
func (d *Downloads) Serve(url string, content []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.content[url] = content
}

// Fetched lists the URLs downloaded, in order.
func (d *Downloads) Fetched() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.fetched...)
}

// Download writes the served content to dest after checking its checksum.
func (d *Downloads) Download(ctx context.Context, url, dest, sum string, progress func(int64)) error {
	if d.Block != nil {
		select {
		case <-d.Block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	d.mu.Lock()
	content, ok := d.content[url]
	d.fetched = append(d.fetched, url)
	d.mu.Unlock()
	if !ok {
		return fmt.Errorf("download %s: HTTP 404", url)
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != sum {
		return platform.ErrChecksum
	}
	if progress != nil {
		progress(int64(len(content)))
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, content, 0o644)
}

package real

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// partSuffix names the file a download writes into until its checksum is confirmed.
const partSuffix = ".part"

// fileDownloader streams downloads to disk with the standard HTTP client, which follows redirects.
type fileDownloader struct {
	client *http.Client
}

func newFileDownloader() platform.FileDownloader {
	return fileDownloader{client: &http.Client{}}
}

// Download resumes a partial file with a range request when the server allows it, and starts over otherwise.
func (d fileDownloader) Download(ctx context.Context, url, dest, sum string, progress func(int64)) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	part := dest + partSuffix
	hasher := sha256.New()
	done, err := hashExisting(part, hasher)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if done > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", done))
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	switch {
	case resp.StatusCode == http.StatusPartialContent && done > 0:
	case resp.StatusCode == http.StatusOK:
		flags |= os.O_TRUNC
		done = 0
		hasher.Reset()
	default:
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	file, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := copyWithProgress(io.MultiWriter(file, hasher), resp.Body, done, progress); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if hex.EncodeToString(hasher.Sum(nil)) != sum {
		_ = os.Remove(part)
		return fmt.Errorf("%w: %s", platform.ErrChecksum, filepath.Base(dest))
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(part, dest)
}

// hashExisting feeds a partial download into hasher and returns its size; a missing file is empty.
func hashExisting(part string, hasher hash.Hash) (int64, error) {
	file, err := os.Open(part)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer file.Close()
	return io.Copy(hasher, file)
}

func copyWithProgress(dst io.Writer, src io.Reader, done int64, progress func(int64)) error {
	buf := make([]byte, 256<<10)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(done)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

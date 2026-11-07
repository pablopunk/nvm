package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// A data URL longer than this is not decoded.
const maxImageDataURLBytes = 4 << 20

var extensionImagePermits = make(chan struct{}, 2)

// loadExtensionBitmap decodes an extension image from a data URL, local file, or public web URL.
// It runs off the UI thread. A nil result means the image is not shown.
func loadExtensionBitmap(files platform.FS, ref string, publicHTTP ...platform.HTTP) *ui.Bitmap {
	return loadExtensionBitmapContext(context.Background(), files, ref, publicHTTP...)
}

func loadExtensionBitmapContext(ctx context.Context, files platform.FS, ref string, publicHTTP ...platform.HTTP) *ui.Bitmap {
	select {
	case extensionImagePermits <- struct{}{}:
	case <-ctx.Done():
		return nil
	}
	defer func() { <-extensionImagePermits }()
	var data []byte
	var ok bool
	parsed, parseErr := url.Parse(ref)
	if parseErr == nil && (strings.EqualFold(parsed.Scheme, "https") || strings.EqualFold(parsed.Scheme, "http")) {
		if len(publicHTTP) > 0 && publicHTTP[0] != nil {
			data, ok = remoteImageBytesContext(ctx, publicHTTP[0], ref)
		}
	} else {
		data, ok = imageBytes(files, ref)
	}
	if !ok {
		return nil
	}
	bitmap, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	return bitmap
}

func imageBytes(files platform.FS, ref string) ([]byte, bool) {
	if rest, found := strings.CutPrefix(ref, "data:"); found {
		_, payload, hasPayload := strings.Cut(rest, "base64,")
		if !hasPayload || len(payload) > maxImageDataURLBytes {
			return nil, false
		}
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, false
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > thumbs.MaxSourcePixels/config.Height {
			return nil, false
		}
		return data, true
	}
	if files == nil {
		return nil, false
	}
	data, err := thumbs.Render(files, localImagePath(ref), thumbs.ReferenceEdge(ref))
	if err != nil {
		return nil, false
	}
	return data, true
}

// localImagePath returns the path of a local image ref: a file: URL, as the files service names a file, or a plain path.
func localImagePath(ref string) string {
	parsed, err := url.Parse(ref)
	if err != nil || parsed.Scheme != "file" {
		return ref
	}
	return platform.FileURLPath(parsed)
}

// remoteImageBytes uses the public-only transport and bounds both network and decoder work.
func remoteImageBytes(client platform.HTTP, ref string) ([]byte, bool) {
	return remoteImageBytesContext(context.Background(), client, ref)
}

func remoteImageBytesContext(parent context.Context, client platform.HTTP, ref string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	result, err := platform.FetchExchange(ctx, client, platform.Exchange{Method: http.MethodGet, URL: ref, MaxBytes: maxImageDataURLBytes, MaxRedirects: 3, Check: func(target *url.URL) error {
		if (!strings.EqualFold(target.Scheme, "http") && !strings.EqualFold(target.Scheme, "https")) || target.Host == "" || target.User != nil {
			return errors.New("invalid image URL")
		}
		target.Scheme = strings.ToLower(target.Scheme)
		return nil
	}})
	if err != nil || result.Status != http.StatusOK {
		return nil, false
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(result.Body))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > thumbs.MaxSourcePixels/config.Height {
		return nil, false
	}
	return result.Body, true
}

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io/fs"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	_ "image/gif"  // registers the GIF decoder with image.DecodeConfig
	_ "image/jpeg" // registers the JPEG decoder with image.DecodeConfig
	_ "image/png"  // registers the PNG decoder with image.DecodeConfig

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
)

// dimensionPrefixBytes bounds how much of an image is read to learn its size.
const dimensionPrefixBytes = 256 << 10

// findFiles walks the requested folders and returns the best files the options select. Missing and relative folders
// are skipped, as the index skips them.
func (h *Handler) findFiles(ctx context.Context, raw json.RawMessage) ([]api.FileRef, error) {
	var args protocol.FindArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	if h.deps.Files == nil {
		return nil, unavailable("files are not available in this build")
	}
	query, err := findQuery(args.Options)
	if err != nil {
		return nil, err
	}
	found, err := files.Find(ctx, h.deps.Files, args.Roots, files.FindOptions{
		Query:     query,
		Home:      h.deps.Home,
		Depth:     args.Options.Depth,
		ScanLimit: args.Options.ScanLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("find files: %w", err)
	}
	refs := make([]api.FileRef, 0, len(found.Results))
	for _, result := range found.Results {
		ref := fileRefOf(result.Path, h.deps.Home, result.Size, result.Modified, result.Created)
		if args.Options.IncludeDimensions && ref.Kind == files.KindImage.String() {
			ref.Width, ref.Height = h.imageDimensions(result.Path)
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// findQuery turns the extension's find options into a file query. Unknown kinds and sort keys are invalid input, so
// a typo does not silently search everything.
func findQuery(options api.FindOptions) (files.Query, error) {
	kinds, err := findKinds(options.Kind)
	if err != nil {
		return files.Query{}, err
	}
	sortBy, err := findSortKey(options.SortBy)
	if err != nil {
		return files.Query{}, err
	}
	return files.Query{
		Kinds:      kinds,
		Extensions: options.Extensions,
		Patterns:   options.Patterns,
		Limit:      options.Limit,
		SortBy:     sortBy,
		Order:      findOrder(options.Order),
	}, nil
}

func findKinds(kind string) ([]files.Kind, error) {
	switch kind {
	case "":
		return nil, nil
	case "image":
		return []files.Kind{files.KindImage}, nil
	case "video":
		return []files.Kind{files.KindVideo}, nil
	case "media":
		return []files.Kind{files.KindMedia}, nil
	case "file":
		return []files.Kind{files.KindFile}, nil
	}
	return nil, invalid("find kind %q is not image, video, media or file", kind)
}

func findSortKey(sortBy string) (files.SortBy, error) {
	switch key := files.SortBy(sortBy); key {
	case files.SortNone, files.SortAdded, files.SortCreated, files.SortRecent, files.SortModified, files.SortName, files.SortSize:
		return key, nil
	}
	return files.SortNone, invalid("find sort key %q is not recent, modified, added, created, name or size", sortBy)
}

// findOrder reads the direction of a sort. Descending is the default, as the palette wants for recency.
func findOrder(order string) files.Order {
	if order == "asc" {
		return files.OrderAsc
	}
	return files.OrderDesc
}

// fileMetadata describes one file, with its image dimensions when it is a picture.
func (h *Handler) fileMetadata(raw json.RawMessage) (api.FileRef, error) {
	path, err := absolutePathArg(raw)
	if err != nil {
		return api.FileRef{}, err
	}
	if h.deps.Files == nil {
		return api.FileRef{}, unavailable("files are not available in this build")
	}
	info, err := h.deps.Files.Stat(path)
	if err != nil {
		return api.FileRef{}, notFound("cannot read %s", files.DisplayPath(h.deps.Home, path))
	}
	ref := fileRefOf(path, h.deps.Home, info.Size(), info.ModTime(), time.Time{})
	if ref.Kind == files.KindImage.String() {
		ref.Width, ref.Height = h.imageDimensions(path)
	}
	return ref, nil
}

// fileThumbnail returns a host-owned reference to the picture of a file, or Found false when it has none. It never
// reads the file: a grid asks for one per row before it can show any row, and reading a file can take seconds. The palette
// decodes the picture when its tile is drawn, so pictures fill in one by one, and a file with no picture stays a
// placeholder. The picture is also made ahead of time, so a grid opened later finds it ready.
func (h *Handler) fileThumbnail(raw json.RawMessage) (protocol.ThumbnailResult, error) {
	path, err := absolutePathArg(raw)
	if err != nil {
		return protocol.ThumbnailResult{}, err
	}
	if h.deps.Files == nil {
		return protocol.ThumbnailResult{}, unavailable("files are not available in this build")
	}
	info, err := h.deps.Files.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return protocol.ThumbnailResult{}, snapshotPermissionDenied{}
		}
		return protocol.ThumbnailResult{}, nil
	}
	if !info.Mode().IsRegular() || !h.drawsPictureOf(path, info.Size()) {
		return protocol.ThumbnailResult{}, nil
	}
	if h.deps.PrepareFilePicture != nil {
		h.deps.PrepareFilePicture(path)
	}
	return protocol.ThumbnailResult{URL: thumbs.GridReference(fileURLFor(path)), Found: true}, nil
}

// drawsPictureOf reports whether a grid can draw this file: any file where the system draws pictures, and otherwise an
// image the decoder accepts.
func (h *Handler) drawsPictureOf(path string, size int64) bool {
	if h.deps.FilePictures != nil {
		return true
	}
	return size <= thumbs.MaxSourceBytes && files.KindOf(filepath.Base(path)) == files.KindImage
}

// fileURLOf returns the file: URL of an absolute path.
func (h *Handler) fileURLOf(raw json.RawMessage) (string, error) {
	path, err := absolutePathArg(raw)
	if err != nil {
		return "", err
	}
	return fileURLFor(path), nil
}

// fileURLFor spells a local path as a file: URL. A URL path starts with a slash, so a Windows drive path gets one.
func fileURLFor(path string) string {
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return (&url.URL{Scheme: "file", Path: slashed}).String()
}

// fileRefOf describes a file from its path and what the stat or scan knows of it.
func fileRefOf(path, home string, size int64, modified, created time.Time) api.FileRef {
	name := filepath.Base(path)
	kind := files.KindOf(name).String()
	ref := api.FileRef{
		Path:        path,
		Name:        name,
		DisplayPath: files.DisplayPath(home, path),
		FileURL:     fileURLFor(path),
		Kind:        kind,
		Extension:   files.ExtensionOf(name),
		MimeType:    files.MIMEType(name),
		Size:        size,
		MtimeMs:     millisOf(modified),
		BirthtimeMs: millisOf(created),
	}
	ref.URL = ref.FileURL
	if kind == files.KindVideo.String() {
		ref.VideoURL = ref.FileURL
	}
	return ref
}

func millisOf(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// imageDimensions reads the width and height of a picture from its header. An unreadable picture has zero dimensions.
func (h *Handler) imageDimensions(path string) (int, int) {
	data, _, err := h.deps.Files.ReadFilePrefix(path, dimensionPrefixBytes)
	if err != nil {
		return 0, 0
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return config.Width, config.Height
}

package files

import (
	"path/filepath"
	"strings"
)

// imageExtensions and videoExtensions are the extensions that give a file the image or video kind, as
// file-utils.ts lists them.
var (
	imageExtensions = setOf("png", "jpg", "jpeg", "gif", "webp", "bmp", "tiff", "tif", "heic")
	videoExtensions = setOf("mp4", "mov", "avi", "mkv", "webm", "wmv", "flv", "m4v")
)

// setOf builds a set from its arguments.
func setOf(values ...string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

// ExtensionOf returns the lowercased extension of a file name without its dot, following Node's path.extname:
// a name whose only dot leads it ("notes", ".bashrc") has no extension.
func ExtensionOf(name string) string {
	base := filepath.Base(name)
	dot := strings.LastIndexByte(base, '.')
	if dot <= 0 {
		return ""
	}
	return strings.ToLower(base[dot+1:])
}

// KindOf classifies a file by its name alone: image and video by extension, file otherwise. It never reads the file, so
// a palette can label a row before any preview loads.
func KindOf(name string) Kind {
	return kindOfExtension(ExtensionOf(name))
}

// kindOfExtension classifies an already-lowercased extension.
func kindOfExtension(ext string) Kind {
	switch {
	case imageExtensions[ext]:
		return KindImage
	case videoExtensions[ext]:
		return KindVideo
	default:
		return KindFile
	}
}

// extensionsForKinds returns the extensions a kind filter selects. "file" selects nothing, so a filter that
// lists only "file" does not narrow the results, as extensionsForFindOptions does.
func extensionsForKinds(kinds []Kind) []string {
	var exts []string
	for _, kind := range kinds {
		switch kind {
		case KindImage:
			exts = appendSet(exts, imageExtensions)
		case KindVideo:
			exts = appendSet(exts, videoExtensions)
		case KindMedia:
			exts = appendSet(exts, imageExtensions)
			exts = appendSet(exts, videoExtensions)
		}
	}
	return exts
}

// appendSet appends the members of set in no particular order.
func appendSet(dst []string, set map[string]bool) []string {
	for ext := range set {
		dst = append(dst, ext)
	}
	return dst
}

package extai

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
)

var dataURLPattern = regexp.MustCompile(`(?s)^data:([^;,]+);base64,(.*)$`)

// decisionImageTypes are the media types a decision reads.
var decisionImageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}

// imageExtensions maps the image file extensions a prompt reads to their media types.
var imageExtensions = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".gif": "image/gif"}

// textExtensions are the file types whose content a prompt reads as text.
var textExtensions = map[string]bool{".md": true, ".markdown": true, ".txt": true, ".json": true, ".csv": true, ".tsv": true,
	".xml": true, ".html": true, ".css": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true, ".yml": true, ".yaml": true, ".log": true}

// dataURL is the data URL of an image, the form the chat route reads.
func dataURL(image backend.DecisionImage) string {
	return "data:" + image.MediaType + ";base64," + image.Data
}

// parseDataURL reads a base64 data URL into an image.
func parseDataURL(source string) (backend.DecisionImage, error) {
	match := dataURLPattern.FindStringSubmatch(source)
	if match == nil {
		return backend.DecisionImage{}, InputError("AI image attachment must be a base64 data URL")
	}
	return backend.DecisionImage{MediaType: match[1], Data: match[2]}, nil
}

// imageFrom reads an image given as a data URL or a file path.
func (s *Service) imageFrom(source string, decision bool, index int) (backend.DecisionImage, error) {
	if !strings.HasPrefix(source, "data:") {
		return s.imageFile(source, decision, index)
	}
	image, err := parseDataURL(source)
	if err != nil {
		return image, err
	}
	if decision {
		return image, checkDecisionImage(image, index)
	}
	return image, nil
}

// imageFile reads an image file. A decision checks its type and size before it reads the file.
func (s *Service) imageFile(source string, decision bool, index int) (backend.DecisionImage, error) {
	path, err := s.absolutePath(source)
	if err != nil {
		return backend.DecisionImage{}, err
	}
	info, err := s.deps.Files.Stat(path)
	if err != nil {
		if decision {
			return backend.DecisionImage{}, inputErrorf("Attachment %s cannot be read as text or an image, so it was not sent.", baseName(path))
		}
		return backend.DecisionImage{}, inputErrorf("AI image attachment cannot be read: %s", path)
	}
	if !decision && info.Size() > askImageBytes {
		return backend.DecisionImage{}, inputErrorf("AI image attachment is too large: %s", path)
	}
	if decision && info.Size() > decisionImageBytes {
		return backend.DecisionImage{}, inputErrorf("Decision image %d is larger than 2 MiB. Resize it before deciding.", index+1)
	}
	data, err := s.deps.Files.ReadFile(path)
	if err != nil {
		return backend.DecisionImage{}, err
	}
	mime, known := mimeOf(path)
	if !known {
		mime = "image/png"
		if decision {
			mime = "application/octet-stream"
		}
	}
	image := backend.DecisionImage{MediaType: mime, Data: base64.StdEncoding.EncodeToString(data)}
	if decision {
		return image, checkDecisionImage(image, index)
	}
	return image, nil
}

// checkDecisionImage refuses an image a decision cannot read: another media type or more than 2 MiB decoded.
func checkDecisionImage(image backend.DecisionImage, index int) error {
	if !decisionImageTypes[image.MediaType] {
		return inputErrorf("Decision image %d is %s; use PNG, JPEG or WebP.", index+1, image.MediaType)
	}
	if decodedBase64Length(image.Data) > decisionImageBytes {
		return inputErrorf("Decision image %d is larger than 2 MiB. Resize it before deciding.", index+1)
	}
	return nil
}

// decodedBase64Length is the byte length of the data once decoded.
func decodedBase64Length(data string) int {
	padding := 0
	switch {
	case strings.HasSuffix(data, "=="):
		padding = 2
	case strings.HasSuffix(data, "="):
		padding = 1
	}
	return len(data)*3/4 - padding
}

// absolutePath expands a leading "~" and requires the result to be absolute, so a path never depends on the working
// folder of the host.
func (s *Service) absolutePath(source string) (string, error) {
	path := source
	if s.deps.Home != "" && (path == "~" || strings.HasPrefix(path, "~/")) {
		path = s.deps.Home + path[1:]
	}
	if !filepath.IsAbs(path) {
		return "", inputErrorf("AI attachment paths must be absolute: %s", source)
	}
	return filepath.Clean(path), nil
}

// ocrSource recognises the text of an image given as a data URL (PNG only) or a file path.
func (s *Service) ocrSource(ctx context.Context, source string) (string, error) {
	if s.deps.OCR == nil {
		return "", InputError("OCR is not available on this system")
	}
	if strings.HasPrefix(source, "data:") {
		return s.ocrDataURL(ctx, source)
	}
	path, err := s.absolutePath(source)
	if err != nil {
		return "", err
	}
	result, err := s.deps.OCR.Image(ctx, path, ocr.Options{})
	return result.Text, err
}

func (s *Service) ocrDataURL(ctx context.Context, source string) (string, error) {
	image, err := parseDataURL(source)
	if err != nil {
		return "", err
	}
	if image.MediaType != "image/png" {
		return "", InputError("OCR needs a PNG image or a file path")
	}
	png, err := base64.StdEncoding.DecodeString(image.Data)
	if err != nil {
		return "", InputError("AI image attachment must be a base64 data URL")
	}
	result, err := s.deps.OCR.ImagePNG(ctx, png, ocr.Options{})
	return result.Text, err
}

// mimeOf returns the media type of an image or text file by its extension, and whether it knows one.
func mimeOf(path string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	if mime, ok := imageExtensions[ext]; ok {
		return mime, true
	}
	if textExtensions[ext] {
		return "text/plain", true
	}
	return "", false
}

func isImagePath(path string) bool {
	_, ok := imageExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

func isTextLikePath(path string) bool {
	return textExtensions[strings.ToLower(filepath.Ext(path))]
}

func baseName(path string) string {
	return filepath.Base(path)
}

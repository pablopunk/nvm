package commands

import (
	"encoding/base64"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// MaximumImageBytes is the largest decoded image the decision route accepts.
const MaximumImageBytes = 2 << 20

// classifyInput is the text the image classification sends with the picture.
const classifyInput = "Classify this image."

// ClassifyQuestions asks the category of the image in one decision call.
var ClassifyQuestions = []backend.DecisionQuestion{
	{ID: "category", Type: "choice", Question: "Which category best describes this image?", Options: []string{"photograph", "screenshot", "document", "illustration", "other"}},
}

// imageOf encodes data as a picture of mediaType for the decision route.
func imageOf(mediaType string, data []byte) backend.DecisionImage {
	return backend.DecisionImage{MediaType: mediaType, Data: base64.StdEncoding.EncodeToString(data)}
}

// mediaTypeOf is the media type of a picture file, named by its extension, or "" when the decision route does not take
// the file.
func mediaTypeOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	}
	return ""
}

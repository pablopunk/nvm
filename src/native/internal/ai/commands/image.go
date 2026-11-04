package commands

import (
	"encoding/base64"

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

// pngImage encodes a PNG for the decision route.
func pngImage(png []byte) backend.DecisionImage {
	return backend.DecisionImage{MediaType: "image/png", Data: base64.StdEncoding.EncodeToString(png)}
}

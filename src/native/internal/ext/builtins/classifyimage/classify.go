package classifyimage

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// MaximumImageBytes is the largest picture the decision route accepts, decoded.
const MaximumImageBytes = 2 << 20

// The refusals, in the words the native command used.
var (
	errNoImage       = errors.New("Choose a PNG, JPEG or WebP image and try again")
	errImageTooLarge = errors.New("That image is too large to classify. Choose a smaller one")
	errNoCategory    = errors.New("Image classification failed")
	errNotSignedIn   = errors.New("Sign in to use AI commands.")
)

// classifyInput is the text the classification sends with the picture.
var classifyInput = json.RawMessage(`"Classify this image."`)

// categoryQuestion asks the category of the picture in one decision call.
var categoryQuestion = api.DecisionQuestion{
	ID: "category", Type: api.DecisionChoice, Question: "Which category best describes this image?",
	Options: []string{"photograph", "screenshot", "document", "illustration", "other"},
}

// pictureForm asks for one picture, typed by its extension, with the operating system's picker.
func pictureForm() api.View {
	picture := api.FieldOf("image", api.FieldFile, "Image")
	picture.Required = true
	picture.Extensions = []string{"png", "jpg", "jpeg"}
	picture.FilterName = "Images"
	picture.ButtonLabel = "Classify Image"
	return api.FormView(Title, []api.Field{picture}, api.Run(classifyAction, "Classify Image", nil))
}

// classify refuses a picture the decision route cannot take, asks the model for its category and lists it.
func classify(ctx api.Context, form api.FormValues) (api.View, error) {
	ai := ctx.AI()
	available, err := ai.IsAvailable()
	if err != nil {
		return api.View{}, err
	}
	if !available {
		return api.View{}, errNotSignedIn
	}
	path := form["image"].Text
	if err := checkPicture(ctx.Desktop().Files(), path); err != nil {
		return api.View{}, err
	}
	category, err := categoryOf(ai, path)
	if err != nil {
		return api.View{}, err
	}
	return api.ListView(ResultTitle, categoryItem(category)), nil
}

// checkPicture refuses a file that is not a PNG, JPEG or WebP picture, that cannot be read, or that is too large.
func checkPicture(files api.Files, path string) error {
	if mediaTypeOf(path) == "" {
		return errNoImage
	}
	info, err := files.Metadata(path)
	if err != nil {
		return errNoImage
	}
	if info.Size > MaximumImageBytes {
		return errImageTooLarge
	}
	return nil
}

// categoryOf is the category the model answers for the picture at path.
func categoryOf(ai api.AI, path string) (string, error) {
	result, err := ai.Decide(api.DecisionRequest{
		Input:       classifyInput,
		Attachments: []api.Attachment{ai.Attachments().Image(path, "", false)},
		Questions:   []api.DecisionQuestion{categoryQuestion},
	})
	if err != nil {
		return "", err
	}
	answer, ok := result.Answers["category"]
	if !ok || answer.Type != api.DecisionChoice {
		return "", errNoCategory
	}
	return answer.Choice, nil
}

// categoryItem is the one row of the result: the category, which Copy puts on the clipboard.
func categoryItem(category string) api.Item {
	return api.Item{
		ID: "category", Title: category, Subtitle: "Copy classification",
		Primary: ptr(api.CopyText("Copy", category, false)),
	}
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

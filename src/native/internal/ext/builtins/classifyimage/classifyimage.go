// Package classifyimage is the built-in that classifies a picture with AI. Its row, "Classify Image with AI", opens a form
// with one picture field; submitting the form sends the picture to the model and lists the category it answers, which
// Copy takes. The copy, the picture types and the size limit are the ones the native command had.
//
// Layer: ext. The package imports api, commandmatch, search and the standard library.
package classifyimage

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/commandmatch"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity.
const ID = "nevermind.classify-image"

// Title names the command, the form and the failures, as the palette lists it.
const Title = "Classify Image with AI"

// ResultTitle titles the list that shows the category.
const ResultTitle = "Image Classification"

// rowKeywords are the words the root matches the row by, besides its title.
const rowKeywords = "classify image picture screenshot photograph ai"

// rowSubtitle and rowIcon draw the row.
const (
	rowSubtitle = "Choose an image and identify its category"
	rowIcon     = "image"
)

// Action IDs of the built-in's handlers: the row opens the form, and the form's submit classifies its picture.
const (
	chooseAction   = "choose"
	classifyAction = "classify"
)

// Extension is the classify-image built-in.
type Extension struct{}

// Manifest identifies the built-in. It declares the AI call it makes and the picture file it reads.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: "Classify Image"},
		Capabilities: []api.Capability{api.CapabilityAI, api.CapabilityDesktopFiles},
	}
}

// SearchItems lists the row for a query that names it by its title or keywords. The row is not at the empty root: the
// native command sat past the root's row limit there, so it was only ever found by typing.
func (Extension) SearchItems(_ api.Context, query string) ([]api.Item, error) {
	score := commandmatch.Score(Title, rowKeywords, query)
	if query == "" || score == 0 {
		return nil, nil
	}
	return []api.Item{rowItem(score)}, nil
}

// rowItem is the command's row: its title and subtitle, and a primary action that opens the picture form.
func rowItem(score int) api.Item {
	return api.Item{
		Title: Title, Subtitle: rowSubtitle, Icon: rowIcon,
		Score: float64(score), Section: search.KindCommands, LearnKey: "cmd:" + Title,
		Primary: ptr(api.Run(chooseAction, Title, nil)),
	}
}

// RunAction opens the picture form, or classifies the picture the form submitted.
func (Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case chooseAction:
		view := pictureForm()
		return api.Result{View: &view}, nil
	case classifyAction:
		view, err := classify(ctx, in.Form)
		if err != nil {
			return api.Result{}, err
		}
		return api.Result{View: &view}, nil
	}
	return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
}

func ptr[T any](v T) *T { return &v }

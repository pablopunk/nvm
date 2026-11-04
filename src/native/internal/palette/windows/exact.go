package windows

import (
	"context"
	"strings"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// exactForm is one of the two exact placements, with the copy of the Electron form (window-management-exact-commands.ts):
// the two fields it takes, the unit it starts in, and their labels.
type exactForm struct {
	entryID    string
	entryTitle string
	submit     string
	icon       string
	keywords   string
	first      geometry.ExactField
	second     geometry.ExactField
	labels     map[geometry.ExactField]string
	unit       geometry.DimensionUnit
}

var moveForm = exactForm{
	entryID: moveEntryID, entryTitle: "Move Window…", submit: "Move Window", icon: "move-horizontal",
	keywords: "position exact coordinates offset place", first: geometry.FieldX, second: geometry.FieldY,
	labels: map[geometry.ExactField]string{geometry.FieldX: "X", geometry.FieldY: "Y"},
	unit:   geometry.UnitPoints,
}

var resizeForm = exactForm{
	entryID: resizeEntryID, entryTitle: "Resize Window…", submit: "Resize Window", icon: "scaling",
	keywords: "size exact dimensions width height", first: geometry.FieldWidth, second: geometry.FieldHeight,
	labels: map[geometry.ExactField]string{geometry.FieldWidth: "Width", geometry.FieldHeight: "Height"},
	unit:   geometry.UnitPercent,
}

const (
	moveEntryID   = "window.move"
	resizeEntryID = "window.resize"
)

// unitWords are the words that choose the unit of an exact placement. The default is the form's own unit.
var unitWords = map[string]geometry.DimensionUnit{
	"%": geometry.UnitPercent, "percent": geometry.UnitPercent,
	"pt": geometry.UnitPoints, "px": geometry.UnitPoints, "points": geometry.UnitPoints,
}

func exactEntries(d Deps) []Entry {
	return []Entry{exactEntry(d, moveForm), exactEntry(d, resizeForm)}
}

func exactEntry(d Deps, form exactForm) Entry {
	return Entry{
		ID: form.entryID, Title: form.entryTitle, Subtitle: Subtitle, Icon: form.icon, Hidden: true,
		Keywords: form.keywords, Run: func(h search.Host) { h.Push(exactView(d, form)) },
	}
}

// exactView asks for the two values in the palette's own field: "120 80" moves the window to X 120, Y 80. The rows
// say what the typed text will do, and Enter applies it.
func exactView(d Deps, form exactForm) *palette.View {
	return &palette.View{
		Title: form.entryTitle, Placeholder: "Two values, for example 120 80",
		Search: func(q string) []search.Item {
			return exactRows(d, form, q)
		},
	}
}

func exactRows(d Deps, form exactForm, q string) []search.Item {
	words, unit := exactWords(form, q)
	if len(words) != 2 {
		return []search.Item{exactHintRow(form)}
	}
	parsed := geometry.ParseExactDimensions(exactInputs(form, words, unit))
	if parsed.Failed() {
		return []search.Item{{Kind: search.KindWindows, Title: exactMessage(form, parsed), Icon: "triangle-alert"}}
	}
	return []search.Item{{
		Kind: search.KindWindows, Title: exactSummary(form, parsed.Dimensions), Icon: form.icon,
		Actions: []search.Action{exactAction(d, form, parsed.Dimensions)},
	}}
}

func exactHintRow(form exactForm) search.Item {
	return search.Item{Kind: search.KindWindows, Title: "Enter two values, for example 120 80",
		Subtitle: "Add % for a share of the work area. Use - to keep a value.", Icon: form.icon}
}

// exactWords splits the typed text into the form's two words and the unit. A trailing unit word applies to both.
func exactWords(form exactForm, q string) ([]string, geometry.DimensionUnit) {
	words := strings.Fields(q)
	unit := form.unit
	if len(words) > 0 {
		if chosen, ok := unitWords[strings.ToLower(words[len(words)-1])]; ok {
			unit = chosen
			words = words[:len(words)-1]
		}
	}
	return words, unit
}

func exactInputs(form exactForm, words []string, unit geometry.DimensionUnit) geometry.ExactInputs {
	var inputs geometry.ExactInputs
	assignExactInput(&inputs, form.first, exactInputOf(words[0], unit))
	assignExactInput(&inputs, form.second, exactInputOf(words[1], unit))
	return inputs
}

// exactInputOf is one typed word. "-" is a blank field, which keeps the window's current value.
func exactInputOf(word string, unit geometry.DimensionUnit) *geometry.ExactInput {
	text := word
	if word == "-" {
		text = ""
	}
	return &geometry.ExactInput{Text: text, Unit: string(unit)}
}

func assignExactInput(inputs *geometry.ExactInputs, field geometry.ExactField, input *geometry.ExactInput) {
	switch field {
	case geometry.FieldX:
		inputs.X = input
	case geometry.FieldY:
		inputs.Y = input
	case geometry.FieldWidth:
		inputs.Width = input
	case geometry.FieldHeight:
		inputs.Height = input
	}
}

// exactMessage is the copy under a refused field: the field's own range or unit message, else the whole form's.
func exactMessage(form exactForm, parsed geometry.ExactParseResult) string {
	for _, field := range []geometry.ExactField{form.first, form.second} {
		if message := parsed.Errors[field]; message != "" {
			return message
		}
	}
	return parsed.Message
}

// exactSummary says what Enter will do, e.g. "Move Window: X 120 pt, Y keeps current".
func exactSummary(form exactForm, values geometry.ExactValues) string {
	parts := make([]string, 0, 2)
	for _, field := range []geometry.ExactField{form.first, form.second} {
		parts = append(parts, form.labels[field]+" "+exactValueText(dimensionOf(values, field)))
	}
	return form.submit + ": " + strings.Join(parts, ", ")
}

func exactValueText(dimension *geometry.Dimension) string {
	if dimension == nil {
		return "keeps current"
	}
	return geometry.FormatExactDimension(*dimension)
}

func dimensionOf(values geometry.ExactValues, field geometry.ExactField) *geometry.Dimension {
	switch field {
	case geometry.FieldX:
		return values.X
	case geometry.FieldY:
		return values.Y
	case geometry.FieldWidth:
		return values.Width
	}
	return values.Height
}

// exactAction places the target with the parsed values. The target is read here, on the UI thread, because the
// palette hides right after the action and the keyboard then goes back to the window it was summoned from.
func exactAction(d Deps, form exactForm, values geometry.ExactValues) search.Action {
	layout := geometry.ExactLayoutFor(values)
	return search.Action{Title: form.submit, Dismiss: search.DismissAuto, Run: func(search.Host) {
		invocation := d.paletteInvocation()
		d.runInBackground("exact", form.submit, func(ctx context.Context) string {
			return d.runner().ExecuteLayout(ctx, layout, invocation)
		})
	}}
}

package windowmanagement

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// exactForm is one of the two exact placements, with the copy of the 1.x prompt: the fields it asks for, their labels and
// the unit it starts in.
type exactForm struct {
	kind         string
	title        string
	submit       string
	icon         string
	keys         [2]string
	labels       map[string]string
	descriptions map[string]string
	defaultUnit  string
}

// exactForms are Move Window and Resize Window, by kind.
var exactForms = map[string]exactForm{
	"move": {
		kind: "move", title: "Move Window…", submit: "Move Window", icon: "move", keys: [2]string{"x", "y"},
		labels:       map[string]string{"x": "X offset", "y": "Y offset"},
		descriptions: map[string]string{"x": "Distance from the left edge of the work area.", "y": "Distance from the top edge of the work area."},
		defaultUnit:  "points",
	},
	"resize": {
		kind: "resize", title: "Resize Window…", submit: "Resize Window", icon: "scaling", keys: [2]string{"width", "height"},
		labels:       map[string]string{"width": "Width", "height": "Height"},
		descriptions: map[string]string{"width": "Width of the window. Percent is of the work area width.", "height": "Height of the window. Percent is of the work area height."},
		defaultUnit:  "percent",
	},
}

// exactHint is the copy under every exact prompt, unless a message names what is wrong.
const exactHint = "Blank keeps the current value. Values are clamped inside the work area."

// Limits of an exact value: offsets reach the whole work area, sizes are above zero. Points cap at 100000.
const (
	maxExactPoints  = 100000
	maxExactPercent = 100
)

var decimalText = regexp.MustCompile(`^\d+(\.\d+)?$`)

func openExactRun(form exactForm) handlerFunc {
	return func(ctx api.Context, _ api.Invocation) (api.Result, error) {
		prompt := exactPrompt(ctx, form, nil, nil, "")
		return api.Result{Next: &prompt}, nil
	}
}

// submitExact places the window with the typed values, or asks again with what is wrong.
func (e Extension) submitExact(ctx api.Context, in api.Invocation) (api.Result, error) {
	form, ok := exactForms[in.Ref.Args["kind"]]
	if !ok {
		return api.Result{}, fmt.Errorf("no exact placement %q: %w", in.Ref.Args["kind"], api.ErrInvalidInput)
	}
	layout, errs, notice := exactLayout(form, in.Form)
	if notice != "" || len(errs) > 0 {
		prompt := exactPrompt(ctx, form, in.Form, errs, notice)
		return api.Result{View: prompt.View, Navigation: api.NavReplace}, nil
	}
	settings := LoadSettings(ctx, e.start)
	windows := ctx.Desktop().WindowManagement()
	if windows == nil {
		return toast(unavailableMessage, "error"), nil
	}
	result, err := windows.Apply(layout, optionsFor(settings))
	if err != nil {
		return api.Result{}, err
	}
	return layoutResult(result), nil
}

// exactPrompt is the form that asks for the two values and their unit, starting from values when it is asked again.
func exactPrompt(ctx api.Context, form exactForm, values api.FormValues, errs map[string]string, notice string) api.Action {
	unit := form.defaultUnit
	if chosen := values["unit"].Text; chosen != "" {
		unit = chosen
	}
	fields := []api.Field{{
		ID: "unit", Label: "Unit", Type: api.FieldDropdown, Value: api.TextValue(unit),
		Options:     []api.Option{{Title: "Points", Value: "points"}, {Title: "Percent of work area", Value: "percent"}},
		Description: "Applies to both values below.",
	}}
	for _, key := range form.keys {
		fields = append(fields, api.Field{
			ID: key, Label: form.labels[key], Type: api.FieldNumber, Value: values[key], Placeholder: "Blank keeps current",
			Description: form.descriptions[key], Error: errs[key],
		})
	}
	message := exactHint
	if notice != "" {
		message = notice
	}
	return ctx.Input().Prompt(api.PromptSpec{
		Title: form.title, Message: message, Fields: fields, SubmitTitle: form.submit,
		Action: api.Run(exactSubmitAction, form.submit, map[string]string{"kind": form.kind}),
	})
}

// exactLayout parses the typed values into an exact layout. Errors are per field; notice is the message when no value
// was given at all.
func exactLayout(form exactForm, values api.FormValues) (api.WindowLayout, map[string]string, string) {
	unit := values["unit"].Text
	if unit == "" {
		unit = form.defaultUnit
	}
	frame := api.ExactFrame{}
	errs := map[string]string{}
	given := 0
	for _, key := range form.keys {
		dimension, message := parseDimension(key, values[key].Text, unit)
		if message != "" {
			errs[key] = message
			continue
		}
		if dimension == nil {
			continue
		}
		given++
		assignExact(&frame, key, dimension)
	}
	if len(errs) > 0 {
		return api.WindowLayout{}, errs, ""
	}
	if given == 0 {
		return api.WindowLayout{}, nil, "Enter at least one value."
	}
	return api.WindowLayout{Kind: api.LayoutExact, Exact: &frame}, nil, ""
}

// parseDimension reads one typed value in a unit. Blank text is no value, and a value out of its range says so.
func parseDimension(key, text, unit string) (*api.Dimension, string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, ""
	}
	if unit != "points" && unit != "percent" {
		return nil, "Choose points or percent."
	}
	if !decimalText.MatchString(trimmed) {
		return nil, rangeMessage(key, unit)
	}
	value, _ := strconv.ParseFloat(trimmed, 64)
	if !exactInRange(key, unit, value) {
		return nil, rangeMessage(key, unit)
	}
	return &api.Dimension{Value: value, Unit: unit}, ""
}

func exactInRange(key, unit string, value float64) bool {
	limit := float64(maxExactPoints)
	if unit == "percent" {
		limit = maxExactPercent
	}
	if value > limit {
		return false
	}
	if isSize(key) {
		return value > 0
	}
	return value >= 0
}

func isSize(key string) bool {
	return key == "width" || key == "height"
}

func rangeMessage(key, unit string) string {
	maximum := strconv.Itoa(maxExactPoints)
	if unit == "percent" {
		maximum = strconv.Itoa(maxExactPercent) + "%"
	}
	if isSize(key) {
		return "Enter a number above 0, up to " + maximum + "."
	}
	return "Enter a number from 0 to " + maximum + "."
}

func assignExact(frame *api.ExactFrame, key string, dimension *api.Dimension) {
	switch key {
	case "x":
		frame.X = dimension
	case "y":
		frame.Y = dimension
	case "width":
		frame.Width = dimension
	case "height":
		frame.Height = dimension
	}
}

package api

// promptInput is the Input service. A prompt is a form view that the palette pushes; its submit action runs with the
// answers in Invocation.Form, so a prompt and a form share one validation and one submit path.
type promptInput struct{}

// NewInput returns the Input service the host hands to extensions.
func NewInput() Input {
	return promptInput{}
}

// Prompt builds the action that opens the prompt. A non-empty Message becomes a description row above the fields.
func (promptInput) Prompt(spec PromptSpec) Action {
	fields := spec.Fields
	if spec.Message != "" {
		message := Field{ID: promptMessageFieldID, Type: FieldDescription, Description: spec.Message}
		fields = append([]Field{message}, fields...)
	}
	submit := spec.Action
	view := View{
		Kind: ViewForm, Title: spec.Title,
		Form: &FormSpec{Fields: fields, Submit: &submit, SubmitTitle: spec.SubmitTitle},
	}
	return Action{Kind: ActionPushView, Title: spec.Title, View: &view}
}

// promptMessageFieldID names the description row that carries a prompt's message. Extensions do not use it.
const promptMessageFieldID = "prompt:message"

package windowmanagement

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// handlerFunc runs one action of the built-in.
type handlerFunc func(api.Context, api.Invocation) (api.Result, error)

// viewHandler maps the view and form actions of the built-in to their handlers.
func (e Extension) viewHandler(id string) (handlerFunc, bool) {
	switch id {
	case moveWindowID:
		return openExactRun(exactForms["move"]), true
	case resizeWindowID:
		return openExactRun(exactForms["resize"]), true
	case exactSubmitAction:
		return e.submitExact, true
	case switchCommandID:
		return switchRun, true
	case focusWindowAction:
		return focusWindowRun, true
	case applyWindowAction:
		return e.applyWindowRun, true
	case settingsCommandID:
		return e.settingsRun, true
	case settingToggle:
		return e.toggleSetting, true
	case settingCycle:
		return e.cycleSetting, true
	case settingSet:
		return e.setSetting, true
	case settingsReset:
		return e.resetSettings, true
	case customCommandID:
		return customRun, true
	}
	return nil, false
}

func switchRun(ctx api.Context, _ api.Invocation) (api.Result, error) {
	windows := ctx.Desktop().WindowManagement()
	if windows == nil {
		return toast(unavailableMessage, "error"), nil
	}
	listed, err := windows.Windows(api.WindowQuery{})
	if err != nil {
		return api.Result{}, err
	}
	view := switchView(listed)
	return api.Result{View: &view}, nil
}

func focusWindowRun(ctx api.Context, in api.Invocation) (api.Result, error) {
	return focusWindow(ctx, api.WindowHandle(in.Ref.Args["handle"]))
}

func (e Extension) applyWindowRun(ctx api.Context, in api.Invocation) (api.Result, error) {
	l, ok := layoutByID(in.Ref.Args["layout"])
	if !ok {
		return api.Result{}, fmt.Errorf("no layout %q: %w", in.Ref.Args["layout"], api.ErrInvalidInput)
	}
	return e.applyToWindow(ctx, api.WindowHandle(in.Ref.Args["handle"]), l)
}

func (e Extension) settingsRun(ctx api.Context, _ api.Invocation) (api.Result, error) {
	view := settingsView(ctx, e.start)
	return api.Result{View: &view}, nil
}

func customRun(_ api.Context, _ api.Invocation) (api.Result, error) {
	view := customView()
	return api.Result{View: &view}, nil
}

// replaceWithSettings redraws the settings view in place after a change, so the rows show the new values.
func (e Extension) replaceWithSettings(ctx api.Context) api.Result {
	view := settingsView(ctx, e.start)
	return api.Result{View: &view, Navigation: api.NavReplace}
}

func (e Extension) toggleSetting(ctx api.Context, in api.Invocation) (api.Result, error) {
	if _, err := ctx.Settings().Toggle(in.Ref.Args["id"]); err != nil {
		return api.Result{}, err
	}
	return e.replaceWithSettings(ctx), nil
}

func (e Extension) cycleSetting(ctx api.Context, in api.Invocation) (api.Result, error) {
	field, ok := bindingNamed(in.Ref.Args["id"])
	if !ok {
		return api.Result{}, fmt.Errorf("no setting %q: %w", in.Ref.Args["id"], api.ErrInvalidInput)
	}
	current := choiceValue(LoadSettings(ctx, e.start), field.id)
	if err := ctx.Settings().Set(field.id, nextOption(field.options, current)); err != nil {
		return api.Result{}, err
	}
	return e.replaceWithSettings(ctx), nil
}

func (e Extension) setSetting(ctx api.Context, in api.Invocation) (api.Result, error) {
	field, ok := bindingNamed(in.Ref.Args["id"])
	if !ok {
		return api.Result{}, fmt.Errorf("no setting %q: %w", in.Ref.Args["id"], api.ErrInvalidInput)
	}
	value, message := parseSettingText(field, in.Form["value"].Text)
	if message != "" {
		return toast(message, "error"), nil
	}
	if err := ctx.Settings().Set(field.id, value); err != nil {
		return api.Result{}, err
	}
	return e.replaceWithSettings(ctx), nil
}

func (e Extension) resetSettings(ctx api.Context, in api.Invocation) (api.Result, error) {
	if err := ResetSettings(ctx, in.Ref.Args["section"]); err != nil {
		return api.Result{}, err
	}
	return e.replaceWithSettings(ctx), nil
}

// choiceValue is the text choice of the setting id in s, or empty when it is not a choice.
func choiceValue(s Settings, id string) string {
	for _, field := range bindings(&s) {
		if field.id == id && field.text != nil {
			return *field.text
		}
	}
	return ""
}

// nextOption is the choice after current, wrapping around. A value the list does not hold starts at the first.
func nextOption(options []string, current string) string {
	for i, option := range options {
		if option == current {
			return options[(i+1)%len(options)]
		}
	}
	return options[0]
}

// bindingNamed finds a declared setting by its ID.
func bindingNamed(id string) (binding, bool) {
	defaults := DefaultSettings()
	for _, field := range bindings(&defaults) {
		if field.id == id {
			return field, true
		}
	}
	return binding{}, false
}

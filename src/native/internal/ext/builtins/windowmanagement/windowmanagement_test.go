package windowmanagement

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeWindows records the window calls the built-in makes and answers with the configured results.
type fakeWindows struct {
	api.WindowManagement
	applied    []api.WindowLayout
	options    []api.WindowManagementOptions
	targeted   []api.WindowHandle
	focused    []string
	switched   []string
	result     api.ApplyResult
	focusReply api.FocusResult
	listed     []api.WindowInfo
}

func (w *fakeWindows) Apply(layout api.WindowLayout, options api.WindowManagementOptions) (api.ApplyResult, error) {
	w.applied = append(w.applied, layout)
	w.options = append(w.options, options)
	return w.result, nil
}

func (w *fakeWindows) ApplyTo(handle api.WindowHandle, layout api.WindowLayout, options api.WindowManagementOptions) (api.ApplyResult, error) {
	w.targeted = append(w.targeted, handle)
	return w.Apply(layout, options)
}

func (w *fakeWindows) FocusNeighbor(direction string, _ bool) (api.FocusResult, error) {
	w.focused = append(w.focused, direction)
	return w.focusReply, nil
}

func (w *fakeWindows) SwitchDesktop(direction string) (api.FocusResult, error) {
	w.switched = append(w.switched, direction)
	return w.focusReply, nil
}

func (w *fakeWindows) Focus(handle api.WindowHandle) (api.FocusResult, error) {
	w.targeted = append(w.targeted, handle)
	return w.focusReply, nil
}

func (w *fakeWindows) Windows(api.WindowQuery) ([]api.WindowInfo, error) {
	return w.listed, nil
}

// fakeSettings keeps stored values as JSON, as the host does, and reports an error for a setting nothing stored.
type fakeSettings struct {
	api.Settings
	values map[string]json.RawMessage
}

func (s *fakeSettings) Get(id string, into any) error {
	raw, ok := s.values[id]
	if !ok {
		return fmt.Errorf("setting %q has no stored value", id)
	}
	return json.Unmarshal(raw, into)
}

func (s *fakeSettings) Set(id string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.values[id] = raw
	return nil
}

func (s *fakeSettings) Toggle(id string) (json.RawMessage, error) {
	var on bool
	_ = s.Get(id, &on)
	if err := s.Set(id, !on); err != nil {
		return nil, err
	}
	return s.values[id], nil
}

type fakeSystem struct {
	api.System
	supported bool
}

func (s fakeSystem) HasCapability(name string) bool {
	return s.supported && name == "window-management"
}

// fakeDesktop serves the window engine, or none where the platform has no window management.
type fakeDesktop struct {
	api.Desktop
	windows api.WindowManagement
}

func (d fakeDesktop) WindowManagement() api.WindowManagement {
	return d.windows
}

// fakeContext serves the services the built-in reads: the system, the desktop, the settings and the input builder.
type fakeContext struct {
	api.Context
	system   fakeSystem
	desktop  fakeDesktop
	settings *fakeSettings
}

func (c fakeContext) System() api.System     { return c.system }
func (c fakeContext) Desktop() api.Desktop   { return c.desktop }
func (c fakeContext) Settings() api.Settings { return c.settings }
func (c fakeContext) Input() api.Input       { return api.NewInput() }

func newContext(windows *fakeWindows) fakeContext {
	return fakeContext{
		system:   fakeSystem{supported: true},
		desktop:  fakeDesktop{windows: windows},
		settings: &fakeSettings{values: map[string]json.RawMessage{}},
	}
}

func newBuiltIn(t *testing.T) Extension {
	t.Helper()
	return New(DefaultSettings())
}

func runAction(t *testing.T, ctx api.Context, id string, args map[string]string, form api.FormValues) api.Result {
	t.Helper()
	result, err := newBuiltIn(t).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: id, Args: args}, Form: form})
	if err != nil {
		t.Fatalf("RunAction(%q): %v", id, err)
	}
	return result
}

func commandsOf(t *testing.T, ctx api.Context) map[string]api.Command {
	t.Helper()
	commands, err := newBuiltIn(t).Commands(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]api.Command{}
	for _, command := range commands {
		byID[command.ID] = command
	}
	return byID
}

func TestCommandsAreListedOnlyWhereTheSystemManagesWindows(t *testing.T) {
	windows := &fakeWindows{}
	if commands := commandsOf(t, fakeContext{system: fakeSystem{}, desktop: fakeDesktop{}, settings: &fakeSettings{}}); len(commands) != 0 {
		t.Fatalf("commands on a system without window management = %d, want none", len(commands))
	}
	if commands := commandsOf(t, newContext(windows)); len(commands) != len(layouts)+len(focusDirections)+len(desktopMoves)+5 {
		t.Fatalf("commands = %d, want every layout, focus, desktop and view command", len(commands))
	}
}

func TestEveryCommandIsValidAndKeepsItsOneXKey(t *testing.T) {
	for _, command := range allCommands() {
		if err := command.Validate(); err != nil {
			t.Fatalf("command %q is invalid: %v", command.ID, err)
		}
	}
	commands := commandsOf(t, newContext(&fakeWindows{}))
	if got := commands["layout.left-half"].ActionID; got != "extension-action:window-management:layout.left-half" {
		t.Fatalf("left half key = %q, want the 1.x key", got)
	}
	if got := commands["layout.focus-right"].ActionID; got != "extension-action:window-management:layout.focus-right" {
		t.Fatalf("focus right key = %q, want the 1.x key", got)
	}
	if got := commands["next-desktop"].ActionID; got != "extension-action:window-management:next-desktop" {
		t.Fatalf("next desktop key = %q, want the 1.x key", got)
	}
}

func TestShortcutCommandsAreTheLayoutsFocusMovesAndDesktopMoves(t *testing.T) {
	want := len(layouts) + len(focusDirections) + len(desktopMoves)
	if got := len(ShortcutCommands()); got != want {
		t.Fatalf("shortcut commands = %d, want %d", got, want)
	}
	for _, command := range ShortcutCommands() {
		if !command.Customizable || command.Mode != api.ModeNoView {
			t.Fatalf("shortcut command %q is not an in-place customizable command", command.ID)
		}
	}
}

func TestLayoutAppliesTheCatalogSpecWithTheSettingsOptions(t *testing.T) {
	windows := &fakeWindows{result: api.ApplyResult{OK: true, Outcome: "exact"}}
	ctx := newContext(windows)
	if result := runAction(t, ctx, "layout.left-half", nil, nil); result.Toast != nil {
		t.Fatalf("result = %+v, want silence on success", result)
	}
	applied := windows.applied[0]
	if applied.Kind != api.LayoutFraction || applied.Fraction.Width != 0.5 || applied.Fraction.Height != 1 || applied.CycleGroup != "left" {
		t.Fatalf("layout = %+v, want the left half in the left cycle group", applied)
	}
	options := windows.options[0]
	if options.OuterGap == nil || options.OuterGap.Top != 8 || options.InnerGap == nil || *options.InnerGap != 4 || options.CycleOrder != cycleHalfTwoThirdsThird {
		t.Fatalf("options = %+v, want the default gaps and cycle order", options)
	}
}

func TestSizedLayoutsReadTheirSettings(t *testing.T) {
	windows := &fakeWindows{result: api.ApplyResult{OK: true}}
	ctx := newContext(windows)
	ctx.settings.values["almostMaximizeWidthPercent"] = json.RawMessage("80")
	ctx.settings.values["outerGapTop"] = json.RawMessage("12")
	runAction(t, ctx, "layout.almost-maximize", nil, nil)
	applied := windows.applied[0]
	if applied.Kind != api.LayoutScaledCenter || applied.WidthFraction != 0.8 || applied.HeightFraction != 0.9 {
		t.Fatalf("almost maximize = %+v, want width 80%% and height 90%%", applied)
	}
	if got := windows.options[0].OuterGap.Top; got != 12 {
		t.Fatalf("top gap = %d, want the stored 12", got)
	}
}

func TestRefusedLayoutSaysWhy(t *testing.T) {
	windows := &fakeWindows{result: api.ApplyResult{OK: false, Message: "The window cannot be placed here."}}
	result := runAction(t, newContext(windows), "layout.maximize", nil, nil)
	if result.Toast == nil || result.Toast.Message != "The window cannot be placed here." || result.Toast.Tone != "error" {
		t.Fatalf("result = %+v, want the refusal as an error toast", result)
	}
}

func TestPartialLayoutSaysItsReason(t *testing.T) {
	windows := &fakeWindows{result: api.ApplyResult{OK: true, Outcome: "partial", Reason: "The app has a minimum width."}}
	result := runAction(t, newContext(windows), "layout.center", nil, nil)
	if result.Toast == nil || result.Toast.Message != "The app has a minimum width." {
		t.Fatalf("result = %+v, want the partial reason", result)
	}
}

func TestLayoutsSayTheyAreUnavailableWithoutWindowManagement(t *testing.T) {
	ctx := fakeContext{system: fakeSystem{supported: true}, desktop: fakeDesktop{}, settings: &fakeSettings{values: map[string]json.RawMessage{}}}
	result := runAction(t, ctx, "layout.left-half", nil, nil)
	if result.Toast == nil || result.Toast.Message != unavailableMessage {
		t.Fatalf("result = %+v, want the unavailable message", result)
	}
}

func TestFocusMovesAskTheNeighborInThatDirection(t *testing.T) {
	windows := &fakeWindows{focusReply: api.FocusResult{OK: false, Reason: "no-neighbor", Message: "No window to the left."}}
	result := runAction(t, newContext(windows), "layout.focus-left", nil, nil)
	if len(windows.focused) != 1 || windows.focused[0] != "left" {
		t.Fatalf("focused = %v, want left", windows.focused)
	}
	if result.Toast == nil || result.Toast.Message != "No window to the left." {
		t.Fatalf("result = %+v, want the no-neighbor message", result)
	}
}

func TestNextAndPreviousDesktopSwitch(t *testing.T) {
	windows := &fakeWindows{focusReply: api.FocusResult{OK: true}}
	ctx := newContext(windows)
	runAction(t, ctx, "next-desktop", nil, nil)
	runAction(t, ctx, "previous-desktop", nil, nil)
	if strings.Join(windows.switched, ",") != "next,previous" {
		t.Fatalf("switched = %v, want next then previous", windows.switched)
	}
}

func TestExactMoveAsksForTheValuesThenPlacesTheWindow(t *testing.T) {
	windows := &fakeWindows{result: api.ApplyResult{OK: true}}
	ctx := newContext(windows)
	prompt := runAction(t, ctx, "move-window", nil, nil)
	if prompt.Next == nil || prompt.Next.View == nil || len(prompt.Next.View.Form.Fields) != 4 {
		t.Fatalf("result = %+v, want a form with a unit and two offsets", prompt)
	}
	form := api.FormValues{"unit": api.TextValue("points"), "x": api.TextValue("120"), "y": api.TextValue("")}
	runAction(t, ctx, exactSubmitAction, map[string]string{"kind": "move"}, form)
	exact := windows.applied[0].Exact
	if exact == nil || exact.X == nil || exact.X.Value != 120 || exact.X.Unit != "points" || exact.Y != nil {
		t.Fatalf("exact = %+v, want X 120 points and Y kept", exact)
	}
}

func TestExactValueOutOfRangeAsksAgainWithTheError(t *testing.T) {
	windows := &fakeWindows{}
	result := runAction(t, newContext(windows), exactSubmitAction, map[string]string{"kind": "resize"},
		api.FormValues{"unit": api.TextValue("percent"), "width": api.TextValue("150")})
	if result.Navigation != api.NavReplace || result.View == nil {
		t.Fatalf("result = %+v, want the form replaced", result)
	}
	if len(windows.applied) != 0 {
		t.Fatal("a refused exact value reached the window engine")
	}
	var message string
	for _, field := range result.View.Form.Fields {
		if field.ID == "width" {
			message = field.Error
		}
	}
	if message != "Enter a number above 0, up to 100%." {
		t.Fatalf("error = %q, want the percent range", message)
	}
}

func TestExactValuesNeedAtLeastOne(t *testing.T) {
	result := runAction(t, newContext(&fakeWindows{}), exactSubmitAction, map[string]string{"kind": "move"}, api.FormValues{"unit": api.TextValue("points")})
	if result.View == nil || result.View.Form == nil {
		t.Fatalf("result = %+v, want the form again", result)
	}
}

func TestSettingsDeclareTheLayoutValuesWithTheirTypes(t *testing.T) {
	definitions := New(DefaultSettings()).Manifest().Settings
	byID := map[string]api.SettingDefinition{}
	for _, definition := range definitions {
		byID[definition.ID] = definition
	}
	if byID["outerGap"].Type != api.SettingNumber || byID["cycle"].Type != api.SettingBoolean || byID["cycleOrder"].Type != api.SettingString {
		t.Fatalf("types = %+v, want number, flag and text choices", byID)
	}
	if byID["outerGapTop"].Type != "" || string(byID["outerGapTop"].Value) != "null" {
		t.Fatalf("outer gap top = %+v, want an untyped blank gap", byID["outerGapTop"])
	}
}

func TestSettingsViewFlipsAFlagAndCyclesAChoice(t *testing.T) {
	ctx := newContext(&fakeWindows{})
	ctx.settings.values["cycle"] = json.RawMessage("true")
	runAction(t, ctx, settingToggle, map[string]string{"id": "cycle"}, nil)
	if string(ctx.settings.values["cycle"]) != "false" {
		t.Fatalf("cycle = %s, want false after the toggle", ctx.settings.values["cycle"])
	}
	runAction(t, ctx, settingCycle, map[string]string{"id": "cycleOrder"}, nil)
	if string(ctx.settings.values["cycleOrder"]) != `"half-third-two-thirds"` {
		t.Fatalf("cycleOrder = %s, want the second choice", ctx.settings.values["cycleOrder"])
	}
}

func TestSettingNumberIsSavedWithinItsRange(t *testing.T) {
	ctx := newContext(&fakeWindows{})
	result := runAction(t, ctx, settingSet, map[string]string{"id": "resizeStepPercent"}, api.FormValues{"value": api.TextValue("40")})
	if result.Toast == nil || result.Toast.Message != "Enter a number from 1 to 25." {
		t.Fatalf("result = %+v, want the range message", result)
	}
	runAction(t, ctx, settingSet, map[string]string{"id": "resizeStepPercent"}, api.FormValues{"value": api.TextValue("10")})
	if string(ctx.settings.values["resizeStepPercent"]) != "10" {
		t.Fatalf("resizeStepPercent = %s, want 10", ctx.settings.values["resizeStepPercent"])
	}
	runAction(t, ctx, settingSet, map[string]string{"id": "outerGapTop"}, api.FormValues{"value": api.TextValue("")})
	if string(ctx.settings.values["outerGapTop"]) != "null" {
		t.Fatalf("outerGapTop = %s, want null after a blank", ctx.settings.values["outerGapTop"])
	}
}

func TestResetRestoresTheDefaultsOfTheSection(t *testing.T) {
	ctx := newContext(&fakeWindows{})
	ctx.settings.values["innerGap"] = json.RawMessage("9")
	ctx.settings.values["cycle"] = json.RawMessage("false")
	runAction(t, ctx, settingsReset, map[string]string{"section": sectionGaps}, nil)
	if string(ctx.settings.values["innerGap"]) != "4" {
		t.Fatalf("innerGap = %s, want the default 4", ctx.settings.values["innerGap"])
	}
	if string(ctx.settings.values["cycle"]) != "false" {
		t.Fatal("a section reset changed a setting of another section")
	}
}

func TestSwitchWindowListsWindowsAndFocusesOne(t *testing.T) {
	windows := &fakeWindows{listed: []api.WindowInfo{{Handle: "w-1", AppName: "Notes", Title: "Ideas"}, {Handle: "w-2", AppName: "Terminal"}}}
	result := runAction(t, newContext(windows), switchCommandID, nil, nil)
	if result.View == nil || len(result.View.Items) != 2 {
		t.Fatalf("result = %+v, want a list of the two windows", result)
	}
	first := result.View.Items[0]
	if first.Title != "Ideas" || first.Primary == nil || first.Primary.Ref.ID != focusWindowAction || first.Primary.Ref.Args["handle"] != "w-1" {
		t.Fatalf("first row = %+v, want Ideas focused by its handle", first)
	}
	if got := result.View.Items[1].Title; got != "Terminal" {
		t.Fatalf("untitled window row = %q, want its app name", got)
	}
}

func TestCustomWindowCommandsPointToAI(t *testing.T) {
	result := runAction(t, newContext(&fakeWindows{}), customCommandID, nil, nil)
	if result.View == nil || len(result.View.Items) != 1 || result.View.Items[0].Title != "Ask AI to build it" {
		t.Fatalf("result = %+v, want the AI notice", result)
	}
}

func TestUnknownActionIsAnInvalidInput(t *testing.T) {
	_, err := newBuiltIn(t).RunAction(newContext(&fakeWindows{}), api.Invocation{Ref: api.ActionRef{ID: "layout.nope"}})
	if err == nil {
		t.Fatal("an unknown layout ran")
	}
}

package api

import (
	"errors"
	"fmt"
)

// SettingRowKind is how a settings row is drawn and edited: toggle flips a boolean, choice steps through Choices,
// recorder opens the host's shortcut recorder, text edits a string, and action runs its Action.
type SettingRowKind string

const (
	SettingRowToggle   SettingRowKind = "toggle"
	SettingRowChoice   SettingRowKind = "choice"
	SettingRowRecorder SettingRowKind = "recorder"
	SettingRowText     SettingRowKind = "text"
	SettingRowAction   SettingRowKind = "action"
)

// settingRowLimit bounds the rows of one settings view, counting the extra actions of each row.
const settingRowLimit = 200

// SettingChoice is one value of a choice row.
type SettingChoice struct {
	Value string `json:"value"`
	Title string `json:"title"`
}

// SettingRow is one row of a settings view, storing Setting.
type SettingRow struct {
	ID          string          `json:"id"`
	Kind        SettingRowKind  `json:"kind"`
	Setting     string          `json:"setting,omitempty"`
	Title       string          `json:"title"`
	Subtitle    string          `json:"subtitle,omitempty"`
	Icon        string          `json:"icon,omitempty"`
	Accessory   string          `json:"accessory,omitempty"`
	ActionTitle string          `json:"actionTitle,omitempty"`
	Choices     []SettingChoice `json:"choices,omitempty"`
	Action      *Action         `json:"action,omitempty"`
	Also        []SettingRow    `json:"also,omitempty"`
}

// SettingsSpec is a settings view's rows.
type SettingsSpec struct {
	Rows []SettingRow `json:"rows"`
}

// SettingsView is a settings page.
func SettingsView(title string, rows ...SettingRow) View {
	return View{Kind: ViewSettings, Title: title, Settings: &SettingsSpec{Rows: rows}}
}

// Validate checks every row: its kind, its setting or action, its choices, and that row ids are unique. Setting ids
// are checked against the manifest by the host, which owns the declarations.
func (s SettingsSpec) Validate() error {
	seen := map[string]bool{}
	count := 0
	for _, row := range s.Rows {
		if err := validateSettingRow(row, seen, &count, true); err != nil {
			return err
		}
	}
	return nil
}

func validateSettingRow(row SettingRow, seen map[string]bool, count *int, primary bool) error {
	*count++
	if *count > settingRowLimit {
		return fmt.Errorf("a settings view has at most %d rows and actions", settingRowLimit)
	}
	if row.ID == "" || len(row.ID) > maxSettingIDLength {
		return fmt.Errorf("settings row %q needs an id of at most %d characters", row.Title, maxSettingIDLength)
	}
	if seen[row.ID] {
		return fmt.Errorf("settings row id %q is used twice", row.ID)
	}
	seen[row.ID] = true
	if row.Title == "" {
		return fmt.Errorf("settings row %q needs a title", row.ID)
	}
	if err := validateSettingRowKind(row); err != nil {
		return err
	}
	if !primary && len(row.Also) > 0 {
		return fmt.Errorf("settings row %q: an extra action cannot have its own extra actions", row.ID)
	}
	for _, also := range row.Also {
		if err := validateSettingRow(also, seen, count, false); err != nil {
			return err
		}
	}
	return nil
}

// validateSettingRowKind checks that the fields a row kind needs are present and the ones it does not need are absent.
func validateSettingRowKind(row SettingRow) error {
	switch row.Kind {
	case SettingRowToggle, SettingRowRecorder, SettingRowText:
		if len(row.Choices) > 0 {
			return fmt.Errorf("settings row %q (%s) takes no choices", row.ID, row.Kind)
		}
		return requireSetting(row)
	case SettingRowChoice:
		if err := requireSetting(row); err != nil {
			return err
		}
		return validateChoices(row)
	case SettingRowAction:
		if row.Setting != "" || len(row.Choices) > 0 {
			return fmt.Errorf("settings row %q: an action row stores no setting or choices", row.ID)
		}
		if row.Action == nil {
			return fmt.Errorf("settings row %q: an action row needs an action", row.ID)
		}
		return row.Action.Validate()
	}
	return fmt.Errorf("settings row %q has unknown kind %q", row.ID, row.Kind)
}

func requireSetting(row SettingRow) error {
	if row.Setting == "" {
		return fmt.Errorf("settings row %q (%s) needs a setting", row.ID, row.Kind)
	}
	if !settingIDPattern.MatchString(row.Setting) {
		return fmt.Errorf("settings row %q names setting %q, which is not a valid setting id", row.ID, row.Setting)
	}
	if row.Action != nil {
		return fmt.Errorf("settings row %q (%s) takes no action", row.ID, row.Kind)
	}
	return nil
}

func validateChoices(row SettingRow) error {
	if len(row.Choices) < 2 {
		return fmt.Errorf("settings row %q needs at least two choices", row.ID)
	}
	values := map[string]bool{}
	for _, choice := range row.Choices {
		if choice.Value == "" || choice.Title == "" {
			return errors.New("every setting choice needs a value and a title")
		}
		if values[choice.Value] {
			return fmt.Errorf("settings row %q repeats choice %q", row.ID, choice.Value)
		}
		values[choice.Value] = true
	}
	return nil
}

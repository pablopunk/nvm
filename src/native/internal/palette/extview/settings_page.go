package extview

import (
	"encoding/json"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// settingsPage draws a settings view. The rows are searched by their titles, and each row's values are read on every
// search, so a change shows on the next frame.
func (s *Session) settingsPage() *palette.View {
	return &palette.View{Title: s.view.Title, Placeholder: s.view.Placeholder, Search: s.settingItems}
}

// settingItems are the rows whose titles contain the query, as the host's settings view has always searched them.
func (s *Session) settingItems(query string) []search.Item {
	if s.view.Settings == nil {
		return nil
	}
	needle := strings.ToLower(query)
	items := []search.Item{}
	for _, row := range s.view.Settings.Rows {
		if needle == "" || strings.Contains(strings.ToLower(row.Title), needle) {
			items = append(items, s.settingItem(row))
		}
	}
	return items
}

func (s *Session) settingItem(row api.SettingRow) search.Item {
	actions := []search.Action{s.settingAction(row)}
	for _, also := range row.Also {
		actions = append(actions, s.settingAction(also))
	}
	subtitle := row.Subtitle
	if text, ok := s.hostRowSubtitle(row.ID); ok {
		subtitle = text
	}
	return search.Item{Title: row.Title, Subtitle: subtitle, Icon: row.Icon, Accessory: s.settingAccessory(row), Actions: actions}
}

func (s *Session) hostRowSubtitle(rowID string) (string, bool) {
	if s.deps.RowAccessory == nil {
		return "", false
	}
	_, subtitle, ok := s.deps.RowAccessory(s.extID, rowID)
	return subtitle, ok && subtitle != ""
}

// settingAccessory is the host's text for a row: the host's own text for the row when it has one, else the stored value
// in the form its kind shows, else the row's fixed accessory.
func (s *Session) settingAccessory(row api.SettingRow) string {
	if text, ok := s.hostRowAccessory(row.ID); ok {
		return text
	}
	if text, ok := s.sharedDisplay(row); ok {
		return text
	}
	if row.Kind == api.SettingRowAction || row.Kind == api.SettingRowRecorder {
		return row.Accessory
	}
	value, err := s.readSetting(row)
	if err != nil {
		return row.Accessory
	}
	switch row.Kind {
	case api.SettingRowToggle:
		return onOff(decodeBool(value))
	case api.SettingRowChoice:
		return settingChoiceTitle(row.Choices, decodeString(value))
	case api.SettingRowText:
		return decodeString(value)
	}
	return row.Accessory
}

// sharedDisplay is the display text another built-in publishes for the setting the row shows, when it has published one.
func (s *Session) sharedDisplay(row api.SettingRow) (string, bool) {
	if row.Shows == nil || s.deps.ReadDisplay == nil {
		return "", false
	}
	return s.deps.ReadDisplay(s.extID, *row.Shows)
}

func (s *Session) hostRowAccessory(rowID string) (string, bool) {
	if s.deps.RowAccessory == nil {
		return "", false
	}
	accessory, _, ok := s.deps.RowAccessory(s.extID, rowID)
	return accessory, ok
}

// settingAction maps a row to the action it runs. Toggle, choice, recorder and text rows edit their setting; an action
// row runs its declared action.
func (s *Session) settingAction(row api.SettingRow) search.Action {
	switch row.Kind {
	case api.SettingRowToggle:
		return s.toggleAction(row)
	case api.SettingRowChoice:
		return s.choiceAction(row)
	case api.SettingRowRecorder:
		return s.recorderAction(row)
	case api.SettingRowText:
		return s.textAction(row)
	}
	if row.Action == nil {
		return search.Action{Title: orDefault(row.ActionTitle, "Run"), Run: func(h search.Host) { h.Toast(unavailableMessage) }}
	}
	return s.action(*row.Action, row.ID)
}

func (s *Session) toggleAction(row api.SettingRow) search.Action {
	return search.Action{Title: orDefault(row.ActionTitle, "Toggle"), Dismiss: search.DismissNever, Run: func(h search.Host) {
		value, err := s.readSetting(row)
		if err == nil {
			err = s.writeSetting(row, mustJSON(!decodeBool(value)))
		}
		s.reportSettingFailure(h, err)
		h.Refresh()
	}}
}

// choiceAction steps the setting to the choice after the current one, and back to the first after the last.
func (s *Session) choiceAction(row api.SettingRow) search.Action {
	return search.Action{Title: orDefault(row.ActionTitle, "Next"), Dismiss: search.DismissNever, Run: func(h search.Host) {
		value, err := s.readSetting(row)
		if err == nil {
			err = s.writeSetting(row, mustJSON(nextChoice(row.Choices, decodeString(value))))
		}
		s.reportSettingFailure(h, err)
		h.Refresh()
	}}
}

// recorderAction opens the host's recorder for the setting. The host saves what is recorded.
func (s *Session) recorderAction(row api.SettingRow) search.Action {
	return search.Action{Title: orDefault(row.ActionTitle, "Change…"), Run: func(h search.Host) {
		if s.deps.RecordSetting == nil {
			h.Toast(unavailableMessage)
			return
		}
		s.deps.RecordSetting(h, s.extID, row.Setting)
	}}
}

// textAction pushes an editor for the setting: the text the user types is the value, and Enter saves it.
func (s *Session) textAction(row api.SettingRow) search.Action {
	return search.Action{Title: orDefault(row.ActionTitle, "Edit"), Dismiss: search.DismissNever, Run: func(h search.Host) {
		h.Push(s.settingTextPage(row))
	}}
}

func (s *Session) settingTextPage(row api.SettingRow) *palette.View {
	current := ""
	if value, err := s.readSetting(row); err == nil {
		current = decodeString(value)
	}
	return &palette.View{Title: row.Title, Placeholder: orDefault(current, row.Title), Search: func(query string) []search.Item {
		if query == "" {
			return nil
		}
		return []search.Item{{Title: query, Subtitle: "Enter saves this value", Actions: []search.Action{{Title: "Save", Dismiss: search.DismissNever, Run: func(h search.Host) {
			h.Pop()
			err := s.writeSetting(row, mustJSON(query))
			s.reportSettingFailure(h, err)
			h.Refresh()
		}}}}}
	}}
}

// readSetting is the stored value of the row's setting, or an error when the host cannot read it.
func (s *Session) readSetting(row api.SettingRow) (json.RawMessage, error) {
	if s.deps.ReadSetting == nil {
		return nil, errSettingsUnavailable
	}
	return s.deps.ReadSetting(s.extID, row.Setting)
}

func (s *Session) writeSetting(row api.SettingRow, value json.RawMessage) error {
	if s.deps.WriteSetting == nil {
		return errSettingsUnavailable
	}
	return s.deps.WriteSetting(s.extID, row.Setting, value)
}

// reportSettingFailure shows why a change was not saved, as a toast. A saved change shows nothing.
func (s *Session) reportSettingFailure(h search.Host, err error) {
	if err != nil {
		h.Toast(err.Error())
	}
}

package extview

import (
	"encoding/json"
	"errors"
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

//lint:ignore ST1005 user-facing copy, shown as written
var errSettingsUnavailable = errors.New("This setting cannot be changed here")

func decodeBool(value json.RawMessage) bool {
	var flag bool
	_ = json.Unmarshal(value, &flag)
	return flag
}

func decodeString(value json.RawMessage) string {
	var text string
	_ = json.Unmarshal(value, &text)
	return text
}

// mustJSON encodes a string or boolean, which always encodes.
func mustJSON(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

// settingChoiceTitle is the title of the choice the value names, or the value itself when no choice has it.
func settingChoiceTitle(choices []api.SettingChoice, value string) string {
	if i := slices.IndexFunc(choices, func(choice api.SettingChoice) bool { return choice.Value == value }); i >= 0 {
		return choices[i].Title
	}
	return value
}

// nextChoice is the value after the current one, wrapping to the first. A value no choice names steps to the first.
func nextChoice(choices []api.SettingChoice, value string) string {
	i := slices.IndexFunc(choices, func(choice api.SettingChoice) bool { return choice.Value == value })
	return choices[(i+1)%len(choices)].Value
}

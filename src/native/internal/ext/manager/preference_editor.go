package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// PreferenceSource is what the preference editor reads and writes. *Manager implements it.
type PreferenceSource interface {
	DeclaredPreferences() []ExtensionPreferences
	SettingValue(id, key string) (json.RawMessage, error)
	SaveSettingForUser(id, key string, value json.RawMessage) error
}

var (
	errPreferencesUnavailable = fmt.Errorf("preferences cannot be read before the extensions start: %w", api.ErrNotAvailable)
	// errPreferenceMoved refuses a change prepared on an older value, such as one the user changed while a question
	// waited.
	errPreferenceMoved = fmt.Errorf("the setting changed since this change was prepared; nothing changed: %w", api.ErrInvalidInput)
)

// PreferenceEditor reads and changes the declared settings of every extension for the user, as services.PreferenceEditor
// says. Every read and write runs on the UI thread through onUI, because a built-in's host store, such as the Settings
// page's, keeps its values in state only that thread touches.
type PreferenceEditor struct {
	source      func() PreferenceSource
	onUI        func(func())
	afterChange func(extensionID string)
	// recordedElsewhere names the settings only their own page records, such as a shortcut a key recorder sets, with
	// where to change each one instead.
	recordedElsewhere map[api.SettingRef]string
}

// NewPreferenceEditor returns the editor over source, which returns nil until the extensions start. afterChange, when
// set, runs on the UI thread after a value was saved, so the app can redraw what shows it.
func NewPreferenceEditor(source func() PreferenceSource, onUI func(func()), afterChange func(extensionID string),
	recordedElsewhere map[api.SettingRef]string) PreferenceEditor {
	return PreferenceEditor{source: source, onUI: onUI, afterChange: afterChange, recordedElsewhere: recordedElsewhere}
}

// List returns the settings that match every word of query, as they are now.
func (e PreferenceEditor) List(ctx context.Context, query string) ([]api.PreferenceRecord, error) {
	var records []api.PreferenceRecord
	err := e.onUIThread(ctx, func(source PreferenceSource) error {
		records = matchingRecords(e.records(source), query)
		return nil
	})
	return records, err
}

// Plan prepares saving value as the setting, against its value now, and saves nothing.
func (e PreferenceEditor) Plan(ctx context.Context, extensionID, settingID string, value json.RawMessage) (protocol.PreferenceChange, error) {
	var change protocol.PreferenceChange
	err := e.onUIThread(ctx, func(source PreferenceSource) error {
		var planErr error
		change, planErr = e.planOn(source, extensionID, settingID, value)
		return planErr
	})
	return change, err
}

// Set saves value as the setting and returns the setting as saved. A non-empty expect refuses the change when the value
// moved since that revision.
func (e PreferenceEditor) Set(ctx context.Context, extensionID, settingID string, value json.RawMessage, expect string) (api.PreferenceRecord, error) {
	var saved api.PreferenceRecord
	err := e.onUIThread(ctx, func(source PreferenceSource) error {
		change, err := e.planOn(source, extensionID, settingID, value)
		if expect != "" && change.Revision != "" && change.Revision != expect {
			return errPreferenceMoved
		}
		if err != nil {
			return err
		}
		if err := source.SaveSettingForUser(extensionID, settingID, value); err != nil {
			return preferenceRefusal(err)
		}
		if e.afterChange != nil {
			e.afterChange(extensionID)
		}
		saved = change.Current
		saved.Value = e.valueNow(source, saved, value)
		return nil
	})
	return saved, err
}

// onUIThread runs read on the UI thread with the source, and waits for it; see platform.CallOnUI.
func (e PreferenceEditor) onUIThread(ctx context.Context, read func(PreferenceSource) error) error {
	return platform.CallOnUI(ctx, e.onUI, func() error {
		source := e.source()
		if source == nil {
			return errPreferencesUnavailable
		}
		return read(source)
	})
}

// valueNow is the setting's value as stored after a save, or the saved value when it cannot be read back.
func (e PreferenceEditor) valueNow(source PreferenceSource, record api.PreferenceRecord, saved json.RawMessage) json.RawMessage {
	if value, err := source.SettingValue(record.ExtensionID, record.ID); err == nil {
		return value
	}
	return saved
}

// preferenceRefusal marks a save the extension's services refused as invalid input, so the caller reads its reason.
func preferenceRefusal(err error) error {
	if err == nil || errors.Is(err, api.ErrInvalidInput) || errors.Is(err, api.ErrNotAvailable) {
		return err
	}
	return fmt.Errorf("%w: %w", err, api.ErrInvalidInput)
}

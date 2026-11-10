package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/settings"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/websearch"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/portable"
	"github.com/pablopunk/nvm/native/internal/update"
)

// errSettingUnknown is returned for a setting id the Settings page does not store.
var errSettingUnknown = errors.New("this setting is not stored by the app")

// settingsStore is the host store of the Settings built-in. Each setting id reads and writes the value the app already
// keeps, and a save runs what the app does when that value changes.
type settingsStore struct {
	p *App
}

// builtInSettingsStore returns the host store for a built-in's settings, or nil for every other extension. The result is
// an interface, so a nil store must be a nil interface and not a typed nil.
func (p *App) builtInSettingsStore(id string) services.SettingsHost {
	if id != settings.ID {
		return nil
	}
	return &settingsStore{p: p}
}

// Load returns the stored value of a setting, always found: the app holds a value for each of its settings.
func (s *settingsStore) Load(key string) (json.RawMessage, bool) {
	value, err := s.value(key)
	if err != nil {
		return nil, false
	}
	raw, _ := json.Marshal(value)
	return raw, true
}

// Save stores a value the page changed, and runs the change it makes in the app.
func (s *settingsStore) Save(key string, raw json.RawMessage) error {
	return s.write(key, raw)
}

func (s *settingsStore) value(key string) (any, error) {
	p := s.p
	switch key {
	case settings.SettingPaletteShortcut, settings.SettingHyperKey:
		return "", nil
	case settings.SettingSearchEngine:
		return p.webEngineKey(), nil
	case settings.SettingClipboardHistory:
		return !p.S.ClipOff, nil
	case settings.SettingClipboardLinkPreviews:
		return !p.S.ClipNoLinks, nil
	case settings.SettingClipboardLimit:
		return strconv.Itoa(p.S.ClipLimit), nil
	case settings.SettingClipboardRootList:
		return !p.S.ClipRootOff, nil
	case settings.SettingClipboardRootSearch:
		return !p.S.ClipRootSearchOff, nil
	case settings.SettingDiagnostics:
		return !p.diagnosticsOff(), nil
	case settings.SettingUpdateChannel:
		return string(p.updateSnapshot().Channel), nil
	case settings.SettingLaunchAtLogin:
		return p.launchAtLogin.Enabled(), nil
	case settings.SettingMenuBarIconStyle:
		return orDefaultString(p.menuBarIconStyle(), "bolt-filled"), nil
	}
	return nil, errSettingUnknown
}

func (s *settingsStore) write(key string, raw json.RawMessage) error {
	p := s.p
	switch key {
	case settings.SettingSearchEngine:
		var engine string
		if err := json.Unmarshal(raw, &engine); err != nil {
			return err
		}
		m := p.extensionManager()
		if m == nil {
			return errNoSharedConfig
		}
		return m.WriteSetting(websearch.ID, websearch.EngineSettingID, engine)
	case settings.SettingClipboardHistory:
		return s.saveFlag(raw, func(on bool) { p.S.ClipOff = !on; p.S.save(); p.clip.setEnabled(on) })
	case settings.SettingClipboardLinkPreviews:
		return s.saveFlag(raw, func(on bool) { p.S.ClipNoLinks = !on; p.S.save(); p.clip.setLinkPreviews(on) })
	case settings.SettingClipboardRootList:
		return s.saveFlag(raw, func(on bool) { p.S.ClipRootOff = !on; p.S.save() })
	case settings.SettingClipboardRootSearch:
		return s.saveFlag(raw, func(on bool) { p.S.ClipRootSearchOff = !on; p.S.save() })
	case settings.SettingClipboardLimit:
		return s.saveClipboardLimit(raw)
	case settings.SettingDiagnostics:
		return s.saveDiagnostics(raw)
	case settings.SettingUpdateChannel:
		return s.saveUpdateChannel(raw)
	case settings.SettingLaunchAtLogin:
		return s.saveLaunchAtLogin(raw)
	case settings.SettingMenuBarIconStyle:
		return s.saveMenuBarIconStyle(raw)
	}
	return errSettingUnknown
}

// saveFlag decodes a boolean and hands it to apply.
func (s *settingsStore) saveFlag(raw json.RawMessage, apply func(on bool)) error {
	var on bool
	if err := json.Unmarshal(raw, &on); err != nil {
		return err
	}
	apply(on)
	return nil
}

func (s *settingsStore) saveClipboardLimit(raw json.RawMessage) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return fmt.Errorf("clipboard size %q is not a size", value)
	}
	s.p.S.ClipLimit = limit
	s.p.S.save()
	s.p.clip.setLimit(limit)
	return nil
}

func (s *settingsStore) saveDiagnostics(raw json.RawMessage) error {
	return s.saveFlag(raw, func(on bool) {
		if err := s.p.setDiagnosticsOff(!on); err != nil {
			log.Println("diagnostics setting not saved:", err)
		}
		s.p.updateAnalyticsConsent(on)
	})
}

func (s *settingsStore) saveUpdateChannel(raw json.RawMessage) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if s.p.updates == nil {
		return update.ErrChannelUnavailable
	}
	return s.p.updates.SetChannel(update.ParseChannel(value))
}

func (s *settingsStore) saveLaunchAtLogin(raw json.RawMessage) error {
	var on bool
	if err := json.Unmarshal(raw, &on); err != nil {
		return err
	}
	message, err := s.p.launchAtLogin.Set(on)
	if err != nil {
		return errors.New(message)
	}
	return nil
}

func (s *settingsStore) saveMenuBarIconStyle(raw json.RawMessage) error {
	var style string
	if err := json.Unmarshal(raw, &style); err != nil {
		return err
	}
	if s.p.sharedConfig == nil {
		return errNoSharedConfig
	}
	if err := s.p.sharedConfig.Update(func(doc portable.Document) {
		doc.SetValue(style, "settings", "menuBarIconStyle")
	}); err != nil {
		return err
	}
	if s.p.menubar != nil {
		_ = s.p.menubar.SetIconStyle(style)
	}
	return nil
}

func orDefaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

package services

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// servePreferences lists the declared settings of every extension after the preferences check, and changes one after
// the preferences.write check, from a palette run or a conversation program.
func (h *Handler) servePreferences(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if h.deps.Preferences == nil {
		return nil, unavailable("preferences are not available in this build")
	}
	if err := h.require(api.CapabilityPreferences); err != nil {
		return nil, err
	}
	switch call.Op {
	case protocol.OpList:
		var args protocol.QueryArgs
		if err := decodeArgs(call.Args, &args); err != nil {
			return nil, err
		}
		return h.deps.Preferences.List(ctx, args.Query)
	case protocol.OpSet:
		return h.setPreference(ctx, call)
	}
	return nil, invalid("preferences has no operation %q", call.Op)
}

func (h *Handler) setPreference(ctx context.Context, call protocol.ServiceCall) (api.PreferenceRecord, error) {
	if err := h.require(api.CapabilityPreferencesWrite); err != nil {
		return api.PreferenceRecord{}, err
	}
	if err := requireRun(protocol.RequestMethodOf(ctx), "changing a preference"); err != nil {
		return api.PreferenceRecord{}, err
	}
	var args protocol.PreferenceArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return api.PreferenceRecord{}, err
	}
	if args.ExtensionID == "" || args.SettingID == "" {
		return api.PreferenceRecord{}, invalid("a preference change needs the extension id and the setting id")
	}
	if len(args.Value) == 0 {
		args.Value = json.RawMessage("null")
	}
	return h.deps.Preferences.Set(ctx, args.ExtensionID, args.SettingID, args.Value, args.Expect)
}

// SaveSettingForUser stores value as the declared setting id, as the user's own change on a settings page. It checks
// the value as every write does but not settings.write: that declaration is the extension's own right to write, and
// the user needs none. Service calls never reach it.
func (h *Handler) SaveSettingForUser(id string, value []byte) error {
	h.settingsMu.Lock()
	defer h.settingsMu.Unlock()
	definition, found := h.declaredSetting(id)
	if !found {
		return notFound("the extension declares no setting %q", id)
	}
	return h.saveSetting(definition, value)
}

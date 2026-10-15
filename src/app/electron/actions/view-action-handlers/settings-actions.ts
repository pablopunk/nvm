export type SettingActionHandlerDeps = {
  settingDefinition: (id: string) => any;
  toggledSettingValue: (definition: any, current: any) => any;
  getSetting: (id: any) => any;
  setSetting: (id: any, value: any) => { ok: boolean; message?: string };
  settingItemPatch: (definition: any) => any;
  setShortcutSetting: (
    settingId: any,
    accelerator: any,
  ) => { ok: boolean; message?: string };
};

export function createSettingActionHandlers(deps: SettingActionHandlerDeps) {
  const {
    settingDefinition,
    toggledSettingValue,
    getSetting,
    setSetting,
    settingItemPatch,
    setShortcutSetting,
  } = deps;

  function handleToggleSetting(action) {
    const definition = settingDefinition(action.settingId);
    if (!definition || definition.type !== 'boolean')
      return { toast: { message: 'Setting not found', tone: 'error' } };
    const result = setSetting(
      definition.id,
      toggledSettingValue(definition, getSetting(definition.id)),
    );
    if (!result.ok)
      return { toast: { message: result.message, tone: 'error' } };
    return { patch: { items: [settingItemPatch(definition)] } };
  }

  function handleSetSettingShortcut(action) {
    const result = setShortcutSetting(
      action.settingId,
      action.accelerator || action.shortcut,
    );
    return {
      patch: {
        items: result.ok
          ? [settingItemPatch(settingDefinition(action.settingId))]
          : [],
      },
      toast: {
        message: result.message,
        tone: result.ok ? 'default' : 'error',
      },
      ok: result.ok,
    };
  }

  return { handleToggleSetting, handleSetSettingShortcut };
}

export type SettingActionHandlerDeps = {
  settingDefinition: (id: string) => any;
  toggledSettingValue: (definition: any, current: any) => any;
  getSetting: (id: any) => any;
  setSetting: (
    id: any,
    value: any,
  ) => Promise<{ ok: boolean; message?: string }>;
  settingItemPatch: (definition: any) => any;
  setShortcutSetting: (
    settingId: any,
    accelerator: any,
  ) => Promise<{ ok: boolean; message?: string }>;
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

  async function handleToggleSetting(action) {
    const definition = settingDefinition(action.settingId);
    if (
      !definition ||
      (definition.type !== 'boolean' && definition.type !== 'choice')
    )
      return { toast: { message: 'Setting not found', tone: 'error' } };
    const result = await setSetting(
      definition.id,
      toggledSettingValue(definition, getSetting(definition.id)),
    );
    if (!result.ok)
      return { toast: { message: result.message, tone: 'error' } };
    return { patch: { items: [settingItemPatch(definition)] } };
  }

  async function handleSetSettingShortcut(action) {
    const result = await setShortcutSetting(
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

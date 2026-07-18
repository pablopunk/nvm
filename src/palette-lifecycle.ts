type ClearNullableState = (value: null) => void;

interface TransientPaletteStateSetters {
  setOptionsFor: ClearNullableState;
  setExtensionItemOptionsFor: ClearNullableState;
  setConfirmRemoveFor: ClearNullableState;
  setPreviewFor: ClearNullableState;
  setChildQuery: (value: string) => void;
  setShortcutFor: ClearNullableState;
  setRecordedShortcut: (value: string) => void;
  setShortcutManagerOpen: (value: boolean) => void;
  setShortcutOptionsFor: ClearNullableState;
  setAliasFor: ClearNullableState;
  setConfirmViewActionFor: ClearNullableState;
  setActionSubmenuFor: ClearNullableState;
}

function resetTransientPaletteState(setters: TransientPaletteStateSetters) {
  setters.setOptionsFor(null);
  setters.setExtensionItemOptionsFor(null);
  setters.setConfirmRemoveFor(null);
  setters.setPreviewFor(null);
  setters.setChildQuery('');
  setters.setShortcutFor(null);
  setters.setRecordedShortcut('');
  setters.setShortcutManagerOpen(false);
  setters.setShortcutOptionsFor(null);
  setters.setAliasFor(null);
  setters.setConfirmViewActionFor(null);
  setters.setActionSubmenuFor(null);
}

export type { TransientPaletteStateSetters };
export { resetTransientPaletteState };

import type { ReactNode } from 'react';
import type {
  ActionPanelVisibility,
  ExtensionAccessoryTone,
  ExtensionEditorFormat,
  ExtensionFormFieldType,
  ExtensionFormOption,
  ExtensionFormValue,
  ExtensionImage,
  ExtensionView,
  ExtensionWebviewPermission,
  ForegroundColor,
  PatchMode,
  ExtensionPermission as PublicExtensionPermission,
  ViewPresentation,
  ViewSize,
} from './resources/nevermind-extension-api';

export type ExtensionPermission = PublicExtensionPermission;

export type ActionDismissBehavior = 'manual' | 'immediate' | 'after-success';
export type ActionLoadingBehavior = 'view' | 'none';
export type ActionExecutionLocation = 'main' | 'renderer';

export type CommandActionDefinition = {
  description: string;
  dismiss: ActionDismissBehavior;
  loading: ActionLoadingBehavior;
  execute: ActionExecutionLocation;
  inline?: boolean;
};

export const ACTION_DEFINITIONS = {
  openPath: {
    description: 'Open with the default app',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  revealPath: {
    description: 'Reveal in file manager',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  quickLook: {
    description: 'Preview this file',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  openWith: {
    description: 'Open with another app',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  openUrl: {
    description: 'Open URL',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  copyText: {
    description: 'Copy text to the clipboard',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  copyImage: {
    description: 'Copy image to the clipboard',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  pasteText: {
    description: 'Paste into the frontmost app',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  pasteClipboard: {
    description: 'Paste into the frontmost app',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  typeText: {
    description: 'Type into the frontmost app',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  trash: {
    description: 'Move to Trash',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  rootView: {
    description: 'Open top-level view',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  pushView: {
    description: 'Open nested view',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  replaceView: {
    description: 'Open nested view',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  popView: {
    description: 'Go back',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  previewClipboardItem: {
    description: 'Preview clipboard item',
    dismiss: 'manual',
    loading: 'none',
    execute: 'renderer',
  },
  removeClipboardHistory: {
    description: 'Remove clipboard history entries',
    dismiss: 'manual',
    loading: 'none',
    execute: 'main',
  },
  runExtensionAction: {
    description: 'Run action',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  shellExec: {
    description: 'Run system command',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  shellScript: {
    description: 'Run system command',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  checkForUpdates: {
    description: 'Check for updates',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  downloadUpdate: {
    description: 'Download update',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  installUpdate: {
    description: 'Install update',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  lockScreen: {
    description: 'Lock screen',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  sleepSystem: {
    description: 'Sleep system',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  restartSystem: {
    description: 'Restart system',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  quitApp: {
    description: 'Quit app',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  forceQuitApp: {
    description: 'Force quit a running application',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  openSystemSettings: {
    description: 'Open system settings',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  openKeyboardSettings: {
    description: 'Open keyboard settings',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  toggleSetting: {
    description: 'Change setting',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
    inline: true,
  },
  recordShortcut: {
    description: 'Record shortcut',
    dismiss: 'manual',
    loading: 'none',
    execute: 'renderer',
  },
  setActionShortcut: {
    description: 'Set shortcut',
    dismiss: 'manual',
    loading: 'none',
    execute: 'main',
    inline: true,
  },
  setSettingShortcut: {
    description: 'Set shortcut setting',
    dismiss: 'manual',
    loading: 'none',
    execute: 'main',
    inline: true,
  },
  removeShortcut: {
    description: 'Remove shortcut',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
    inline: true,
  },
  setActionAlias: {
    description: 'Set alias',
    dismiss: 'manual',
    loading: 'none',
    execute: 'main',
    inline: true,
  },
  removeActionAlias: {
    description: 'Remove alias',
    dismiss: 'manual',
    loading: 'none',
    execute: 'main',
    inline: true,
  },
  duplicateCreatedAction: {
    description: 'Duplicate action',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  removeCreatedAction: {
    description: 'Remove action',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  submitExtensionPr: {
    description: 'Submit as PR',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  clearActionOverride: {
    description: 'Restore original action',
    dismiss: 'manual',
    loading: 'none',
    execute: 'main',
    inline: true,
  },
  nativeAction: {
    description: 'Run command',
    dismiss: 'manual',
    loading: 'none',
    execute: 'main',
  },
  promptAction: {
    description: 'Prompt for input',
    dismiss: 'manual',
    loading: 'none',
    execute: 'main',
  },
  renameExtensionPrompt: {
    description: 'Prompt for extension rename',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  renameExtension: {
    description: 'Rename extension',
    dismiss: 'manual',
    loading: 'view',
    execute: 'main',
  },
  createWindow: {
    description: 'Open extension window',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  showWindow: {
    description: 'Show extension window',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  hideWindow: {
    description: 'Hide extension window',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  toggleWindow: {
    description: 'Toggle extension window',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  closeWindow: {
    description: 'Close extension window',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  runExtensionRegisteredAction: {
    description: 'Run persistent extension action',
    dismiss: 'immediate',
    loading: 'none',
    execute: 'main',
  },
  setSearchQuery: {
    description: 'Replace the root search query',
    dismiss: 'manual',
    loading: 'none',
    execute: 'renderer',
  },
} as const satisfies Record<string, CommandActionDefinition>;

export type CommandActionType = keyof typeof ACTION_DEFINITIONS;

export type CommandApp = { name?: string; path?: string };

export type CommandAction = {
  type: CommandActionType;
  title: string;
  subtitle?: string;
  description?: string;
  path?: string;
  paths?: string[];
  app?: CommandApp;
  appPath?: string;
  url?: string;
  text?: string;
  html?: string;
  keepPaletteOpen?: boolean;
  restoreClipboard?: boolean;
  plainText?: boolean;
  concealed?: boolean;
  restoreDelayMs?: number;
  delayMs?: number;
  imageDataUrl?: string;
  imagePath?: string;
  view?: CommandView;
  handlerId?: string;
  shortcut?: string;
  shortcutScope?: 'local' | 'global';
  nativeAction?: unknown;
  settingId?: string;
  action?: unknown;
  actionId?: string;
  extensionId?: string;
  commandId?: string;
  registeredActionId?: string;
  targetAction?: unknown;
  windowId?: string;
  windowOptions?: Record<string, unknown>;
  alias?: string;
  accelerator?: string;
  clipboardType?: string;
  clipboardHistoryRange?:
    | 'item'
    | 'ids'
    | 'last-hour'
    | 'last-day'
    | 'older-than'
    | 'all';
  clipboardHistoryItemId?: string;
  clipboardHistoryItemIds?: string[];
  content?: unknown;
  videoUrl?: string;
  filePath?: string;
  thumbnailUrl?: string;
  aiChatId?: string;
  query?: string;
  select?: boolean;
  extensionFile?: string;
  command?: string;
  args?: string[];
  script?: string;
  options?: Record<string, unknown>;
  formValues?: Record<string, CommandFormValue>;
  editorContent?: string;
  fields?: CommandFormField[];
  promptMessage?: string;
  submitTitle?: string;
  selectedItemId?: string;
  value?: string;
  submenu?: CommandActionPanel;
  lazySubmenu?: boolean;
  style?: 'regular' | 'destructive';
  requiresConfirmation?: boolean;
  confirmMessage?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  dismissAfterRun?: 'auto';
  executionId?: string;
};

export type CommandActionSection = {
  title?: string;
  actions: CommandAction[];
  lazyActions?: CommandAction[];
  isLoading?: boolean;
};

export type CommandActionPanel = {
  title?: string;
  sections: CommandActionSection[];
};

export type CommandAccessoryTone = ExtensionAccessoryTone;
export type CommandItemAccessory = {
  text?: string;
  icon?: string | ReactNode;
  tone?: CommandAccessoryTone;
  tooltip?: string;
};
export type CommandImage = ExtensionImage;
export type CommandMetadataItem =
  | { type?: 'text'; label: string; value: string; copyable?: boolean }
  | { type: 'link'; label: string; value: string; url: string }
  | { type: 'tag'; label?: string; value: string; tone?: CommandAccessoryTone }
  | { type: 'separator' };
export type CommandDetail = {
  title?: string;
  subtitle?: string;
  markdown?: string;
  metadata?: CommandMetadataItem[];
  image?: CommandImage;
  actions?: CommandAction[];
};
export type CommandItemForeground = ForegroundColor;
export type CommandItemAppearance = { foreground?: CommandItemForeground };

export type CommandItemPatch = Partial<Omit<CommandItem, 'id'>> & {
  id: string;
};
export type CommandFormValue = ExtensionFormValue;
export type CommandFormFieldType = ExtensionFormFieldType;
export type CommandFormOption = ExtensionFormOption;
export type CommandFormField = {
  id: string;
  label?: string;
  type?: CommandFormFieldType;
  value?: CommandFormValue;
  placeholder?: string;
  required?: boolean;
  options?: CommandFormOption[];
  description?: string;
  error?: string;
  rows?: number;
  extensions?: string[];
  filterName?: string;
  buttonLabel?: string;
  defaultPath?: string;
  canCreateDirectories?: boolean;
};

export type CommandItem = {
  id: string;
  title: string;
  subtitle?: string;
  accessories?: CommandItemAccessory[];
  shortcut?: string;
  keywords?: string[];
  text?: string;
  icon?: string;
  image?: CommandImage;
  video?: string;
  videoUrl?: string;
  path?: string;
  filePath?: string;
  fileUrl?: string;
  primaryAction?: CommandAction;
  /** Persistent root/search action represented by this view item; enables aliases/shortcuts/options for durable actions shown inside views. */
  persistentAction?: unknown;
  actions?: CommandAction[];
  actionPanel?: CommandActionPanel;
  actionPanelVisibility?: ActionPanelVisibility;
  appearance?: CommandItemAppearance;
  /** Visible context only; disabled items are skipped by keyboard selection. */
  disabled?: boolean;
  className?: string;
  detail?: CommandDetail;
};

export type CommandItemSection = {
  title?: string;
  subtitle?: string;
  items: CommandItem[];
};

export type CommandViewPatch = {
  items?: CommandItemPatch[];
  mode?: PatchMode;
  removeItemIds?: string[];
  isLoading?: boolean;
  selectedItemId?: string;
};

export type CommandView = {
  id?: string;
  type:
    | 'list'
    | 'grid'
    | 'preview'
    | 'chat'
    | 'form'
    | 'editor'
    | 'progress'
    | 'webview'
    | 'camera';
  title: string;
  size?: ViewSize;
  image?: CommandImage;
  video?: string;
  videoUrl?: string;
  deviceId?: string;
  showDeviceSwitcher?: boolean;
  muted?: boolean;
  controls?: boolean;
  aiChat?: boolean;
  chatId?: string;
  initialPrompt?: string;
  subtitle?: string;
  content?: string;
  placeholder?: string;
  format?: ExtensionEditorFormat;
  language?: string;
  readOnly?: boolean;
  html?: string;
  webviewPermissions?: ExtensionWebviewPermission[];
  items?: CommandItem[];
  sections?: CommandItemSection[];
  isLoading?: boolean;
  emptyView?: { title?: string; subtitle?: string };
  detail?: { placement?: 'side' | 'bottom'; visible?: boolean };
  searchBarPlaceholder?: string;
  presentation?: ViewPresentation;
  selectedItemId?: string;
  onSelectionChange?: CommandAction;
  pagination?: {
    hasMore?: boolean;
    pageSize?: number;
    onLoadMore?: CommandAction;
  };
  searchAccessory?: {
    id?: string;
    tooltip?: string;
    value?: string;
    items: { title: string; value: string }[];
    onChange?: CommandAction;
  };
  refresh?: {
    id?: string;
    intervalMs?: number;
    mode?: CommandViewPatch['mode'];
    immediate?: boolean;
  };
  messages?: { role: 'user' | 'assistant' | 'system'; content: string }[];
  fields?: CommandFormField[];
  submitAction?: CommandAction;
  steps?: { title: string; status?: string }[];
  value?: number;
  total?: number;
  status?: string;
  actions?: CommandAction[];
  actionPanel?: CommandActionPanel;
  actionPanelVisibility?: ActionPanelVisibility;
  layout?: NonNullable<ExtensionView['layout']>;
  aspectRatio?: NonNullable<ExtensionView['aspectRatio']>;
  columns?: NonNullable<ExtensionView['columns']>;
};

export type RowModel = {
  value: string;
  icon: ReactNode;
  title: string;
  subtitle?: string;
  shortcut?: string;
  extras?: string[];
  className?: string;
  onSelect: () => void;
};

export type CustomizableCommandAction = {
  kind?: string;
  customizable?: boolean;
};

const CUSTOMIZABLE_ACTION_KINDS = new Set([
  'app',
  'builtin',
  'clipboard-history',
  'extension-action',
]);

export function canCustomizeCommandAction(
  action: CustomizableCommandAction | null | undefined,
) {
  return (
    Boolean(action?.customizable) ||
    CUSTOMIZABLE_ACTION_KINDS.has(String(action?.kind || ''))
  );
}

export function actionPanelFromActions(
  actions?: CommandAction[],
  title?: string,
): CommandActionPanel | undefined {
  if (!actions?.length) return;
  return { title, sections: [{ actions }] };
}

export function actionsFromPanel(
  panel?: CommandActionPanel,
  fallbackActions: CommandAction[] = [],
) {
  return (
    panel?.sections.flatMap((section) => section.actions) || fallbackActions
  );
}

export function actionDefinition(
  action: Pick<CommandAction, 'type'> | null | undefined,
) {
  return action?.type ? ACTION_DEFINITIONS[action.type] : undefined;
}

export function actionDescription(action: CommandAction) {
  if (action.subtitle || action.description)
    return action.subtitle || action.description;
  if (action.type === 'quickLook' || action.type === 'revealPath')
    return (
      action.title || actionDefinition(action)?.description || 'Run action'
    );
  if (action.type === 'openWith')
    return action.app?.name
      ? `Open with ${action.app.name}`
      : actionDefinition(action)?.description || 'Run action';
  return actionDefinition(action)?.description || 'Run action';
}

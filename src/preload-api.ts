// biome-ignore-all lint/style/useConsistentTypeDefinitions: Public preload contracts retain the established object type-alias style.
import type {
  CommandAction,
  CommandItemAppearance,
  CommandView,
  CommandViewPatch,
} from './model';

export type RootAction = {
  id: string;
  kind:
    | 'open-url'
    | 'web-search'
    | 'app'
    | 'clipboard'
    | 'clipboard-history'
    | 'keyboard-shortcuts'
    | 'app-settings'
    | 'check-for-updates'
    | 'download-update'
    | 'install-update'
    | 'file'
    | 'ai-placeholder'
    | 'ai-chat'
    | 'ai-chats'
    | 'ai-tweak-extension'
    | 'remove-ai-chat'
    | 'builtin'
    | 'calculate'
    | 'extension-root-item'
    | 'extension-action';
  title: string;
  subtitle: string;
  icon: string;
  score: number;
  iconUrl?: string | null;
  url?: string;
  query?: string;
  result?: string;
  text?: string;
  clipboardType?: 'text' | 'image' | 'video';
  imageDataUrl?: string;
  videoUrl?: string;
  thumbnailUrl?: string;
  filePath?: string;
  defaultActionId?: string;
  isOverridden?: boolean;
  overrideSummary?: string;
  app?: { name?: string; path?: string };
  extensionId?: string;
  commandId?: string;
  aiChatId?: string;
  extensionFile?: string;
  removable?: boolean;
  background?: boolean;
  dismissAfterRun?: 'auto';
  actionPanel?: CommandView['actionPanel'];
  shortcut?: string;
  userAliases?: string[];
  appearance?: CommandItemAppearance;
  executionId?: string;
};

export type SearchOptions = {
  generation: number;
  clipboardOnly?: boolean;
};

export type SearchSnapshot<T = RootAction> = {
  generation: number;
  revision: number;
  results: T[];
  complete: boolean;
};

export type SaveResult = {
  ok: boolean;
  message: string;
};

export type ShortcutRecord = {
  actionId: string;
  accelerator: string;
  action: RootAction;
};

export type PaletteMode = 'default' | 'ai-chat' | 'stacked' | 'preview';

export type ViewActionResult = {
  view?: CommandView;
  patch?: CommandViewPatch;
  navigation?: 'root' | 'push' | 'replace' | 'pop';
  toast?: { message: string; tone?: 'default' | 'info' | 'success' | 'error' };
  skipped?: boolean;
};

export type OpenActionViewPayload = {
  view?: CommandView;
  revealWhenReady?: boolean;
  asSibling?: boolean;
};

export type AiChatEvent = {
  type: string;
  text?: string;
  message?: string;
  name?: string;
  chatId?: string;
  label?: string;
  data?: unknown;
};

export type ViewHydratePayload = {
  viewId: string;
  items?: CommandView['items'];
  isLoading?: boolean;
  error?: { message: string };
  retry?: boolean;
};

export type NevermindApi = {
  search: (
    query: string,
    options: SearchOptions,
  ) => Promise<SearchSnapshot<RootAction>>;
  cancelSearch: (generation: number) => void;
  onSearchUpdate: (
    callback: (snapshot: SearchSnapshot<RootAction>) => void,
  ) => () => void;
  execute: (action: RootAction) => Promise<{ view?: CommandView }>;
  runViewAction: (action: CommandAction) => Promise<ViewActionResult>;
  refreshView: (input: {
    id: string;
    viewId?: string;
  }) => Promise<ViewActionResult>;
  pickFormFieldPaths: (input: {
    type?: 'file' | 'files' | 'folder';
    title?: string;
    buttonLabel?: string;
    defaultPath?: string;
    extensions?: string[];
    filterName?: string;
    canCreateDirectories?: boolean;
  }) => Promise<{ canceled: boolean; paths: string[] }>;
  startFileDrag: (filePath: string) => void;
  sendAiMessage: (message: string, chatId?: string) => Promise<void>;
  aiChatExited: (chatId?: string) => Promise<void>;
  abortAiChat: (chatId?: string) => Promise<void>;
  resetAiChat: (chatId?: string) => Promise<void>;
  setAlias: (action: RootAction, alias: string) => Promise<SaveResult>;
  removeAlias: (action: RootAction, alias: string) => Promise<SaveResult>;
  setShortcut: (action: RootAction, shortcut: string) => Promise<SaveResult>;
  setPaletteHotkey: (
    accelerator: string,
  ) => Promise<SaveResult & { spotlightConflict?: boolean }>;
  getSetting: (id: string) => Promise<string | boolean | undefined>;
  openSystemKeyboardSettings: () => Promise<{ ok: boolean }>;
  getShortcuts: () => Promise<ShortcutRecord[]>;
  removeShortcut: (actionId: string) => Promise<SaveResult>;
  suspendShortcuts: () => Promise<void>;
  resumeShortcuts: () => Promise<void>;
  setOverride: (action: RootAction, instruction: string) => Promise<SaveResult>;
  clearOverride: (action: RootAction) => Promise<SaveResult>;
  duplicateCreatedAction: (
    action: RootAction,
  ) => Promise<SaveResult & { action?: RootAction }>;
  removeCreatedAction: (action: RootAction) => Promise<SaveResult>;
  tweakExtension: (input: {
    extensionFile: string;
    title?: string;
    prompt?: string;
  }) => Promise<ViewActionResult>;
  startBuilderChat: (input: {
    prompt: string;
    title?: string;
  }) => Promise<ViewActionResult>;
  getAppIcon: (appPath: string) => Promise<string | null>;
  getRunningAppPaths: (appPaths: string[]) => Promise<string[]>;
  setPaletteMode: (mode: PaletteMode) => Promise<void>;
  hide: () => Promise<void>;
  testInvoke: () => Promise<unknown>;
  testStageExtensionProposal: (
    filename: string,
    source: string,
  ) => Promise<{ draftFile: string }>;
  testRunJob: (id: string) => Promise<{ found: boolean }>;
  testFailNextExtensionActivation: (
    phase: 'state-persist' | 'after-persist' | 'runtime-commit',
  ) => Promise<{ armed: boolean }>;
  testIsActionShortcutRegistered: (
    accelerator: string,
  ) => Promise<{ registered: boolean }>;
  shortcutReady: () => Promise<void>;
  requestCameraAccess: () => Promise<{ ok: boolean; status: string }>;
  log: (
    level: 'debug' | 'info' | 'warn' | 'error',
    message: string,
    data?: unknown,
  ) => Promise<void>;
  getNevermindAuthStatus: () => Promise<{ authed: boolean; email?: string }>;
  getNevermindDebugStatus: () => Promise<{
    client: { environment: string; baseUrl: string };
    active: { environment: string; baseUrl: string };
    backend: { environment: string; version: string } | null;
  }>;
  getGhStatus: () => Promise<{ installed: boolean; authed: boolean }>;
  signInToNevermind: () => Promise<{
    ok: boolean;
    email?: string;
    error?: string;
  }>;
  onNevermindAuthChanged: (
    callback: (status: { authed: boolean; email?: string }) => void,
  ) => () => void;
  onShown: (callback: () => void) => () => void;
  onShortcutShown: (callback: () => void) => () => void;
  onHidden: (callback: () => void) => () => void;
  onAppsIndexed: (callback: (count: number) => void) => () => void;
  onRunningAppPathsChanged: (callback: () => void) => () => void;
  onClipboardChanged: (callback: () => void) => () => void;
  onRootItemsChanged: (callback: () => void) => () => void;
  onOpenActionView: (
    callback: (payload?: OpenActionViewPayload) => void,
  ) => () => void;
  onAiChatEvent: (callback: (event: AiChatEvent) => void) => () => void;
  getExtensionWindowState: () => Promise<{
    id: string;
    view: CommandView;
    options?: Record<string, unknown>;
  } | null>;
  closeExtensionWindow: () => Promise<void>;
  saveExtensionDraft: (
    ref: string,
    content: string,
  ) => Promise<{ ok: boolean; error?: string }>;
  onExtensionWindowView: (
    callback: (payload: {
      id: string;
      view: CommandView;
      options?: Record<string, unknown>;
    }) => void,
  ) => () => void;
  onViewPatch: (
    callback: (payload: { viewId?: string; patch: CommandViewPatch }) => void,
  ) => () => void;
  onViewHydrate: (
    callback: (payload: ViewHydratePayload) => void,
  ) => () => void;
  retryViewLoader: (viewId: string) => Promise<void>;
};

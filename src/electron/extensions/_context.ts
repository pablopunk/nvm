export const extensionContext: {
  userState: Record<string, any>;
  fileIndex: any[];
  clipboardService: { createClipboardExtension: () => any } | null;
  nevermindAi: any;
  activeAiChatId: string | undefined;
  draftAiChats: Map<string, Record<string, any>>;
  jobRegistry: {
    snapshot: () => any[];
    run: (id: string, reason: string) => Promise<unknown>;
    setEnabled: (id: string, enabled: boolean) => void;
    clearError: (id: string) => void;
  };
  appIndexService: { get: () => any[] };
  runningAppStatus: { refresh: (reason: string) => Promise<Set<string>> };
  FILE_RESULT_LIMIT: number;
  usageBoost: (actionId: any) => number;
  recentBoost: (actionId: any) => number;
  rankAction: (action: any, query: any) => boolean;
  actionAliases: (actionId: any) => string[];
  commandFromItem: (item: any) => any;
  createExtensionContext: (
    extension: any,
    command: any,
    launchContext?: any,
  ) => any;
  scheduleSaveState: () => void;
  saveUserState: () => Promise<void>;
  invalidateExtensionRootItems: () => void;
  broadcastAuthChanged: (status: { authed: boolean; email?: string }) => void;
  activeNevermindBaseUrl: string | null;
  setActiveNevermindBaseUrl: (value: string | null) => void;
  getPaletteHotkey: () => any;
  extensionShortcutRecords: () => any[];
  patchKeyboardShortcutsView: () => void;
  patchOpenView: (viewId: string, patch: any) => void;
  aiChatsView: () => any;
  aiChatView: (item: any, options?: any) => any;
  updatesStateSnapshot: () => any;
  checkForUpdatesView: () => any;
  compatibilityPromptAction: () => any;
  updatePromptAction: () => any;
  settingsItems: () => any[];
  buildRecordShortcutAction: (input: any, options: any) => any;
  buildRemoveShortcutAction: (input: any, options: any) => any;
  paletteWindow: {
    win?: {
      webContents: { send: (channel: string, ...args: any[]) => void };
    } | null;
  };
} = {} as any;

export function initExtensionContext(ctx: typeof extensionContext): void {
  Object.assign(extensionContext, ctx);
}

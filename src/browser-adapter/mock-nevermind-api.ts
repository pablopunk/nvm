import type { CommandAction, CommandView } from '../model';
import type {
  DesignTokenState,
  NevermindApi,
  RootAction,
  SearchSnapshot,
} from '../preload-api';

const fixturesView: CommandView = {
  id: 'browser-fixtures',
  type: 'list',
  title: 'Fixtures',
  subtitle: 'Real Nevermind list and navigation components',
  searchBarPlaceholder: 'Search fixtures',
  sections: [
    {
      title: 'Extension views',
      items: [
        {
          id: 'fixture-list',
          title: 'List view',
          subtitle: 'Rows, accessories, and item actions',
          icon: 'list',
          accessories: [{ text: '3 items' }],
          primaryAction: {
            type: 'pushView',
            title: 'Open List Fixture',
            view: {
              id: 'browser-list-fixture',
              type: 'list',
              title: 'List Fixture',
              searchBarPlaceholder: 'Filter fixture rows',
              items: [
                {
                  id: 'alpha',
                  title: 'Alpha project',
                  subtitle: 'Ready for review',
                  icon: 'folder',
                  accessories: [{ text: 'Updated now' }],
                  actions: [toastAction('Mark Alpha complete')],
                },
                {
                  id: 'beta',
                  title: 'Beta project',
                  subtitle: 'Work in progress',
                  icon: 'folder',
                  accessories: [{ text: '2 tasks' }],
                  actions: [toastAction('Open Beta actions')],
                },
                {
                  id: 'gamma',
                  title: 'Gamma project',
                  subtitle: 'Planning',
                  icon: 'folder',
                  actions: [toastAction('Open Gamma actions')],
                },
              ],
              actions: [toastAction('Create project')],
            },
          },
        },
        {
          id: 'fixture-grid',
          title: 'Grid view',
          subtitle: 'Tiles rendered by ExtensionViewRenderer',
          icon: 'grid-2x2',
          primaryAction: {
            type: 'pushView',
            title: 'Open Grid Fixture',
            view: {
              id: 'browser-grid-fixture',
              type: 'grid',
              title: 'Grid Fixture',
              columns: 3,
              items: [
                {
                  id: 'one',
                  title: 'One',
                  subtitle: 'First tile',
                  icon: 'star',
                },
                {
                  id: 'two',
                  title: 'Two',
                  subtitle: 'Second tile',
                  icon: 'heart',
                },
                {
                  id: 'three',
                  title: 'Three',
                  subtitle: 'Third tile',
                  icon: 'zap',
                },
              ],
            },
          },
        },
        {
          id: 'fixture-detail',
          title: 'Detail view',
          subtitle: 'Production preview renderer',
          icon: 'panel-right',
          primaryAction: {
            type: 'pushView',
            title: 'Open Detail Fixture',
            view: {
              id: 'browser-detail-fixture',
              type: 'preview',
              title: 'Detail Fixture',
              content:
                '# Real renderer\n\nThis content is rendered by Nevermind’s production preview view.',
              actions: [toastAction('Confirm detail')],
            },
          },
        },
      ],
    },
  ],
  actions: [toastAction('Refresh fixtures')],
};

function toastAction(title: string): CommandAction {
  return { type: 'nativeAction', title, nativeAction: { mock: 'toast' } };
}

const rootActions: RootAction[] = [
  {
    id: 'browser-fixtures-root',
    kind: 'extension-root-item',
    title: 'Fixtures',
    subtitle: 'Real Nevermind views running in the browser',
    icon: 'wrench',
    score: 100,
    actionPanel: {
      title: 'Fixtures actions',
      sections: [{ actions: [toastAction('Open Fixtures')] }],
    },
  },
  {
    id: 'browser-search-root',
    kind: 'builtin',
    title: 'Search Preview',
    subtitle: 'Canonical root result row and actions',
    icon: 'search',
    score: 90,
    actionPanel: {
      sections: [{ actions: [toastAction('Run search action')] }],
    },
  },
  {
    id: 'browser-settings-root',
    kind: 'app-settings',
    title: 'Settings',
    subtitle: 'Configure Nevermind',
    icon: 'settings',
    score: 80,
  },
  {
    id: 'browser-updates-root',
    kind: 'check-for-updates',
    title: 'Check for Updates',
    subtitle: 'Current version is ready',
    icon: 'refresh-cw',
    score: 70,
  },
];

export function createMockBrowserNevermindApi(
  initialTokens: DesignTokenState,
): NevermindApi {
  let tokens = initialTokens;
  const eventUnsubscribe = () => () => {};
  const search = async (
    query: string,
    options: { generation: number },
  ): Promise<SearchSnapshot> => {
    const normalized = query.trim().toLowerCase();
    const results = normalized
      ? rootActions.filter((action) =>
          `${action.title} ${action.subtitle}`
            .toLowerCase()
            .includes(normalized),
        )
      : rootActions;
    return {
      generation: options.generation,
      revision: 0,
      results,
      complete: true,
    };
  };
  const api = {
    getDesignTokens: async () => tokens,
    openDesignTokenEditor: async () => tokens,
    onOpenDesignTokenEditor: eventUnsubscribe,
    setDesignTokens: async (overrides) => {
      tokens = {
        ...tokens,
        overrides,
        values: { ...tokens.defaults, ...overrides },
      };
      return tokens;
    },
    resetDesignTokens: async () => {
      tokens = { ...tokens, overrides: {}, values: { ...tokens.defaults } };
      return tokens;
    },
    closeDesignTokenEditor: async () => {},
    search,
    cancelSearch: () => {},
    onSearchUpdate: eventUnsubscribe,
    execute: async (action: RootAction) => {
      if (action.id === 'browser-fixtures-root') return { view: fixturesView };
      return { toast: { message: `${action.title} ran in browser preview` } };
    },
    runViewAction: async (action: CommandAction) => {
      if (action.type === 'nativeAction') {
        const rootAction = action.nativeAction as RootAction | undefined;
        if (rootAction) return api.execute(rootAction);
      }
      if (action.type === 'pushView' && action.view)
        return { view: action.view, navigation: 'push' as const };
      if (action.type === 'popView') return { navigation: 'pop' as const };
      return { toast: { message: action.title } };
    },
    refreshView: async () => ({}),
    pickFormFieldPaths: async () => ({ canceled: true, paths: [] }),
    startFileDrag: () => {},
    sendAiMessage: async () => {},
    aiChatExited: async () => {},
    abortAiChat: async () => {},
    resetAiChat: async () => {},
    setAlias: async () => ({ ok: true, message: 'Alias saved' }),
    removeAlias: async () => ({ ok: true, message: 'Alias removed' }),
    setShortcut: async () => ({ ok: true, message: 'Shortcut saved' }),
    setPaletteHotkey: async () => ({ ok: true, message: 'Shortcut saved' }),
    getSetting: async () => null,
    openSystemKeyboardSettings: async () => ({ ok: false }),
    getShortcuts: async () => [],
    removeShortcut: async () => ({ ok: true, message: 'Shortcut removed' }),
    suspendShortcuts: async () => {},
    resumeShortcuts: async () => {},
    getAppIcon: async () => null,
    getRunningAppPaths: async () => [],
    setPaletteMode: async () => {},
    hide: async () => {},
    onNevermindAuthChanged: eventUnsubscribe,
    getNevermindAuthStatus: async () => ({ authed: false }),
    getGhStatus: async () => ({ installed: false, authed: false }),
    onShown: eventUnsubscribe,
    onShortcutShown: eventUnsubscribe,
    onHidden: eventUnsubscribe,
    onAppsIndexed: eventUnsubscribe,
    onRunningAppPathsChanged: eventUnsubscribe,
    onClipboardChanged: eventUnsubscribe,
    onRootItemsChanged: eventUnsubscribe,
    onOpenActionView: eventUnsubscribe,
    onAiChatEvent: eventUnsubscribe,
    getExtensionWindowState: async () => undefined,
    closeExtensionWindow: async () => {},
    onExtensionWindowView: eventUnsubscribe,
    onViewPatch: eventUnsubscribe,
    onViewHydrate: eventUnsubscribe,
    onOpenAiChat: eventUnsubscribe,
    onOpenBuilderPreview: eventUnsubscribe,
    shortcutReady: async () => {},
    log: async () => {},
  };
  return new Proxy(api as unknown as NevermindApi, {
    get(target, property, receiver) {
      if (property in target) return Reflect.get(target, property, receiver);
      return async () => undefined;
    },
  });
}

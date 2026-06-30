import { Command } from 'cmdk';
import {
  Clipboard,
  Copy,
  Keyboard,
  Pencil,
  RotateCcw,
  Search,
  Sparkles,
  Tag,
  Trash2,
  Wand2,
  Zap,
} from 'lucide-react';
import {
  type MouseEvent,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { ActionPanel } from './action-panel';
import {
  type CommandIconName,
  iconFor,
  iconForAction,
  iconForItem,
} from './command-icons';
import { RootCommandList } from './command-list';
import {
  markDebugPerformance,
  measureDebugPerformance,
  measureDebugPerformanceSync,
} from './debug-performance';
import { ExtensionViewRenderer } from './extension-view';
import {
  allViewItems,
  filterCommandItems,
  filterCommandSections,
  valuesMatch,
} from './filtering';
import {
  actionDefinition,
  actionDescription,
  actionPanelFromActions,
  actionsFromPanel,
  type CommandAction,
  type CommandActionPanel,
  type CommandItem,
  type CommandItemAppearance,
  type CommandView,
  type CommandViewPatch,
  canCustomizeCommandAction,
} from './model';
import type { NevermindApi, PaletteMode, ShortcutRecord } from './preload-api';
import {
  ShortcutManagerView,
  type ShortcutRecordLike,
  shortcutItems,
  shortcutOptionRows,
  shortcutRecorderRows,
} from './shortcut-manager';
import {
  acceleratorFromKeyboardEvent,
  keyNameForShortcut,
  normalizedShortcut,
} from './shortcuts';
import {
  type ActionPanelRow,
  EMPTY_ACTIONS_TITLE,
  EMPTY_ITEMS_TITLE,
  EMPTY_ROOT_SUBTITLE,
  EMPTY_ROOT_TITLE,
  EmptyState,
  type FormValue,
  MarkdownContent,
  SearchAccessory,
  setShortcutLabelHyperKey,
  shortcutLabel,
  Toast,
} from './ui';
import { useAiChat } from './use-ai-chat';
import { useExtensionNavigation } from './use-extension-navigation';
import { useSearchResults } from './use-search-results';
import { patchCommandView } from './view-patches';

type ActionKind =
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

type ActionIcon = string;

type AppInfo = {
  name?: string;
  path?: string;
};

type Action = {
  id: string;
  kind: ActionKind;
  title: string;
  subtitle: string;
  icon: ActionIcon;
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
  app?: AppInfo;
  extensionId?: string;
  commandId?: string;
  aiChatId?: string;
  extensionFile?: string;
  removable?: boolean;
  customizable?: boolean;
  background?: boolean;
  dismissAfterRun?: 'auto';
  rootAction?: CommandAction;
  actionPanel?: CommandActionPanel;
  shortcut?: string;
  userAliases?: string[];
  appearance?: CommandItemAppearance;
};

type ExtensionViewAction = CommandAction;
type ExtensionViewItem = CommandItem;
type ExtensionView = CommandView;

declare global {
  interface Window {
    nvm: NevermindApi;
  }
}

const PALETTE_HOTKEY_ACTION_ID = '__palette-hotkey__';
const HYPER_KEY_ACTION_ID = '__hyper-key__';

function spotlightConflictView(accelerator: string): ExtensionView {
  const label = shortcutLabel(accelerator);
  const openSettings: CommandAction = {
    type: 'nativeAction',
    title: 'Open Keyboard Shortcuts',
    subtitle: 'Open system keyboard shortcut settings',
    nativeAction: { kind: 'open-keyboard-settings' },
  };
  const dismiss: CommandAction = { type: 'popView', title: 'Dismiss' };
  return {
    type: 'preview',
    title: `${label} conflicts with a system shortcut`,
    content: `# ${label} is used by the system\n\nNevermind cannot use \`${label}\` until the current system shortcut binding is disabled or changed.`,
    actions: [openSettings, dismiss],
    actionPanel: { sections: [{ actions: [openSettings, dismiss] }] },
  };
}

const SETTINGS_ROOT_ACTION: Action = {
  id: 'app-settings',
  kind: 'app-settings',
  title: 'Settings',
  subtitle: 'Configure Nevermind',
  icon: 'settings',
  score: 0,
};
const PALETTE_HOTKEY_PSEUDO_ACTION: Action = {
  id: PALETTE_HOTKEY_ACTION_ID,
  kind: 'builtin',
  title: 'Set Nevermind shortcut',
  subtitle: 'Global shortcut that toggles the palette',
  icon: 'keyboard',
  score: 0,
};
const HYPER_KEY_PSEUDO_ACTION: Action = {
  id: HYPER_KEY_ACTION_ID,
  kind: 'builtin',
  title: 'Set Hyper key',
  subtitle: 'Key combination displayed as Hyper in shortcut labels',
  icon: 'keyboard',
  score: 0,
};

const SEARCH_PLACEHOLDERS = [
  'Watcha gonna do?',
  'I cannot do that... Oh, nevermind.',
  'Make it happen.',
];

const QUERY_HISTORY_STORAGE_KEY = 'nevermind.queryHistory';
const QUERY_HISTORY_LIMIT = 100;

function storedQueryHistory() {
  try {
    const value = JSON.parse(
      window.localStorage.getItem(QUERY_HISTORY_STORAGE_KEY) || '[]',
    );
    return Array.isArray(value)
      ? value
          .map(String)
          .map((item) => item.trim())
          .filter(Boolean)
          .slice(-QUERY_HISTORY_LIMIT)
      : [];
  } catch {
    return [];
  }
}

function saveQueryHistory(history: string[]) {
  window.localStorage.setItem(
    QUERY_HISTORY_STORAGE_KEY,
    JSON.stringify(history.slice(-QUERY_HISTORY_LIMIT)),
  );
}

function seedFormValuesFromView(view: ExtensionView | null) {
  if (view?.type !== 'form') return {};
  return Object.fromEntries(
    (view.fields || []).map((field) => {
      if (field.type === 'checkbox') return [field.id, Boolean(field.value)];
      if (field.type === 'multiselect' || field.type === 'files')
        return [field.id, Array.isArray(field.value) ? field.value : []];
      return [field.id, field.value || ''];
    }),
  );
}

function selectedItemIdForView(view: ExtensionView | null, current = '') {
  if (!view) return '';
  const items = allViewItems(view);
  const declaredSelection =
    view.selectedItemId && items.some((item) => item.id === view.selectedItemId)
      ? view.selectedItemId
      : '';
  return (
    declaredSelection ||
    (current && items.some((item) => item.id === current)
      ? current
      : items[0]?.id || '')
  );
}

export function ExtensionWindowApp({ windowId }: { windowId: string }) {
  const [view, setView] = useState<ExtensionView | null>(null);
  const [windowOptions, setWindowOptions] = useState<Record<string, unknown>>(
    {},
  );
  const [formValues, setFormValues] = useState<Record<string, FormValue>>({});
  const [selectedValue, setSelectedValue] = useState('');
  const aiChat = useAiChat(window.nvm.sendAiMessage, window.nvm.resetAiChat);
  const windowAiChatIdRef = useRef<string | undefined>(undefined);
  const [nevermindAuthed, setNevermindAuthed] = useState<boolean | null>(null);
  const [toast, setToast] = useState<{
    message: string;
    tone?: 'default' | 'error';
  } | null>(null);

  useEffect(() => {
    window.nvm
      .getNevermindAuthStatus()
      .then((status) => setNevermindAuthed(Boolean(status.authed)))
      .catch(() => setNevermindAuthed(false));
    window.nvm.getExtensionWindowState(windowId).then((state) => {
      if (state?.view) {
        setView(state.view);
        setFormValues(seedFormValuesFromView(state.view));
        setSelectedValue(selectedItemIdForView(state.view));
        setWindowOptions(state.options || {});
        if (state.view.aiChat) void aiChat.openChat(state.view);
      }
    });
    return window.nvm.onExtensionWindowView((payload) => {
      if (payload.id === windowId) {
        setView(payload.view);
        setFormValues(seedFormValuesFromView(payload.view));
        setSelectedValue(selectedItemIdForView(payload.view));
        setWindowOptions(payload.options || {});
        if (payload.view.aiChat) void aiChat.openChat(payload.view);
      }
    });
  }, [windowId]);

  useEffect(() => {
    windowAiChatIdRef.current = view?.chatId;
  }, [view?.chatId]);

  useEffect(
    () =>
      window.nvm.onAiChatEvent((event) =>
        aiChat.handleEvent(event, windowAiChatIdRef.current),
      ),
    [],
  );

  function applyPatch(patch: CommandViewPatch) {
    setView((current) => {
      if (!current) return current;
      const nextView = patchCommandView(current, patch, {
        preserveMissingItems: true,
      });
      setSelectedValue(selectedItemIdForView(nextView, selectedValue));
      return nextView;
    });
  }

  async function handleActionResult(
    result?: {
      view?: ExtensionView;
      patch?: CommandViewPatch;
      navigation?: 'root' | 'push' | 'replace' | 'pop';
      toast?: { message: string; tone?: 'default' | 'error' };
    } | void,
  ) {
    if (!result) return;
    if (result.patch) applyPatch(result.patch);
    if (result.navigation === 'pop') await window.nvm.closeExtensionWindow();
    else if (result.view) {
      setView(result.view);
      setFormValues(seedFormValuesFromView(result.view));
      setSelectedValue(selectedItemIdForView(result.view, selectedValue));
    }
    if (result.toast) {
      setToast(result.toast);
      window.setTimeout(() => setToast(null), 2_000);
    }
  }

  async function runAction(action: ExtensionViewAction) {
    if (String(action.type || '').startsWith('camera.')) {
      window.dispatchEvent(
        new CustomEvent('nvm:camera-action', { detail: action }),
      );
      return;
    }
    const nativeKind = (action.nativeAction as { kind?: string } | undefined)
      ?.kind;
    if (String(nativeKind || '').startsWith('camera.')) {
      window.dispatchEvent(
        new CustomEvent('nvm:camera-action', { detail: action.nativeAction }),
      );
      return;
    }
    await handleActionResult(await window.nvm.runViewAction(action));
  }

  function actionPanelRows(
    panel?: CommandView['actionPanel'],
    fallbackActions: ExtensionViewAction[] = [],
  ): ActionPanelRow[] {
    const sections = panel?.sections?.length
      ? panel.sections
      : [{ actions: fallbackActions }];
    return sections.flatMap((section, sectionIndex) => {
      const rows = (section.actions || []).map((action, index) => ({
        value: `window-action:${sectionIndex}:${index}:${action.type}:${action.title}`,
        icon: iconForAction(action),
        title: action.title,
        subtitle: actionDescription(action),
        shortcut: action.shortcut,
        onSelect: () => runAction(action),
        className:
          action.style === 'destructive' ? 'result dangerResult' : 'result',
      }));
      return section.title
        ? [
            {
              value: `window-section:${sectionIndex}`,
              title: section.title,
              subtitle: '',
              sectionHeader: true,
              onSelect: () => {},
              className: 'actionSectionHeader',
            },
            ...rows,
          ]
        : rows;
    });
  }

  function renderMarkdown(content: string) {
    return <MarkdownContent content={content} />;
  }

  function runDefaultAction(item: ExtensionViewItem) {
    const action =
      item.primaryAction ||
      actionsFromPanel(item.actionPanel, item.actions || [])[0];
    if (action) runAction(action);
  }

  function dragPathForItem(item: ExtensionViewItem) {
    return item.path || item.filePath || null;
  }

  function startItemDrag(event: React.DragEvent, item: ExtensionViewItem) {
    const filePath = dragPathForItem(item);
    if (!filePath) return;
    event.preventDefault();
    window.nvm.startFileDrag(filePath);
  }

  if (!view)
    return (
      <div className="extensionWindowShell">
        <EmptyState icon={<Search size={24} />} title="Loading window…" />
      </div>
    );

  function renderWindowActionPanel(rows: unknown[]) {
    const actionRows = (rows as ActionPanelRow[]).filter(
      (row) => !row.sectionHeader,
    );
    if (actionRows.length === 0) return null;
    return (
      <div className="extensionWindowActions">
        {actionRows.map((row) => (
          <button
            key={row.value}
            className={
              row.className?.includes('danger')
                ? 'dangerWindowAction'
                : undefined
            }
            type="button"
            onClick={row.onSelect}
          >
            {row.icon}
            <span>{row.title}</span>
            {row.shortcut ? <small>{shortcutLabel(row.shortcut)}</small> : null}
          </button>
        ))}
      </div>
    );
  }

  return (
    <Command
      className={`extensionWindowShell ${windowOptions.titleBar === 'hidden' ? 'extensionWindowTitleBarHidden' : ''} ${windowOptions.chrome === 'none' ? 'extensionWindowChromeNone' : ''}`}
      value={selectedValue}
      onValueChange={setSelectedValue}
    >
      {toast ? <Toast message={toast.message} tone={toast.tone} /> : null}
      <main className="extensionWindowBody">
        <ExtensionViewRenderer
          view={view}
          aiChat={aiChat}
          nevermindAuthed={nevermindAuthed}
          onSignInToNevermind={() =>
            window.nvm
              .signInToNevermind()
              .then((result) => setNevermindAuthed(Boolean(result.ok)))
          }
          formValues={formValues}
          setFormValues={setFormValues}
          filterItems={(items) => items || []}
          filterSections={(currentView) => currentView.sections}
          renderMarkdown={renderMarkdown}
          renderActionPanel={(rows) => renderWindowActionPanel(rows)}
          actionPanelRows={(panel, fallbackActions) =>
            actionPanelRows(panel, fallbackActions as ExtensionViewAction[])
          }
          renderRootIcon={(item) => iconForItem(item)}
          runDefaultAction={runDefaultAction}
          runAction={runAction}
          sendAiPrompt={(message) => aiChat.sendPrompt(message, view.chatId)}
          abortAiChat={(chatId) => window.nvm.abortAiChat(chatId)}
          dragPathForItem={dragPathForItem}
          startItemDrag={startItemDrag}
          selectedItemId={selectedValue}
        />
      </main>
    </Command>
  );
}

export function App() {
  const inputRef = useRef<HTMLInputElement>(null);
  const resultsListRef = useRef<HTMLDivElement>(null);
  const paletteRef = useRef<HTMLDivElement>(null);
  const requestedIcons = useRef(new Set<string>());
  const runningAppsRequestIdRef = useRef(0);
  const aiChatOpenRef = useRef(false);
  const aiChatIdRef = useRef<string | undefined>(undefined);
  const lastVisibleAiChatIdRef = useRef<string | undefined>(undefined);
  const runningViewActionsRef = useRef(new Set<string>());
  const paletteModeRef = useRef<PaletteMode | null>(null);
  const queryRef = useRef('');
  const queryHistoryRef = useRef<string[]>([]);
  const queryHistoryIndexRef = useRef<number | null>(null);
  const queryHistoryDraftRef = useRef('');
  const [query, setQuery] = useState('');
  const [refreshNonce, setRefreshNonce] = useState(0);
  const [actions, setActions] = useSearchResults<Action>(
    window.nvm.search,
    query,
    refreshNonce,
  );
  const [iconUrls, setIconUrls] = useState<Record<string, string | null>>({});
  const [runningAppPaths, setRunningAppPaths] = useState<
    Record<string, boolean>
  >({});
  const [runningAppPathsRefreshNonce, setRunningAppPathsRefreshNonce] =
    useState(0);
  const [selectedValue, setSelectedValue] = useState('');
  const selectedValueRef = useRef('');
  const [optionsFor, setOptionsFor] = useState<Action | null>(null);
  const [extensionItemOptionsFor, setExtensionItemOptionsFor] =
    useState<ExtensionViewItem | null>(null);
  const [actionSubmenuFor, setActionSubmenuFor] = useState<{
    title: string;
    panel: CommandActionPanel;
  } | null>(null);
  const [confirmRemoveFor, setConfirmRemoveFor] = useState<Action | null>(null);
  const [confirmViewActionFor, setConfirmViewActionFor] =
    useState<ExtensionViewAction | null>(null);
  const [aliasFor, setAliasFor] = useState<Action | null>(null);
  const [previewFor, setPreviewFor] = useState<Action | null>(null);
  const extensionNavigation = useExtensionNavigation();
  const extensionView = extensionNavigation.view;
  const extensionViewBackStack = extensionNavigation.backStack;
  const aiChat = useAiChat(window.nvm.sendAiMessage, window.nvm.resetAiChat);
  const [nevermindAuthed, setNevermindAuthed] = useState<boolean | null>(null);
  const [toast, setToast] = useState<{
    message: string;
    tone?: 'default' | 'error';
  } | null>(null);
  const [placeholderIndex, setPlaceholderIndex] = useState(
    SEARCH_PLACEHOLDERS.length - 1,
  );
  const [pendingShortcutReveal, setPendingShortcutReveal] = useState(false);
  const [childQuery, setChildQuery] = useState('');
  const [shortcutFor, setShortcutFor] = useState<Action | null>(null);
  const [recordedShortcut, setRecordedShortcut] = useState('');
  const [shortcutManagerOpen, setShortcutManagerOpen] = useState(false);
  const [shortcutRecords, setShortcutRecords] = useState<ShortcutRecord[]>([]);
  const [shortcutOptionsFor, setShortcutOptionsFor] =
    useState<ShortcutRecord | null>(null);
  const [formValues, setFormValues] = useState<Record<string, FormValue>>({});
  const [siblingViews, setSiblingViews] = useState<ExtensionView[]>([]);
  const extensionViewRef = useRef<ExtensionView | null>(null);
  const wasChildOpenRef = useRef(false);
  const scrollResultsToTop = () => resultsListRef.current?.scrollTo({ top: 0 });
  function selectValue(value: string) {
    selectedValueRef.current = value;
    setSelectedValue(value);
  }
  useEffect(() => {
    extensionViewRef.current = extensionView;
  }, [extensionView]);
  useEffect(() => {
    selectedValueRef.current = selectedValue;
  }, [selectedValue]);
  useEffect(() => {
    queryRef.current = query;
  }, [query]);
  useEffect(() => {
    queryHistoryRef.current = storedQueryHistory();
  }, []);
  useEffect(() => {
    markDebugPerformance('app.commit', {
      queryLength: query.length,
      childQueryLength: childQuery.length,
      actionCount: actions.length,
      selectedValue,
      extensionViewType: extensionView?.type,
      extensionViewId: extensionView?.id,
      siblingViewCount: siblingViews.length,
    });
  });

  useEffect(() => {
    window.nvm
      .getSetting('hyperKey')
      .then(setShortcutLabelHyperKey)
      .catch(() => {});
  }, []);

  useEffect(() => {
    if (!extensionView?.refresh?.id || !extensionView.refresh.intervalMs)
      return;
    const viewKey = viewIdentity(extensionView);
    const refreshId = extensionView.refresh.id;
    const intervalMs = extensionView.refresh.intervalMs;
    let cancelled = false;
    let running = false;
    const refresh = async () => {
      const current = extensionViewRef.current;
      if (
        cancelled ||
        running ||
        viewIdentity(current) !== viewKey ||
        current?.refresh?.id !== refreshId
      )
        return;
      running = true;
      try {
        const result = await measureDebugPerformance(
          'view.refresh',
          { refreshId, viewId: current.id, viewKey, alwaysLog: true },
          () => window.nvm.refreshView({ id: refreshId, viewId: current.id }),
        );
        if (
          !result?.skipped &&
          !cancelled &&
          viewIdentity(extensionViewRef.current) === viewKey
        )
          await handleViewActionResult(result, 'replace');
      } catch (error) {
        await window.nvm.log('warn', 'Extension view refresh failed', {
          viewKey,
          error: error instanceof Error ? error.message : String(error),
        });
      } finally {
        running = false;
      }
    };
    const minimumIntervalMs = extensionView.type === 'grid' ? 5000 : 1000;
    const timer = window.setInterval(
      refresh,
      Math.max(minimumIntervalMs, intervalMs),
    );
    const immediateTimer = extensionView.refresh.immediate
      ? window.setTimeout(refresh, 0)
      : undefined;
    return () => {
      cancelled = true;
      window.clearInterval(timer);
      if (immediateTimer !== undefined) window.clearTimeout(immediateTimer);
    };
  }, [
    extensionView?.id,
    extensionView?.type,
    extensionView?.title,
    extensionView?.refresh?.id,
    extensionView?.refresh?.intervalMs,
    extensionView?.refresh?.immediate,
  ]);

  useLayoutEffect(() => {
    const card = resultsListRef.current;
    if (!card) return;
    const LOADING_BORDER_MS_PER_PX = 0.8;
    const updateDuration = () => {
      const rect = card.getBoundingClientRect();
      const perimeter = (rect.width + rect.height) * 2;
      if (!perimeter) return;
      card.style.setProperty(
        '--loading-border-duration',
        `${Math.round(perimeter * LOADING_BORDER_MS_PER_PX)}ms`,
      );
    };
    updateDuration();
    const observer = new ResizeObserver(updateDuration);
    observer.observe(card);
    return () => observer.disconnect();
  }, [extensionView?.id]);

  useLayoutEffect(() => {
    const palette = paletteRef.current;
    if (!palette) return;
    if (siblingViews.length === 0) {
      palette.style.removeProperty('--spine-top');
      palette.style.removeProperty('--spine-height');
      return;
    }
    const updateSpine = () => {
      const searchRow = palette.querySelector(
        '.searchRow',
      ) as HTMLElement | null;
      const activePane = palette.querySelector(
        '.results',
      ) as HTMLElement | null;
      if (!searchRow || !activePane) return;
      const paletteRect = palette.getBoundingClientRect();
      const top = searchRow.getBoundingClientRect().bottom - paletteRect.top;
      const bottom =
        activePane.getBoundingClientRect().top - paletteRect.top - 11;
      palette.style.setProperty('--spine-top', `${top}px`);
      palette.style.setProperty(
        '--spine-height',
        `${Math.max(0, bottom - top)}px`,
      );
    };
    updateSpine();
    const observer = new ResizeObserver(updateSpine);
    observer.observe(palette);
    palette
      .querySelectorAll('.siblingPane')
      .forEach((node) => observer.observe(node));
    return () => observer.disconnect();
  }, [siblingViews, extensionView]);

  useEffect(() => {
    let cancelled = false;
    window.nvm.getNevermindAuthStatus().then((status) => {
      if (!cancelled) setNevermindAuthed(status.authed);
    });
    const stop = window.nvm.onNevermindAuthChanged((status) =>
      setNevermindAuthed(status.authed),
    );
    return () => {
      cancelled = true;
      stop();
    };
  }, []);

  useEffect(() => {
    const focusInput = () => {
      const input = inputRef.current;
      if (!input) return;
      input.focus();
      if (!input.readOnly) input.select();
      else input.setSelectionRange(input.value.length, input.value.length);
    };

    const stopShown = window.nvm.onShown(() => {
      markDebugPerformance('palette.shown', { queryLength: query.length });
      setOptionsFor(null);
      setExtensionItemOptionsFor(null);
      setConfirmRemoveFor(null);
      setPreviewFor(null);
      setChildQuery('');
      setShortcutFor(null);
      setRecordedShortcut('');
      setShortcutManagerOpen(false);
      setShortcutOptionsFor(null);
      if (!aiChatOpenRef.current) {
        extensionNavigation.clearView();
        aiChat.setMessages([]);
      }
      setRefreshNonce((nonce) => nonce + 1);
      setPlaceholderIndex((index) => (index + 1) % SEARCH_PLACEHOLDERS.length);
      if (!aiChatOpenRef.current) {
        extensionNavigation.setBackStack([]);
        setSiblingViews([]);
      }
      requestAnimationFrame(focusInput);
      window.setTimeout(focusInput, 50);
    });
    const stopShortcutShown = window.nvm.onShortcutShown(() => {
      markDebugPerformance('palette.shortcut-shown');
      requestAnimationFrame(focusInput);
      window.setTimeout(focusInput, 50);
    });
    const stopHidden = window.nvm.onHidden(() => {
      markDebugPerformance('palette.hidden', {
        aiChatOpen: aiChatOpenRef.current,
      });
      const closedAiChatId = aiChatOpenRef.current
        ? aiChatIdRef.current
        : undefined;
      setRootQuery('');
      setOptionsFor(null);
      setExtensionItemOptionsFor(null);
      setConfirmRemoveFor(null);
      setPreviewFor(null);
      setChildQuery('');
      setShortcutFor(null);
      setRecordedShortcut('');
      setShortcutManagerOpen(false);
      setShortcutOptionsFor(null);
      if (!aiChatOpenRef.current) {
        extensionNavigation.clearView();
        aiChat.setMessages([]);
      }
      extensionNavigation.setBackStack([]);
      setSiblingViews([]);
      if (closedAiChatId) void window.nvm.aiChatExited(closedAiChatId);
    });
    const stopApps = window.nvm.onAppsIndexed(() =>
      setRefreshNonce((nonce) => nonce + 1),
    );
    const stopClipboard = window.nvm.onClipboardChanged(() =>
      setRefreshNonce((nonce) => nonce + 1),
    );
    const stopRootItems = window.nvm.onRootItemsChanged(() =>
      setRefreshNonce((nonce) => nonce + 1),
    );
    const stopOpenActionView = window.nvm.onOpenActionView(async (payload) => {
      markDebugPerformance('view.open-event', {
        hasView: Boolean(payload?.view),
        asSibling: Boolean(payload?.asSibling),
        revealWhenReady: Boolean(payload?.revealWhenReady),
        viewType: payload?.view?.type,
        viewId: payload?.view?.id,
      });
      if (!payload?.view) return;
      setOptionsFor(null);
      setPreviewFor(null);
      const current = extensionViewRef.current;
      if (payload.asSibling && current && current.id !== payload.view.id) {
        setSiblingViews((siblings) => [...siblings, current]);
      } else if (!payload.asSibling) {
        setSiblingViews([]);
      }
      if (payload.view.aiChat) await openAiChat(payload.view, 'root');
      else showExtensionView(payload.view, 'root');
      markShortcutReady(Boolean(payload?.revealWhenReady));
    });
    const stopViewPatch = window.nvm.onViewPatch((payload) => {
      markDebugPerformance('view.patch-event', {
        viewId: payload?.viewId,
        itemPatchCount: payload?.patch?.items?.length || 0,
        removeCount: payload?.patch?.removeItemIds?.length || 0,
        mode: payload?.patch?.mode,
      });
      if (!payload) return;
      const current = extensionViewRef.current;
      if (payload.viewId && current?.id !== payload.viewId) return;
      if (!current) return;
      applyViewPatch(payload.patch);
    });
    const stopViewHydrate = window.nvm.onViewHydrate((payload) => {
      markDebugPerformance('view.hydrate-event', {
        viewId: payload?.viewId,
        hasItems: Boolean(payload?.items),
        hasError: Boolean(payload?.error),
      });
      const current = extensionViewRef.current;
      if (payload.viewId && current?.id !== payload.viewId) return;
      if (!current) return;
      if (payload.error) {
        const retryAction: CommandAction | undefined = payload.retry
          ? {
              type: 'nativeAction',
              title: 'Retry',
              nativeAction: {
                kind: 'view-hydrate-retry',
                viewId: payload.viewId,
              },
            }
          : undefined;
        const dismissAction: CommandAction = {
          type: 'popView',
          title: 'Dismiss',
        };
        showExtensionView(
          {
            type: 'preview',
            title: 'Something went wrong',
            content: `# Failed to load items\n\n\`\`\`\n${payload.error.message}\n\`\`\``,
            actions: [...(retryAction ? [retryAction] : []), dismissAction],
            ...(retryAction
              ? { actionPanel: { sections: [{ actions: [retryAction] }] } }
              : {}),
          },
          'replace',
        );
        return;
      }
      if (payload.items) {
        applyViewPatch({
          mode: 'replace',
          items: payload.items as any,
          isLoading:
            payload.isLoading === undefined ? false : payload.isLoading,
        });
      }
    });
    const stopAi = window.nvm.onAiChatEvent((event) => {
      markDebugPerformance(`ai.event.${event.type}`, {
        chatId: event.chatId,
        textLength: event.text?.length || 0,
        label: event.label,
      });
      if (event.type === 'debug')
        window.nvm.log(
          'debug',
          `Nevermind AI: ${event.label || ''}`,
          event.data,
        );
      aiChat.handleEvent(event, aiChatIdRef.current);
    });
    return () => {
      stopShown();
      stopShortcutShown();
      stopHidden();
      stopApps();
      stopClipboard();
      stopRootItems();
      stopOpenActionView();
      stopViewPatch();
      stopViewHydrate();
      stopAi();
    };
  }, []);

  const extensionViewSelectionKey = extensionView
    ? `${extensionView.id || ''}:${extensionView.type}:${extensionView.title}:${extensionView.selectedItemId || ''}`
    : '';

  useLayoutEffect(() => {
    function selectExtensionItem(view: ExtensionView) {
      const items = filterExtensionItems(allViewItems(view));
      const current = selectedValueRef.current;
      const declaredSelection =
        view.selectedItemId &&
        items.some((item) => item.id === view.selectedItemId)
          ? view.selectedItemId
          : '';
      selectValue(
        declaredSelection ||
          (current && items.some((item) => item.id === current)
            ? current
            : items[0]?.id || ''),
      );
    }

    if (shortcutFor) selectValue('shortcut:save');
    else if (shortcutOptionsFor)
      selectValue(getShortcutOptionRows()[0]?.value ?? '');
    else if (shortcutManagerOpen)
      selectValue(getShortcutRows()[0]?.value ?? '');
    else if (aliasFor) selectValue(getAliasActionRows()[0]?.value ?? '');
    else if (confirmRemoveFor)
      selectValue(getConfirmActionRows()[0]?.value ?? '');
    else if (confirmViewActionFor)
      selectValue(getConfirmViewActionRows()[0]?.value ?? '');
    else if (actionSubmenuFor)
      selectValue(
        actionPanelRows(
          actionSubmenuFor.panel,
          [],
          'action-submenu',
          true,
        ).find((row) => !row.sectionHeader)?.value ?? '',
      );
    else if (extensionItemOptionsFor)
      selectValue(getExtensionItemActionRows()[0]?.value ?? '');
    else if (optionsFor) selectValue(getOptionActionRows()[0]?.value ?? '');
    else if (previewFor) selectValue('preview');
    else if (extensionView && isFilterableExtensionView)
      selectExtensionItem(extensionView);
    else if (extensionView?.actions?.length)
      selectValue(
        `extension-view:0:${extensionView.actions[0].type}:${extensionView.actions[0].title}`,
      );
    else if (extensionView) selectValue('preview');
    else selectValue(actions[0]?.id ?? '');
  }, [
    actions,
    actionSubmenuFor,
    aliasFor,
    childQuery,
    confirmRemoveFor,
    confirmViewActionFor,
    extensionItemOptionsFor,
    optionsFor,
    previewFor,
    extensionViewSelectionKey,
    shortcutFor,
    shortcutManagerOpen,
    shortcutRecords,
    shortcutOptionsFor,
  ]);

  useEffect(() => {
    setChildQuery('');
  }, [
    actionSubmenuFor?.title,
    confirmRemoveFor?.id,
    confirmViewActionFor?.title,
    extensionItemOptionsFor?.id,
    extensionView?.title,
    extensionView?.type,
    optionsFor?.id,
    previewFor?.id,
  ]);

  useEffect(() => {
    setFormValues(seedFormValuesFromView(extensionView));
  }, [extensionView]);

  useEffect(() => {
    if (!pendingShortcutReveal || !extensionView) return;
    const frame = requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        window.nvm.shortcutReady();
        setPendingShortcutReveal(false);
      });
    });
    return () => cancelAnimationFrame(frame);
  }, [pendingShortcutReveal, extensionView]);

  useEffect(() => {
    const visibleAiChat = extensionView?.aiChat
      ? extensionView
      : [...siblingViews].reverse().find((view) => view.aiChat);
    const isAiChat = Boolean(visibleAiChat);
    const isLarge =
      extensionView?.size === 'large' ||
      extensionView?.presentation === 'preview';
    const isActionPanelOpen = Boolean(
      actionSubmenuFor || extensionItemOptionsFor,
    );
    aiChatOpenRef.current = isAiChat;
    aiChatIdRef.current = visibleAiChat?.chatId;
    const previousAiChatId = lastVisibleAiChatIdRef.current;
    if (
      previousAiChatId &&
      (!visibleAiChat || previousAiChatId !== visibleAiChat.chatId)
    )
      void window.nvm.aiChatExited(previousAiChatId);
    lastVisibleAiChatIdRef.current = visibleAiChat?.chatId;
    const mode: PaletteMode =
      previewFor || (isLarge && !isActionPanelOpen)
        ? 'preview'
        : siblingViews.length > 0
          ? 'stacked'
          : isAiChat
            ? 'ai-chat'
            : 'default';
    if (paletteModeRef.current !== mode) {
      paletteModeRef.current = mode;
      window.nvm.setPaletteMode(mode);
    }
  }, [
    actionSubmenuFor,
    extensionItemOptionsFor,
    extensionView,
    previewFor,
    siblingViews,
  ]);

  useEffect(() => {
    if (!extensionView?.aiChat && !siblingViews.some((view) => view.aiChat))
      return;
    requestAnimationFrame(() => {
      aiChat.messagesRef.current?.scrollTo({
        top: aiChat.messagesRef.current.scrollHeight,
      });
    });
  }, [aiChat.messages, aiChat.busy, extensionView, siblingViews]);

  useEffect(() => {
    if (!shortcutFor) return;
    window.nvm.suspendShortcuts();
    return () => {
      window.nvm.resumeShortcuts();
    };
  }, [shortcutFor?.id]);

  useEffect(() => {
    for (const action of actions) {
      const appPath = appPathForIcon(action);
      if (!appPath || requestedIcons.current.has(appPath)) continue;

      requestedIcons.current.add(appPath);
      window.nvm.getAppIcon(appPath).then((iconUrl) => {
        if (iconUrl)
          setIconUrls((current) => ({
            ...current,
            [action.id]: iconUrl,
            [appPath]: iconUrl,
          }));
        else requestedIcons.current.delete(appPath);
      });
    }
  }, [actions]);

  useEffect(() => {
    const views = [extensionView, ...siblingViews].filter(
      Boolean,
    ) as ExtensionView[];
    for (const view of views) {
      for (const item of allViewItems(view)) {
        const appPath = diskPathForItem(item);
        if (
          !appPath?.endsWith('.app') ||
          item.image ||
          iconUrls[appPath] ||
          requestedIcons.current.has(appPath)
        )
          continue;
        requestedIcons.current.add(appPath);
        window.nvm.getAppIcon(appPath).then((iconUrl) => {
          if (iconUrl)
            setIconUrls((current) => ({ ...current, [appPath]: iconUrl }));
          else requestedIcons.current.delete(appPath);
        });
      }
    }
  }, [extensionView, siblingViews, iconUrls]);

  useEffect(
    () =>
      window.nvm.onRunningAppPathsChanged(() => {
        setRunningAppPathsRefreshNonce((nonce) => nonce + 1);
      }),
    [],
  );

  useEffect(() => {
    const appPaths = Array.from(
      new Set(actions.map(appPathForRunningStatus).filter(Boolean) as string[]),
    );
    if (!appPaths.length) return;
    const requestId = ++runningAppsRequestIdRef.current;
    window.nvm
      .getRunningAppPaths(appPaths)
      .then((openPaths) => {
        if (requestId !== runningAppsRequestIdRef.current) return;
        const openSet = new Set(openPaths);
        setRunningAppPaths((current) => {
          let changed = false;
          const next = { ...current };
          for (const appPath of appPaths) {
            const isOpen = openSet.has(appPath);
            if (next[appPath] !== isOpen) {
              next[appPath] = isOpen;
              changed = true;
            }
          }
          return changed ? next : current;
        });
      })
      .catch(() => {});
  }, [actions, runningAppPathsRefreshNonce]);

  const selectedAction = useMemo(
    () => actions.find((action) => action.id === selectedValue),
    [actions, selectedValue],
  );
  const createAction = useMemo(
    () =>
      actions.find(
        (action) =>
          action.kind === 'extension-root-item' &&
          action.extensionId === 'nevermind.ai-builder' &&
          action.id.startsWith('extension-root:nevermind.ai-builder:ai:'),
      ),
    [actions],
  );
  const isFilterableExtensionView =
    extensionView?.type === 'list' || extensionView?.type === 'grid';
  const isRootLikeExtensionView = extensionView?.id === 'clipboard-history';
  const isFilterableChildOpen = Boolean(
    actionSubmenuFor ||
      confirmRemoveFor ||
      confirmViewActionFor ||
      extensionItemOptionsFor ||
      optionsFor ||
      aliasFor ||
      shortcutManagerOpen ||
      isFilterableExtensionView,
  );
  const isLargeExtensionView = Boolean(
    extensionView?.size === 'large' &&
      !actionSubmenuFor &&
      !extensionItemOptionsFor,
  );
  const isChildOpen = Boolean(
    shortcutFor ||
      shortcutOptionsFor ||
      shortcutManagerOpen ||
      actionSubmenuFor ||
      confirmRemoveFor ||
      confirmViewActionFor ||
      extensionItemOptionsFor ||
      optionsFor ||
      aliasFor ||
      previewFor ||
      extensionView,
  );
  const isVisuallyStacked =
    (isChildOpen && !isRootLikeExtensionView) || siblingViews.length > 0;
  const childPlaceholder = actionSubmenuFor
    ? `Filter ${actionSubmenuFor.title}`
    : shortcutOptionsFor
      ? `Actions for “${shortcutOptionsFor.action.title}”`
      : shortcutManagerOpen
        ? 'Filter keyboard shortcuts'
        : confirmRemoveFor || confirmViewActionFor
          ? 'Filter confirmation actions'
          : extensionItemOptionsFor
            ? `Filter actions for “${extensionItemOptionsFor.title}”`
            : optionsFor
              ? `Filter actions for “${optionsFor.title}”`
              : aliasFor
                ? `Alias for “${aliasFor.title}”`
                : extensionView
                  ? `Filter ${extensionView.title}`
                  : '';
  const inputValue = shortcutFor
    ? recordedShortcut
    : isFilterableChildOpen
      ? childQuery
      : previewFor
        ? previewFor.title
        : extensionView
          ? extensionView.title
          : optionsFor && !query
            ? optionsFor.title
            : query;
  const placeholder = shortcutFor
    ? 'Press a keyboard shortcut'
    : isFilterableChildOpen
      ? extensionView?.searchBarPlaceholder || childPlaceholder
      : SEARCH_PLACEHOLDERS[placeholderIndex];
  const activeSearchQuery =
    !shortcutFor && isFilterableChildOpen
      ? childQuery
      : !isChildOpen
        ? query
        : '';
  const activeSearchScope =
    !shortcutFor && isFilterableChildOpen
      ? `child:${actionSubmenuFor?.title || confirmRemoveFor?.id || confirmViewActionFor?.title || extensionItemOptionsFor?.id || optionsFor?.id || aliasFor?.id || extensionView?.id || extensionView?.title || shortcutManagerOpen}`
      : !isChildOpen
        ? 'root'
        : '';

  function setRootQuery(value: string) {
    queryHistoryIndexRef.current = null;
    queryHistoryDraftRef.current = '';
    setQuery(value);
  }

  function rememberRootQuery(value = queryRef.current) {
    const trimmed = value.trim();
    if (!trimmed) return;
    const history = [
      ...queryHistoryRef.current.filter((item) => item !== trimmed),
      trimmed,
    ].slice(-QUERY_HISTORY_LIMIT);
    queryHistoryRef.current = history;
    saveQueryHistory(history);
    queryHistoryIndexRef.current = null;
    queryHistoryDraftRef.current = '';
  }

  function rootSelectionIsAtTop() {
    return !actions.length || selectedValueRef.current === actions[0]?.id;
  }

  function navigateRootQueryHistory(direction: -1 | 1) {
    const history = queryHistoryRef.current;
    if (!history.length) return direction < 0 && rootSelectionIsAtTop();
    if (direction < 0 && !rootSelectionIsAtTop()) return false;
    let index = queryHistoryIndexRef.current;
    if (index === null) {
      if (direction > 0) return false;
      queryHistoryDraftRef.current = queryRef.current;
      index = history.length - 1;
    } else {
      index += direction;
    }
    if (index < 0) index = 0;
    if (index >= history.length) {
      queryHistoryIndexRef.current = null;
      setQuery(queryHistoryDraftRef.current);
      return true;
    }
    queryHistoryIndexRef.current = index;
    setQuery(history[index]);
    requestAnimationFrame(() => {
      const input = inputRef.current;
      input?.focus();
      input?.setSelectionRange(input.value.length, input.value.length);
    });
    return true;
  }

  useEffect(() => {
    if (!isFilterableChildOpen && !shortcutFor) return;
    requestAnimationFrame(() => inputRef.current?.focus());
  }, [
    confirmRemoveFor?.id,
    confirmViewActionFor?.title,
    extensionItemOptionsFor?.id,
    extensionView?.title,
    extensionView?.type,
    isFilterableChildOpen,
    optionsFor?.id,
    shortcutFor?.id,
  ]);

  useEffect(() => {
    if (optionsFor) refreshShortcuts();
  }, [optionsFor?.id]);

  useEffect(() => {
    if (isChildOpen) {
      wasChildOpenRef.current = true;
      return;
    }
    if (!wasChildOpenRef.current) return;
    wasChildOpenRef.current = false;
    requestAnimationFrame(() => inputRef.current?.focus());
  }, [isChildOpen]);

  useLayoutEffect(() => {
    if (!activeSearchQuery) return;
    scrollResultsToTop();
    const frame = requestAnimationFrame(scrollResultsToTop);
    return () => cancelAnimationFrame(frame);
  }, [activeSearchQuery, activeSearchScope, actions]);

  function markShortcutReady(shouldReveal: boolean) {
    if (shouldReveal) setPendingShortcutReveal(true);
  }

  function showToast(message: string, tone: 'default' | 'error' = 'default') {
    setToast({ message, tone });
    const duration = tone === 'error' ? 4000 : 2200;
    window.setTimeout(
      () =>
        setToast((current) => (current?.message === message ? null : current)),
      duration,
    );
  }

  async function sendAiPrompt(message: string, chatId = extensionView?.chatId) {
    await aiChat.sendPrompt(message, chatId);
  }

  function showExtensionView(
    view: ExtensionView,
    navigation: 'root' | 'push' | 'replace' = 'replace',
  ) {
    extensionNavigation.showView(view, navigation);
  }

  function showActionLoadingView(
    title = 'Running…',
    subtitle = 'Waiting for the action to finish',
    navigation: 'root' | 'push' | 'replace' = 'root',
  ) {
    showExtensionView(
      {
        type: 'list',
        id: `action-loading:${title}`,
        title,
        presentation: navigation === 'root' ? 'root' : undefined,
        searchBarPlaceholder: title,
        isLoading: true,
        items: [
          {
            id: 'action-loading',
            title,
            subtitle,
            icon: 'sparkles',
          },
        ],
      },
      navigation,
    );
  }

  function popExtensionView() {
    setExtensionItemOptionsFor(null);
    if (extensionViewBackStack.length === 0 && siblingViews.length > 0) {
      const next = siblingViews[siblingViews.length - 1];
      setSiblingViews((siblings) => siblings.slice(0, -1));
      extensionNavigation.showView(next, 'root');
      return;
    }
    extensionNavigation.popView();
  }

  function selectionAfterPatch(
    current: ExtensionView,
    next: ExtensionView,
    patch: CommandViewPatch,
  ) {
    const nextItems = allViewItems(next);
    if (nextItems.length === 0) return '';
    if (patch.selectedItemId !== undefined)
      return nextItems.some((item) => item.id === patch.selectedItemId)
        ? patch.selectedItemId
        : nextItems[0].id;
    if (
      next.selectedItemId &&
      nextItems.some((item) => item.id === next.selectedItemId)
    )
      return next.selectedItemId;
    const selected = selectedValueRef.current;
    if (selected && nextItems.some((item) => item.id === selected))
      return selected;
    const currentItems = allViewItems(current);
    const currentIndex = currentItems.findIndex((item) => item.id === selected);
    const fallbackIndex =
      currentIndex < 0 ? 0 : Math.min(currentIndex, nextItems.length - 1);
    return nextItems[fallbackIndex]?.id || '';
  }

  function viewIdentity(view: ExtensionView | null) {
    return view ? `${view.id || ''}:${view.type}:${view.title}` : '';
  }

  function restorePatchViewport(
    viewKey: string,
    scrollTop: number | undefined,
    selectedItemId: string,
  ) {
    requestAnimationFrame(() => {
      if (viewIdentity(extensionViewRef.current) !== viewKey) return;
      if (selectedItemId && selectedValueRef.current !== selectedItemId) {
        selectValue(selectedItemId);
      }
      if (scrollTop !== undefined && resultsListRef.current)
        resultsListRef.current.scrollTop = scrollTop;
    });
  }

  function applyViewPatch(patch?: CommandViewPatch) {
    if (!patch) return;
    measureDebugPerformanceSync(
      'view.apply-patch',
      {
        itemPatchCount: patch.items?.length || 0,
        removeCount: patch.removeItemIds?.length || 0,
        mode: patch.mode,
        currentViewId: extensionViewRef.current?.id,
      },
      () => {
        const current = extensionViewRef.current;
        if (!current) return;
        const scrollTop = resultsListRef.current?.scrollTop;
        const next = patchCommandView(current, patch);
        const nextSelected = selectionAfterPatch(current, next, patch);
        extensionViewRef.current = next;
        extensionNavigation.setView(next);
        selectValue(nextSelected);
        restorePatchViewport(viewIdentity(current), scrollTop, nextSelected);
      },
    );
  }

  async function handleViewActionResult(
    result?: {
      view?: ExtensionView;
      patch?: CommandViewPatch;
      navigation?: 'root' | 'push' | 'replace' | 'pop';
      toast?: { message: string; tone?: 'default' | 'error' };
    } | void,
    fallbackNavigation: 'push' | 'replace' | 'root' = 'push',
  ) {
    if (!result) return;
    return measureDebugPerformance(
      'view.handle-action-result',
      {
        hasView: Boolean(result.view),
        viewType: result.view?.type,
        hasPatch: Boolean(result.patch),
        navigation: result.navigation,
        fallbackNavigation,
      },
      async () => {
        if (result.toast)
          showToast(result.toast.message, result.toast.tone || 'default');
        if (result.patch) applyViewPatch(result.patch);
        if (result.navigation === 'pop') popExtensionView();
        else if (result.view?.aiChat)
          await openAiChat(
            result.view,
            result.navigation || fallbackNavigation,
          );
        else if (result.view)
          showExtensionView(
            result.view,
            result.navigation || fallbackNavigation,
          );
      },
    );
  }

  function actionCanDismissImmediately(action: ExtensionViewAction) {
    return (
      !action.keepPaletteOpen &&
      action.dismissAfterRun === 'auto' &&
      actionDefinition(action)?.dismiss === 'immediate'
    );
  }

  function rootNativeActionCanDismissImmediately(
    action: Action | { kind?: string },
  ) {
    return [
      'open-url',
      'web-search',
      'app',
      'clipboard',
      'file',
      'calculate',
      'builtin',
      'open-keyboard-settings',
    ].includes(String(action.kind));
  }

  function rootActionCanDismissImmediately(
    action:
      | Action
      | { kind?: string; background?: boolean; dismissAfterRun?: 'auto' },
  ) {
    return (
      rootNativeActionCanDismissImmediately(action) ||
      (['extension-action', 'extension-root-item'].includes(
        String(action.kind),
      ) &&
        (action.background || action.dismissAfterRun === 'auto'))
    );
  }

  function rootActionRequestsQuit(action: unknown) {
    const root = action as Action | undefined;
    return (
      root?.id === 'extension-root:nevermind.system:builtin:quit' ||
      root?.id === 'extension:nevermind.system:builtin:quit' ||
      (root?.extensionId === 'nevermind.system' &&
        root?.commandId === 'builtin:quit') ||
      root?.rootAction?.type === 'quitApp'
    );
  }

  function viewActionRequestsQuit(action: ExtensionViewAction) {
    return (
      action.type === 'quitApp' ||
      (action.type === 'nativeAction' &&
        rootActionRequestsQuit(action.nativeAction))
    );
  }

  async function runViewAction(action: ExtensionViewAction, confirmed = false) {
    if (action.requiresConfirmation && !confirmed) {
      setConfirmViewActionFor(action);
      setChildQuery('');
      return;
    }
    if (viewActionRequestsQuit(action)) {
      await window.nvm.quitApp();
      return;
    }
    const nativeAction =
      action.type === 'nativeAction'
        ? (action.nativeAction as
            | Action
            | { kind?: string; action?: Action; actionId?: string }
            | undefined)
        : undefined;
    if (action.type === 'setSearchQuery') {
      const nextQuery = String(action.query ?? action.text ?? '');
      setRootQuery(nextQuery);
      setChildQuery('');
      setOptionsFor(null);
      setExtensionItemOptionsFor(null);
      setActionSubmenuFor(null);
      setPreviewFor(null);
      if (extensionView) extensionNavigation.clearView();
      setSiblingViews([]);
      requestAnimationFrame(() => {
        const input = inputRef.current;
        input?.focus();
        if (action.select !== false) input?.select();
        else input?.setSelectionRange(nextQuery.length, nextQuery.length);
      });
      return;
    }
    if (action.type === 'recordShortcut') {
      const targetAction = action.action as Action | undefined;
      const target =
        targetAction?.id === PALETTE_HOTKEY_ACTION_ID
          ? PALETTE_HOTKEY_PSEUDO_ACTION
          : targetAction?.id === HYPER_KEY_ACTION_ID
            ? HYPER_KEY_PSEUDO_ACTION
            : targetAction;
      if (target) startShortcutRecorder(target);
      return;
    }
    if (nativeAction?.kind === 'record-palette-hotkey') {
      startShortcutRecorder(PALETTE_HOTKEY_PSEUDO_ACTION);
      return;
    }
    if (nativeAction?.kind === 'record-shortcut' && nativeAction.action) {
      startShortcutRecorder(nativeAction.action as Action);
      return;
    }
    if (nativeAction?.kind === 'remove-shortcut' && nativeAction.actionId) {
      const result = await window.nvm.removeShortcut(
        String(nativeAction.actionId),
      );
      showToast(result.message, result.ok ? 'default' : 'error');
      if (result.ok && extensionView?.id === 'keyboard-shortcuts') {
        const refreshed = await window.nvm.execute({
          id: 'keyboard-shortcuts',
          kind: 'extension-action',
          extensionId: 'nevermind.shortcuts',
          registeredActionId: 'keyboard-shortcuts',
          title: 'Keyboard Shortcuts',
          subtitle: 'View, change, or remove global shortcuts',
          icon: 'keyboard',
          score: 16,
        } as Action);
        if (refreshed?.view) showExtensionView(refreshed.view, 'replace');
      }
      return;
    }
    if (action.type === 'previewClipboardItem') {
      setPreviewFor({
        id: `clipboard-preview:${action.text || action.imageDataUrl || action.videoUrl || action.filePath || action.title}`,
        kind: 'clipboard',
        title: action.title,
        subtitle: action.clipboardType,
        icon: 'clipboard',
        score: 0,
        clipboardType: action.clipboardType,
        text: action.text,
        imageDataUrl: action.imageDataUrl,
        imagePath: action.imagePath,
        videoUrl: action.videoUrl,
        filePath: action.filePath,
        thumbnailUrl: action.thumbnailUrl,
      } as Action);
      setExtensionItemOptionsFor(null);
      return;
    }
    if (
      nativeAction?.kind === 'clipboard' &&
      ('imageDataUrl' in nativeAction ||
        'videoUrl' in nativeAction ||
        'text' in nativeAction)
    ) {
      setPreviewFor(nativeAction as Action);
      setExtensionItemOptionsFor(null);
      return;
    }
    if (String(nativeAction?.kind || '').startsWith('camera.')) {
      window.dispatchEvent(
        new CustomEvent('nvm:camera-action', { detail: nativeAction }),
      );
      return;
    }
    const actionKey =
      action.handlerId ||
      `${action.type}:${action.title}:${action.path || action.url || action.text || ''}`;
    if (runningViewActionsRef.current.has(actionKey)) return;
    runningViewActionsRef.current.add(actionKey);
    const dismissedImmediately =
      actionCanDismissImmediately(action) ||
      Boolean(nativeAction && rootActionCanDismissImmediately(nativeAction));
    const loadingNavigation = nativeAction ? 'root' : 'push';
    const definition = actionDefinition(action);
    const showsLoading =
      !dismissedImmediately && !nativeAction && definition?.loading === 'view';
    try {
      markDebugPerformance('view-action.start', {
        actionType: action.type,
        title: action.title,
        dismissedImmediately,
        showsLoading,
      });
      if (dismissedImmediately) {
        markDebugPerformance('view-action.hide-before-ipc', {
          actionType: action.type,
          title: action.title,
        });
        void window.nvm.hide();
      }
      const resultPromise = measureDebugPerformance(
        'view-action.ipc',
        {
          actionType: action.type,
          title: action.title,
          dismissedImmediately,
          showsLoading,
          alwaysLog: true,
        },
        () => window.nvm.runViewAction(action),
      );
      if (!dismissedImmediately && showsLoading)
        showActionLoadingView(
          action.title || 'Running…',
          'Waiting for the action to finish',
          loadingNavigation,
        );
      const result = await resultPromise;
      const resultAfterLoading =
        showsLoading &&
        loadingNavigation === 'push' &&
        result?.navigation === 'push' &&
        result.view
          ? { ...result, navigation: 'replace' as const }
          : result;
      if (
        showsLoading &&
        loadingNavigation === 'push' &&
        (result?.navigation === 'replace' || result?.navigation === 'pop')
      ) {
        extensionNavigation.setBackStack((stack) => stack.slice(0, -1));
      }
      await handleViewActionResult(
        resultAfterLoading,
        showsLoading ? 'replace' : 'push',
      );
      if (
        !dismissedImmediately &&
        !action.keepPaletteOpen &&
        action.dismissAfterRun === 'auto' &&
        !result?.view &&
        !result?.patch &&
        result?.navigation !== 'pop'
      ) {
        if (extensionNavigation.backStack.length > 0) popExtensionView();
        else window.nvm.hide();
      } else if (showsLoading && !result?.view && !result?.navigation) {
        if (loadingNavigation === 'push') popExtensionView();
        else window.nvm.hide();
        if (result?.patch && loadingNavigation === 'push')
          queueMicrotask(() => applyViewPatch(result.patch));
      }
    } finally {
      runningViewActionsRef.current.delete(actionKey);
    }
  }

  async function openAiChat(
    view: ExtensionView,
    navigation: 'root' | 'push' | 'replace' = 'root',
  ) {
    await measureDebugPerformance(
      'ai.open-chat',
      {
        chatId: view.chatId,
        messageCount: view.messages?.length || 0,
        navigation,
      },
      async () => {
        showExtensionView(view, navigation);
        await aiChat.openChat(view);
      },
    );
  }

  async function startBuilderChatFromQuery(prompt: string) {
    const result = await measureDebugPerformance(
      'ai-builder.start-chat',
      { promptLength: prompt.length, alwaysLog: true },
      () =>
        window.nvm.startBuilderChat({ prompt, title: `Automate "${prompt}"` }),
    );
    if (result?.view?.aiChat) await openAiChat(result.view, 'root');
    else if (result?.view) showExtensionView(result.view, 'root');
  }

  async function run(action: Action) {
    if (rootActionRequestsQuit(action)) {
      await window.nvm.quitApp();
      return;
    }
    const dismissedImmediately = rootActionCanDismissImmediately(action);
    markDebugPerformance('root-action.start', {
      id: action.id,
      kind: action.kind,
      title: action.title,
      dismissedImmediately,
    });
    if (dismissedImmediately) {
      markDebugPerformance('root-action.hide-before-ipc', {
        id: action.id,
        kind: action.kind,
        title: action.title,
      });
      void window.nvm.hide();
    }
    const resultPromise = measureDebugPerformance(
      'root-action.ipc',
      {
        id: action.id,
        kind: action.kind,
        title: action.title,
        dismissedImmediately,
        alwaysLog: true,
      },
      () => window.nvm.execute(action),
    );
    if (!dismissedImmediately)
      showActionLoadingView(
        action.title || 'Running…',
        action.subtitle || 'Waiting for the action to finish',
        'root',
      );
    const result = await resultPromise;
    if (result?.view) {
      setOptionsFor(null);
      setPreviewFor(null);
      if (result.view.aiChat) await openAiChat(result.view, 'root');
      else showExtensionView(result.view, 'root');
    } else if (!dismissedImmediately) {
      window.nvm.hide();
    }
  }

  function acceleratorFromEvent(event: React.KeyboardEvent) {
    return acceleratorFromKeyboardEvent(event.nativeEvent);
  }

  function modifierAcceleratorFromEvent(event: React.KeyboardEvent) {
    const parts = [];
    if (event.metaKey) parts.push('Command');
    if (event.ctrlKey) parts.push('Control');
    if (event.altKey) parts.push('Alt');
    if (event.shiftKey) parts.push('Shift');
    return parts.join('+');
  }

  async function refreshShortcuts() {
    setShortcutRecords(
      await measureDebugPerformance(
        'shortcuts.refresh',
        { alwaysLog: true },
        () => window.nvm.getShortcuts(),
      ),
    );
  }

  async function openShortcutManager() {
    await refreshShortcuts();
    setShortcutManagerOpen(true);
    setShortcutFor(null);
    setOptionsFor(null);
    setPreviewFor(null);
  }

  function startShortcutRecorder(action: Action) {
    setShortcutFor(action);
    setRecordedShortcut('');
  }

  async function saveRecordedShortcut() {
    if (!shortcutFor || !recordedShortcut) return;
    if (shortcutFor.id === PALETTE_HOTKEY_ACTION_ID) {
      const result = await window.nvm.setPaletteHotkey(recordedShortcut);
      showToast(result.message, result.ok ? 'default' : 'error');
      if (!result.ok && !result.spotlightConflict) return;
      setShortcutFor(null);
      setOptionsFor(null);
      const refreshed = await window.nvm.execute(SETTINGS_ROOT_ACTION);
      if (refreshed?.view) showExtensionView(refreshed.view, 'replace');
      if (result.spotlightConflict)
        showExtensionView(spotlightConflictView(recordedShortcut), 'push');
      return;
    }
    if (shortcutFor.id === HYPER_KEY_ACTION_ID) {
      const result = (await window.nvm.runViewAction({
        type: 'setSettingShortcut',
        title: 'Save Hyper Key',
        settingId: 'hyperKey',
        shortcut: recordedShortcut,
      })) as any;
      showToast(
        result?.toast?.message || 'Hyper key saved',
        result?.ok ? 'default' : 'error',
      );
      if (!result?.ok) return;
      setShortcutLabelHyperKey(recordedShortcut);
      setShortcutFor(null);
      setOptionsFor(null);
      setRefreshNonce((nonce) => nonce + 1);
      const refreshed = await window.nvm.execute(SETTINGS_ROOT_ACTION);
      if (refreshed?.view) showExtensionView(refreshed.view, 'replace');
      return;
    }
    const result = (await window.nvm.runViewAction({
      type: 'setActionShortcut',
      title: 'Save Shortcut',
      targetAction: shortcutFor,
      accelerator: recordedShortcut,
    })) as any;
    if (!result?.ok) return;
    setShortcutFor(null);
    setOptionsFor(null);
    setRefreshNonce((nonce) => nonce + 1);
    if (shortcutManagerOpen) await refreshShortcuts();
  }

  async function removeShortcut(record: ShortcutRecord) {
    const result = await window.nvm.removeShortcut(record.actionId);
    showToast(result.message, result.ok ? 'default' : 'error');
    if (result.ok) {
      setShortcutOptionsFor(null);
      await refreshShortcuts();
    }
  }

  async function setAlias() {
    if (!optionsFor) return;
    setAliasFor(optionsFor);
    setChildQuery('');
    setOptionsFor(null);
  }

  async function submitAlias() {
    if (!aliasFor) return;
    const alias = childQuery.trim();
    if (!alias) return;
    const result = (await window.nvm.runViewAction({
      type: 'setActionAlias',
      title: 'Save Alias',
      targetAction: aliasFor,
      alias,
    })) as any;
    if (!result?.ok) return;
    const current = aliasFor.userAliases || [];
    const userAliases = current.includes(alias) ? current : [...current, alias];
    setAliasFor({ ...aliasFor, userAliases });
    setChildQuery('');
  }

  async function removeAliasEntry(alias: string) {
    if (!aliasFor) return;
    const result = (await window.nvm.runViewAction({
      type: 'removeActionAlias',
      title: 'Remove Alias',
      targetAction: aliasFor,
      alias,
    })) as any;
    if (!result?.ok) return;
    const userAliases = (aliasFor.userAliases || []).filter(
      (value) => value !== alias,
    );
    setAliasFor({ ...aliasFor, userAliases });
  }

  async function setShortcut() {
    if (!optionsFor) return;
    startShortcutRecorder(optionsFor);
  }

  async function removeOptionsShortcut() {
    if (!optionsFor?.id) return;
    const result = await window.nvm.removeShortcut(optionsFor.id);
    showToast(result.message, result.ok ? 'default' : 'error');
    if (!result.ok) return;
    setOptionsFor(null);
    setRefreshNonce((nonce) => nonce + 1);
  }

  async function quickLookOptionsAction() {
    if (!optionsFor?.filePath) return;
    await window.nvm.runViewAction({
      type: 'quickLook',
      title: 'Preview File',
      path: optionsFor.filePath,
    });
  }

  async function setOverride() {
    if (!optionsFor) return;
    const instruction = window.prompt(
      `How should AI override “${optionsFor.title}”?`,
      optionsFor.overrideSummary || '',
    );
    if (!instruction?.trim()) return;
    const result = await window.nvm.setOverride(optionsFor, instruction);
    showToast(result.message, result.ok ? 'default' : 'error');
    if (result.ok) setOptionsFor(null);
  }

  async function tweakActionWithAi(action: Action | null | undefined) {
    if (!action?.extensionFile) return;
    const result = await window.nvm.tweakExtension({
      extensionFile: action.extensionFile,
      title: action.title,
    });
    if (result?.view) {
      setOptionsFor(null);
      await openAiChat(result.view, 'root');
    }
  }

  async function tweakWithAi() {
    await tweakActionWithAi(optionsFor);
  }

  async function renameExtensionAction() {
    if (!optionsFor) return;
    setOptionsFor(null);
    await runViewAction({
      type: 'renameExtensionPrompt',
      title: 'Rename Extension',
      targetAction: optionsFor,
    });
  }

  function tabActionForRootAction(action: Action | null | undefined) {
    if (!action) return null;
    if (
      action.kind === 'extension-root-item' &&
      action.extensionId === 'nevermind.ai-builder' &&
      action.id.startsWith('extension-root:nevermind.ai-builder:ai-chat:')
    )
      return () => run(action);
    if (
      ['extension-root-item', 'extension-action'].includes(action.kind) &&
      action.extensionFile
    )
      return () => tweakActionWithAi(action);
    return null;
  }

  async function restoreOriginal() {
    if (!optionsFor) return;
    const result = await window.nvm.runViewAction({
      type: 'clearActionOverride',
      title: 'Restore Original',
      targetAction: optionsFor,
    });
    if ((result as any)?.ok) setOptionsFor(null);
  }

  async function duplicateCreatedAction() {
    if (!optionsFor) return;
    const result = (await window.nvm.runViewAction({
      type: 'duplicateCreatedAction',
      title: 'Duplicate Action',
      targetAction: optionsFor,
    })) as any;
    if (!result?.action) return;
    setOptionsFor(null);
    setRefreshNonce((nonce) => nonce + 1);
    await tweakActionWithAi(result.action);
  }

  function askRemoveCreatedAction() {
    if (!optionsFor) return;
    setConfirmRemoveFor(optionsFor);
  }

  async function confirmRemoveCreatedAction() {
    if (!confirmRemoveFor) return;
    const result = (await window.nvm.runViewAction({
      type: 'removeCreatedAction',
      title: 'Remove Action',
      targetAction: confirmRemoveFor,
    })) as any;
    if (result?.ok) {
      setConfirmRemoveFor(null);
      setOptionsFor(null);
      setRefreshNonce((nonce) => nonce + 1);
    }
  }

  const canOverride = Boolean(optionsFor?.defaultActionId);
  const canTweakWithAi = Boolean(
    optionsFor?.extensionFile &&
      ['extension-root-item', 'extension-action'].includes(optionsFor.kind),
  );
  const canRemoveCreatedAction = Boolean(
    optionsFor?.kind === 'ai-chat' || optionsFor?.removable,
  );
  const canDuplicateCreatedAction = Boolean(
    ['extension-root-item', 'extension-action'].includes(
      optionsFor?.kind || '',
    ) && optionsFor?.removable,
  );
  const canCustomizeAction = canCustomizeCommandAction(optionsFor);
  const canRemoveOptionsShortcut = Boolean(
    optionsFor &&
      shortcutRecords.some((record) => record.actionId === optionsFor.id),
  );
  const canPreviewAction = Boolean(
    optionsFor?.imageDataUrl ||
      optionsFor?.videoUrl ||
      optionsFor?.thumbnailUrl ||
      optionsFor?.text ||
      optionsFor?.filePath,
  );
  const canQuickLookAction = Boolean(optionsFor?.filePath);
  const selectedExtensionItem = useMemo(() => {
    if (!extensionView) return null;
    return (
      allViewItems(extensionView).find((item) => item.id === selectedValue) ||
      null
    );
  }, [extensionView, selectedValue]);
  const selectedShortcutRecord = useMemo(
    () =>
      shortcutRecords.find(
        (record) => `shortcut:${record.actionId}` === selectedValue,
      ) || null,
    [selectedValue, shortcutRecords],
  );
  const activeAction = optionsFor || previewFor || selectedAction;

  useEffect(() => {
    if (
      !extensionView?.onSelectionChange ||
      !selectedValue ||
      !allViewItems(extensionView).some((item) => item.id === selectedValue)
    )
      return;
    runViewAction({
      ...extensionView.onSelectionChange,
      selectedItemId: selectedValue,
      text: selectedValue,
    });
  }, [extensionView?.onSelectionChange, selectedValue]);

  function childMatches(...values: Array<string | undefined>) {
    return valuesMatch(childQuery, ...values);
  }

  function filterViewSections(view: ExtensionView) {
    const minScore = view.id === 'clipboard-history' ? 50 : undefined;
    return measureDebugPerformanceSync(
      'view.filter-sections',
      {
        childQueryLength: childQuery.length,
        sectionCount: view.sections?.length || 0,
      },
      () =>
        filterCommandSections(
          view,
          childQuery,
          minScore ? { minScore } : undefined,
        ),
    );
  }

  function filterExtensionItems(items: ExtensionViewItem[] = []) {
    // Filterable child views (list/grid) use a minimum score of 50 to
    // require exact, starts-with, or contains-substring matches. This
    // prevents long strings like file paths from causing false-positive
    // sequential-character matches (score 20) that would show every item.
    const minScore =
      extensionView?.id === 'clipboard-history' ||
      extensionView?.type === 'list' ||
      extensionView?.type === 'grid'
        ? 50
        : undefined;
    return measureDebugPerformanceSync(
      'view.filter-items',
      { childQueryLength: childQuery.length, itemCount: items.length },
      () =>
        filterCommandItems(
          items,
          childQuery,
          minScore ? { minScore } : undefined,
        ),
    );
  }

  function getAliasActionRows() {
    const draft = childQuery.trim();
    const existing = aliasFor?.userAliases || [];
    const rows: ActionPanelRow[] = [];
    if (draft && !existing.includes(draft)) {
      rows.push({
        value: 'alias:save',
        icon: <Tag size={18} />,
        title: `Save alias “${draft}”`,
        subtitle: aliasFor
          ? `Make “${aliasFor.title}” appear for this phrase`
          : '',
        onSelect: submitAlias,
      });
    }
    for (const alias of existing) {
      rows.push({
        value: `alias:remove:${alias}`,
        icon: <Trash2 size={18} />,
        title: alias,
        subtitle: 'Remove this alias',
        onSelect: () => removeAliasEntry(alias),
      });
    }
    rows.push({
      value: 'alias:done',
      icon: <RotateCcw size={18} />,
      title: 'Done',
      subtitle: existing.length
        ? `${existing.length} alias${existing.length === 1 ? '' : 'es'} saved`
        : 'Close without changes',
      onSelect: () => {
        setAliasFor(null);
        setChildQuery('');
      },
    });
    return rows;
  }

  function getConfirmActionRows() {
    return [
      {
        value: 'confirm:remove',
        icon: <Trash2 size={18} />,
        title: 'Remove action',
        subtitle: confirmRemoveFor
          ? `Delete “${confirmRemoveFor.title}” from Nevermind`
          : '',
        onSelect: confirmRemoveCreatedAction,
        className: 'result dangerResult',
      },
      {
        value: 'confirm:cancel',
        icon: <RotateCcw size={18} />,
        title: 'Cancel',
        subtitle: 'Keep this action',
        onSelect: () => setConfirmRemoveFor(null),
        className: 'result',
      },
    ].filter((row) => childMatches(row.title, row.subtitle));
  }

  function getConfirmViewActionRows() {
    const action = confirmViewActionFor;
    return [
      {
        value: 'confirm:view-action',
        icon:
          action?.style === 'destructive' ? (
            <Trash2 size={18} />
          ) : (
            <Zap size={18} />
          ),
        title: action?.confirmLabel || action?.title || 'Run action',
        subtitle: action?.confirmMessage || 'Confirm this action',
        onSelect: async () => {
          if (!action) return;
          setConfirmViewActionFor(null);
          await runViewAction({ ...action, requiresConfirmation: false }, true);
        },
        className:
          action?.style === 'destructive' ? 'result dangerResult' : 'result',
      },
      {
        value: 'confirm:view-cancel',
        icon: <RotateCcw size={18} />,
        title: action?.cancelLabel || 'Cancel',
        subtitle: 'Do nothing',
        onSelect: () => setConfirmViewActionFor(null),
        className: 'result',
      },
    ].filter((row) => childMatches(row.title, row.subtitle));
  }

  function actionIdentity(action: ExtensionViewAction) {
    const nativeId = (action.nativeAction as { id?: string } | undefined)?.id;
    return [
      action.type,
      action.handlerId,
      nativeId,
      action.title,
      action.path,
      action.url,
      action.text,
    ]
      .filter(Boolean)
      .join('\u0000');
  }

  function closeActionPanels() {
    setActionSubmenuFor(null);
    setExtensionItemOptionsFor(null);
    setOptionsFor(null);
    setChildQuery('');
  }

  function actionRow(
    action: ExtensionViewAction,
    value: string,
    closeAfterSelect: boolean,
    icon = iconForAction(action),
  ): ActionPanelRow {
    return {
      value,
      icon,
      title: action.title,
      subtitle: action.submenu ? 'Open submenu' : actionDescription(action),
      shortcut: action.shortcut,
      onSelect: async () => {
        if (action.submenu) {
          setActionSubmenuFor({ title: action.title, panel: action.submenu });
          return;
        }
        await runViewAction(action);
        if (closeAfterSelect) closeActionPanels();
      },
      className:
        action.style === 'destructive' ? 'result dangerResult' : 'result',
    };
  }

  function actionPanelRows(
    panel = extensionItemOptionsFor?.actionPanel,
    fallbackActions = extensionItemOptionsFor?.actions || [],
    prefix = 'extension-item',
    closeAfterSelect = true,
    skipActionIdentities = new Set<string>(),
  ): ActionPanelRow[] {
    const sections = panel?.sections?.length
      ? panel.sections
      : [{ actions: fallbackActions }];
    return sections.flatMap((section, sectionIndex) => {
      const rows = (section.actions || [])
        .filter((action) => !skipActionIdentities.has(actionIdentity(action)))
        .map((action, index) =>
          actionRow(
            action,
            `${prefix}:${sectionIndex}:${index}:${action.type}:${action.title}`,
            closeAfterSelect,
          ),
        )
        .filter((row) => childMatches(row.title, row.subtitle));
      if (rows.length === 0) return [];
      return section.title
        ? [
            {
              value: `${prefix}:section:${sectionIndex}`,
              title: section.title,
              subtitle: '',
              sectionHeader: true,
              onSelect: () => {},
              className: 'actionSectionHeader',
            },
            ...rows,
          ]
        : rows;
    });
  }

  function commandItemActionRows(
    item: CommandItem,
    prefix: string,
    closeAfterSelect = true,
  ) {
    const primary = item.primaryAction;
    const skip = new Set<string>();
    const primaryRows =
      primary && childMatches(primary.title, actionDescription(primary))
        ? [
            actionRow(
              primary,
              `${prefix}:primary:${primary.type}:${primary.title}`,
              closeAfterSelect,
              iconForItem(item),
            ),
          ]
        : [];
    if (primary) skip.add(actionIdentity(primary));
    return [
      ...primaryRows,
      ...actionPanelRows(
        item.actionPanel,
        item.actions || [],
        prefix,
        closeAfterSelect,
        skip,
      ),
    ];
  }

  function getExtensionItemActionRows() {
    return extensionItemOptionsFor
      ? commandItemActionRows(extensionItemOptionsFor, 'extension-item')
      : [];
  }

  function getShortcutRows() {
    return shortcutItems(shortcutRecords, childMatches).map((item) => ({
      value: item.id,
      icon: <Keyboard size={18} />,
      title: item.title,
      subtitle: item.subtitle,
      onSelect: () => {
        const record = shortcutRecords.find(
          (candidate) => `shortcut:${candidate.actionId}` === item.id,
        );
        if (record) startShortcutRecorder(record.action);
      },
      className: 'result',
    }));
  }

  function renderShortcutManager() {
    return (
      <ShortcutManagerView
        records={shortcutRecords}
        matches={childMatches}
        onSelect={(record) => setShortcutOptionsFor(record as ShortcutRecord)}
      />
    );
  }

  function getShortcutOptionRows() {
    return shortcutOptionRows(
      shortcutOptionsFor as ShortcutRecordLike | null,
      (action) => {
        startShortcutRecorder(action as Action);
        setShortcutOptionsFor(null);
      },
      (record) => removeShortcut(record as ShortcutRecord),
      childMatches,
    );
  }

  function getShortcutRecorderRows() {
    return shortcutRecorderRows(
      recordedShortcut,
      shortcutFor,
      saveRecordedShortcut,
      () => setShortcutFor(null),
    );
  }

  function getRootActionRows() {
    return optionsFor
      ? commandItemActionRows(commandItemFromAction(optionsFor), 'root-action')
      : [];
  }

  function getOptionActionRows() {
    const declaredActions = actionsFromPanel(optionsFor?.actionPanel);
    const hasDeclaredPreviewAction = declaredActions.some(
      (action) => action.type === 'previewClipboardItem',
    );
    const hasDeclaredQuickLookAction = declaredActions.some(
      (action) => action.type === 'quickLook',
    );
    const optionRows = [
      {
        value: 'option:shortcut',
        icon: <Keyboard size={18} />,
        title: 'Set keyboard shortcut',
        subtitle: 'Run this action globally without opening Nevermind',
        onSelect: setShortcut,
        show: canCustomizeAction,
      },
      {
        value: 'option:remove-shortcut',
        icon: <Trash2 size={18} />,
        title: 'Remove keyboard shortcut',
        subtitle: optionsFor?.shortcut,
        onSelect: removeOptionsShortcut,
        show: canCustomizeAction && canRemoveOptionsShortcut,
        className: 'dangerResult',
      },
      {
        value: 'option:preview',
        icon: <Search size={18} />,
        title: 'Preview',
        subtitle: 'Press → to preview an item',
        shortcut: 'Command+Y',
        onSelect: () => optionsFor && setPreviewFor(optionsFor),
        show: canPreviewAction && !hasDeclaredPreviewAction,
      },
      {
        value: 'option:quick-look',
        icon: <Search size={18} />,
        title: 'Preview File',
        subtitle: 'Preview this file',
        shortcut: 'Command+Y',
        onSelect: quickLookOptionsAction,
        show: canQuickLookAction && !hasDeclaredQuickLookAction,
      },
      {
        value: 'option:alias',
        icon: <Tag size={18} />,
        title: 'Set alias',
        subtitle: 'Make this action appear for another phrase',
        onSelect: setAlias,
        show: canCustomizeAction,
      },
      {
        value: 'option:duplicate',
        icon: <Copy size={18} />,
        title: 'Duplicate',
        subtitle: 'Create a separate copy to tweak while keeping this action',
        onSelect: duplicateCreatedAction,
        show: canDuplicateCreatedAction,
      },
      {
        value: 'option:rename',
        icon: <Pencil size={18} />,
        title: 'Rename',
        subtitle: 'Change the display name of this extension',
        onSelect: renameExtensionAction,
        show: canTweakWithAi,
      },
      {
        value: 'option:tweak',
        icon: <Wand2 size={18} />,
        title: 'Tweak with AI',
        subtitle: 'Open the original creation chat for this action',
        shortcut: 'Tab',
        onSelect: tweakWithAi,
        show: canTweakWithAi,
      },
      {
        value: 'option:override',
        icon: <Sparkles size={18} />,
        title: 'Override with AI',
        subtitle: 'Customize this action to do what you want',
        onSelect: setOverride,
        show: canOverride,
      },
      {
        value: 'option:restore',
        icon: <RotateCcw size={18} />,
        title: 'Restore original',
        subtitle: 'Remove the AI override and use the built-in behavior',
        onSelect: restoreOriginal,
        show: Boolean(optionsFor?.isOverridden),
      },
      {
        value: 'option:remove',
        icon: <Trash2 size={18} />,
        title: 'Remove action',
        subtitle: 'Delete this AI-created action from Nevermind',
        onSelect: askRemoveCreatedAction,
        show: canRemoveCreatedAction,
      },
    ].filter((row) => row.show && childMatches(row.title, row.subtitle));
    return [...getRootActionRows(), ...optionRows];
  }

  function renderChildEmpty(message = EMPTY_ACTIONS_TITLE, subtitle?: string) {
    return (
      <EmptyState
        icon={<Search size={24} />}
        title={message}
        subtitle={subtitle}
      />
    );
  }

  function renderViewEmpty(view: ExtensionView, fallback = EMPTY_ITEMS_TITLE) {
    return renderChildEmpty(
      view.emptyView?.title || fallback,
      view.emptyView?.subtitle,
    );
  }

  function runningAppClassName(action: Action) {
    const appPath = appPathForRunningStatus(action);
    return appPath && runningAppPaths[appPath] ? 'runningAppResult' : undefined;
  }

  function commandItemFromAction(action: Action): CommandItem {
    return {
      id: action.id,
      title: action.title,
      subtitle: action.isOverridden
        ? `AI override: ${action.overrideSummary}`
        : action.subtitle,
      icon: action.icon,
      image:
        action.thumbnailUrl ||
        action.iconUrl ||
        iconUrls[action.id] ||
        iconUrls[appPathForIcon(action) || ''] ||
        undefined,
      appearance: action.appearance,
      className: runningAppClassName(action),
      primaryAction: {
        type: 'nativeAction',
        title: action.title,
        shortcut: action.shortcut,
        nativeAction: action,
      },
      actionPanel:
        action.actionPanel ||
        actionPanelFromActions([
          {
            type: 'nativeAction',
            title: action.title,
            shortcut: action.shortcut,
            nativeAction: action,
          },
        ]),
    };
  }

  function primaryCommandAction(item: CommandItem) {
    return (
      item.primaryAction ||
      actionsFromPanel(item.actionPanel, item.actions || [])[0]
    );
  }

  function actionFromCommandItem(item: CommandItem) {
    return primaryCommandAction(item)?.nativeAction as Action | undefined;
  }

  async function runCommandItem(item: CommandItem) {
    const action = primaryCommandAction(item);
    if (!action) return;
    if (!isChildOpen) rememberRootQuery();
    await runViewAction(action);
  }

  function iconForCommandItem(item: CommandItem) {
    const action = actionFromCommandItem(item);
    return iconForItem({
      ...item,
      icon: (action?.icon || item.icon) as CommandIconName,
      image: item.image,
    });
  }

  function renderActionResults() {
    const items = measureDebugPerformanceSync(
      'root-actions.to-command-items',
      { actionCount: actions.length },
      () => actions.map(commandItemFromAction),
    );
    return (
      <RootCommandList
        items={items}
        iconForItem={iconForCommandItem}
        onSelect={runCommandItem}
        extraForItem={(item) =>
          ['extension-root-item', 'extension-action'].includes(
            actionFromCommandItem(item)?.kind || '',
          ) && actionFromCommandItem(item)?.extensionFile
            ? ['Tab tweak']
            : []
        }
      />
    );
  }

  function renderActionPanel(
    rows: ActionPanelRow[] | unknown[],
    emptyMessage = EMPTY_ACTIONS_TITLE,
  ) {
    return (
      <ActionPanel
        rows={rows as ActionPanelRow[]}
        emptyMessage={emptyMessage}
      />
    );
  }

  function renderMarkdown(content: string) {
    markDebugPerformance('markdown.render', { contentLength: content.length });
    return <MarkdownContent content={content} />;
  }

  function runDefaultViewAction(item: ExtensionViewItem) {
    const action =
      item.primaryAction ||
      actionsFromPanel(item.actionPanel, item.actions || [])[0];
    if (action) runViewAction(action);
  }

  function appPathForIcon(action: Action | null | undefined) {
    const candidate = action?.app?.path || action?.rootAction?.path;
    return candidate?.endsWith('.app') ? candidate : null;
  }

  function appPathForRunningStatus(action: Action | null | undefined) {
    const candidate =
      action?.extensionId === 'nevermind.apps'
        ? action.rootAction?.path
        : action?.app?.path;
    return typeof candidate === 'string' && candidate ? candidate : null;
  }

  function diskPathForAction(action: Action | null | undefined) {
    return (
      action?.filePath || action?.app?.path || action?.rootAction?.path || null
    );
  }

  function diskPathForItem(item: ExtensionViewItem | null | undefined) {
    if (!item) return null;
    if (item.path || item.filePath) return item.path || item.filePath;
    const actions = [
      item.primaryAction,
      ...actionsFromPanel(item.actionPanel, item.actions || []),
    ].filter(Boolean) as ExtensionViewAction[];
    return actions.find((action) => action.path)?.path || null;
  }

  function hydrateExtensionItemIcon(item: ExtensionViewItem) {
    if (item.image) return item;
    const appPath = diskPathForItem(item);
    const iconUrl = appPath?.endsWith('.app') ? iconUrls[appPath] : null;
    return iconUrl ? { ...item, image: iconUrl } : item;
  }

  function dragPathForItem(item: ExtensionViewItem) {
    return diskPathForItem(item);
  }

  function startItemDrag(event: React.DragEvent, item: ExtensionViewItem) {
    const filePath = dragPathForItem(item);
    if (!filePath) return;
    event.preventDefault();
    window.nvm.startFileDrag(filePath);
  }

  function persistentActionForItem(item: ExtensionViewItem | null | undefined) {
    return item?.persistentAction as Action | undefined;
  }

  function itemActionPanelIsVisible(
    item: ExtensionViewItem | null | undefined,
  ) {
    return (
      Boolean(persistentActionForItem(item)) ||
      item?.actionPanelVisibility !== 'hidden'
    );
  }

  function viewActionPanelIsVisible(view: ExtensionView | null | undefined) {
    return view?.actionPanelVisibility !== 'hidden';
  }

  function viewActionPanel(view: ExtensionView) {
    return view.actionPanel || actionPanelFromActions(view.actions || []);
  }

  function renderSearchAccessory(view: ExtensionView | null) {
    if (!view?.searchAccessory?.items?.length) return null;
    return (
      <SearchAccessory
        tooltip={view.searchAccessory.tooltip}
        value={view.searchAccessory.value}
        items={view.searchAccessory.items}
        onChange={(value) => {
          const action = view.searchAccessory?.onChange;
          if (action) runViewAction({ ...action, value, text: value });
        }}
      />
    );
  }

  function renderExtensionView(view: ExtensionView) {
    return (
      <ExtensionViewRenderer
        view={view}
        aiChat={aiChat}
        nevermindAuthed={nevermindAuthed}
        onSignInToNevermind={async () => {
          const result = await window.nvm.signInToNevermind();
          if (result.ok) setNevermindAuthed(true);
          else
            setToast({
              message: `Sign-in failed: ${result.error || 'unknown'}`,
              tone: 'error',
            });
        }}
        formValues={formValues}
        setFormValues={setFormValues}
        filterItems={(items) =>
          filterExtensionItems(items).map(hydrateExtensionItemIcon)
        }
        filterSections={filterViewSections}
        renderMarkdown={renderMarkdown}
        renderActionPanel={renderActionPanel}
        actionPanelRows={actionPanelRows}
        renderRootIcon={iconForCommandItem}
        renderEmpty={renderViewEmpty}
        runDefaultAction={runDefaultViewAction}
        runAction={runViewAction}
        sendAiPrompt={(message) => sendAiPrompt(message, view.chatId)}
        abortAiChat={window.nvm.abortAiChat}
        dragPathForItem={dragPathForItem}
        startItemDrag={startItemDrag}
        selectedItemId={selectedValue}
      />
    );
  }

  function runLocalShortcut(accelerator: string) {
    if (
      !extensionView ||
      extensionItemOptionsFor ||
      optionsFor ||
      previewFor ||
      confirmRemoveFor ||
      shortcutManagerOpen ||
      shortcutFor
    )
      return false;
    const normalized = normalizedShortcut(accelerator);
    const selectedItem = selectedExtensionItem;
    const itemActions = selectedItem
      ? actionsFromPanel(selectedItem.actionPanel, selectedItem.actions || [])
      : [];
    const viewActions = actionsFromPanel(
      extensionView.actionPanel,
      extensionView.actions || [],
    );
    const actions = selectedItem
      ? ([selectedItem.primaryAction, ...itemActions].filter(
          Boolean,
        ) as ExtensionViewAction[])
      : viewActions;
    const action = actions.find(
      (item) => normalizedShortcut(item.shortcut) === normalized,
    );
    if (!action) return false;
    runViewAction(action);
    return true;
  }

  function rootSearchQueryAction(
    action: Action | null | undefined,
    title: 'Continue Calculation' | 'Swap Units',
  ) {
    return actionsFromPanel(action?.actionPanel, []).find(
      (candidate) =>
        candidate.type === 'setSearchQuery' && candidate.title === title,
    );
  }

  function moveGridSelection(key: string) {
    if (
      extensionView?.type !== 'grid' ||
      confirmRemoveFor ||
      extensionItemOptionsFor ||
      optionsFor ||
      previewFor
    )
      return false;
    const items = filterExtensionItems(allViewItems(extensionView));
    if (items.length === 0) return false;
    const currentIndex = Math.max(
      0,
      items.findIndex((item) => item.id === selectedValue),
    );
    const grid = document.querySelector<HTMLElement>('.extensionGrid');
    const columns = grid
      ? Math.max(
          1,
          getComputedStyle(grid).gridTemplateColumns.split(' ').filter(Boolean)
            .length,
        )
      : 4;
    const delta =
      key === 'ArrowRight'
        ? 1
        : key === 'ArrowLeft'
          ? -1
          : key === 'ArrowDown'
            ? columns
            : -columns;
    const nextIndex = Math.max(
      0,
      Math.min(items.length - 1, currentIndex + delta),
    );
    const next = items[nextIndex];
    if (!next) return false;
    selectValue(next.id);
    requestAnimationFrame(() =>
      document
        .querySelector(`[data-extension-item-id="${CSS.escape(next.id)}"]`)
        ?.scrollIntoView({ block: 'nearest', inline: 'nearest' }),
    );
    return true;
  }

  function selectedDiskPath() {
    if (!isChildOpen) return diskPathForAction(selectedAction);
    if (
      selectedExtensionItem &&
      !confirmRemoveFor &&
      !extensionItemOptionsFor &&
      !optionsFor &&
      !previewFor
    )
      return diskPathForItem(selectedExtensionItem);
    return null;
  }

  async function revealSelectedDiskItem(path: string) {
    await runViewAction({
      type: 'revealPath',
      title: 'Reveal in File Manager',
      path,
    });
  }

  function dismissFromEmptyWindowSpace(event: MouseEvent<HTMLElement>) {
    if (event.button !== 0) return;
    const target = event.target instanceof Element ? event.target : null;
    if (target?.closest('.card, .toast')) return;
    event.preventDefault();
    void window.nvm.hide();
  }

  function onCommandKeyDown(event: React.KeyboardEvent) {
    if (shortcutFor) {
      event.preventDefault();
      event.stopPropagation();
      if (event.key === 'Escape') {
        setShortcutFor(null);
        setRecordedShortcut('');
        return;
      }
      if (event.key === 'Enter') {
        saveRecordedShortcut();
        return;
      }
      if (event.key === 'Backspace' || event.key === 'Delete') {
        setRecordedShortcut('');
        return;
      }
      const accelerator =
        shortcutFor.id === HYPER_KEY_ACTION_ID
          ? acceleratorFromEvent(event) || modifierAcceleratorFromEvent(event)
          : acceleratorFromEvent(event);
      if (accelerator) setRecordedShortcut(accelerator);
      return;
    }

    const localAccelerator = acceleratorFromEvent(event);
    if (normalizedShortcut(localAccelerator) === 'command+enter') {
      const path = selectedDiskPath();
      if (path) {
        event.preventDefault();
        revealSelectedDiskItem(path);
        return;
      }
    }
    if (localAccelerator && runLocalShortcut(localAccelerator)) {
      event.preventDefault();
      return;
    }
    if (
      !isChildOpen &&
      selectedAction &&
      normalizedShortcut(localAccelerator) === 'command+enter'
    ) {
      const action = rootSearchQueryAction(
        selectedAction,
        'Continue Calculation',
      );
      if (action) {
        event.preventDefault();
        runViewAction(action);
        return;
      }
    }
    if (
      !isChildOpen &&
      selectedAction &&
      normalizedShortcut(localAccelerator) === 'command+shift+enter'
    ) {
      const action =
        rootSearchQueryAction(selectedAction, 'Swap Units') ||
        rootSearchQueryAction(selectedAction, 'Continue Calculation');
      if (action) {
        event.preventDefault();
        runViewAction(action);
        return;
      }
    }
    if (
      !isChildOpen &&
      normalizedShortcut(localAccelerator) === 'command+y' &&
      selectedAction &&
      (selectedAction.imageDataUrl ||
        selectedAction.videoUrl ||
        selectedAction.text)
    ) {
      event.preventDefault();
      setPreviewFor(selectedAction);
      setOptionsFor(null);
      return;
    }
    if (
      !isChildOpen &&
      normalizedShortcut(localAccelerator) === 'command+y' &&
      selectedAction?.filePath
    ) {
      event.preventDefault();
      runViewAction({
        type: 'quickLook',
        title: 'Preview File',
        path: selectedAction.filePath,
      });
      return;
    }

    if (
      !isChildOpen &&
      event.key === 'ArrowUp' &&
      navigateRootQueryHistory(-1)
    ) {
      event.preventDefault();
      return;
    }
    if (
      !isChildOpen &&
      event.key === 'ArrowDown' &&
      queryHistoryIndexRef.current !== null &&
      navigateRootQueryHistory(1)
    ) {
      event.preventDefault();
      return;
    }

    if (
      ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key) &&
      moveGridSelection(event.key)
    ) {
      event.preventDefault();
      return;
    }

    if (event.key === 'Escape') {
      event.preventDefault();
      if (shortcutOptionsFor) setShortcutOptionsFor(null);
      else if (shortcutManagerOpen) setShortcutManagerOpen(false);
      else if (aliasFor) {
        setAliasFor(null);
        setChildQuery('');
      } else if (confirmRemoveFor) setConfirmRemoveFor(null);
      else if (confirmViewActionFor) setConfirmViewActionFor(null);
      else if (actionSubmenuFor) setActionSubmenuFor(null);
      else if (extensionItemOptionsFor) setExtensionItemOptionsFor(null);
      else if (optionsFor) setOptionsFor(null);
      else if (previewFor) setPreviewFor(null);
      else if (extensionView) popExtensionView();
      else window.nvm.hide();
      return;
    }

    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      if (
        selectedShortcutRecord &&
        shortcutManagerOpen &&
        !shortcutOptionsFor
      ) {
        setShortcutOptionsFor(selectedShortcutRecord);
        return;
      }
      if (
        selectedExtensionItem &&
        itemActionPanelIsVisible(selectedExtensionItem) &&
        extensionView &&
        !confirmRemoveFor &&
        !confirmViewActionFor &&
        !extensionItemOptionsFor &&
        !optionsFor &&
        !previewFor
      ) {
        setChildQuery('');
        const persistentAction = persistentActionForItem(selectedExtensionItem);
        if (persistentAction) setOptionsFor(persistentAction);
        else setExtensionItemOptionsFor(selectedExtensionItem);
        return;
      }
      if (
        !selectedExtensionItem &&
        extensionView &&
        viewActionPanelIsVisible(extensionView) &&
        actionsFromPanel(extensionView.actionPanel, extensionView.actions || [])
          .length > 0 &&
        !confirmRemoveFor &&
        !confirmViewActionFor &&
        !extensionItemOptionsFor &&
        !optionsFor &&
        !previewFor
      ) {
        setChildQuery('');
        setActionSubmenuFor({
          title: extensionView.title,
          panel: viewActionPanel(extensionView),
        });
        return;
      }
      if (
        activeAction &&
        !shortcutManagerOpen &&
        !confirmRemoveFor &&
        !confirmViewActionFor &&
        !extensionItemOptionsFor &&
        !optionsFor &&
        !extensionView
      ) {
        setPreviewFor(null);
        setOptionsFor(activeAction);
      }
      return;
    }

    if (event.key === 'ArrowLeft' && isChildOpen) {
      const input =
        event.target instanceof HTMLInputElement ? event.target : null;
      if (input && input.selectionStart !== 0) return;
      if (event.target instanceof HTMLTextAreaElement) return;
      event.preventDefault();
      if (shortcutOptionsFor) setShortcutOptionsFor(null);
      else if (shortcutManagerOpen) setShortcutManagerOpen(false);
      else if (aliasFor) {
        setAliasFor(null);
        setChildQuery('');
      } else if (confirmRemoveFor) setConfirmRemoveFor(null);
      else if (confirmViewActionFor) setConfirmViewActionFor(null);
      else if (actionSubmenuFor) setActionSubmenuFor(null);
      else if (extensionItemOptionsFor) setExtensionItemOptionsFor(null);
      else if (optionsFor) setOptionsFor(null);
      else if (previewFor) setPreviewFor(null);
      else if (extensionView) popExtensionView();
      return;
    }

    if (event.key === 'ArrowRight') {
      const input =
        event.target instanceof HTMLInputElement ? event.target : null;
      if (input && input.selectionStart !== input.value.length) return;
      if (event.target instanceof HTMLTextAreaElement) return;
      if (
        !isChildOpen &&
        selectedAction &&
        (selectedAction.imageDataUrl ||
          selectedAction.videoUrl ||
          selectedAction.thumbnailUrl ||
          selectedAction.text)
      ) {
        event.preventDefault();
        setPreviewFor(selectedAction);
        setOptionsFor(null);
        return;
      }
      if (
        selectedExtensionItem &&
        itemActionPanelIsVisible(selectedExtensionItem) &&
        extensionView &&
        !confirmRemoveFor &&
        !confirmViewActionFor &&
        !extensionItemOptionsFor &&
        !optionsFor &&
        !previewFor
      ) {
        event.preventDefault();
        setChildQuery('');
        const persistentAction = persistentActionForItem(selectedExtensionItem);
        if (persistentAction) setOptionsFor(persistentAction);
        else setExtensionItemOptionsFor(selectedExtensionItem);
        return;
      }
      if (
        activeAction &&
        !shortcutManagerOpen &&
        !confirmRemoveFor &&
        !confirmViewActionFor &&
        !extensionItemOptionsFor &&
        !optionsFor &&
        !extensionView
      ) {
        event.preventDefault();
        setPreviewFor(null);
        setOptionsFor(activeAction);
        return;
      }
    }

    if (!isChildOpen && event.key === 'Tab') {
      const tabAction = tabActionForRootAction(selectedAction);
      if (tabAction) {
        event.preventDefault();
        tabAction();
        return;
      }
    }

    if (!isChildOpen && event.key === 'Tab' && query) {
      event.preventDefault();
      rememberRootQuery(query);
      if (createAction) {
        run(createAction);
      } else {
        startBuilderChatFromQuery(query);
      }
    }
  }

  return (
    <main className="shell" onMouseDown={dismissFromEmptyWindowSpace}>
      {toast ? <Toast message={toast.message} tone={toast.tone} /> : null}
      <Command
        ref={paletteRef}
        className={`palette ${isVisuallyStacked ? 'isStacked' : ''}`}
        label="Nevermind"
        loop
        shouldFilter={false}
        value={selectedValue}
        onValueChange={selectValue}
        onKeyDown={onCommandKeyDown}
      >
        <div
          className={`searchRow card searchCard ${isVisuallyStacked ? 'stackParentCard' : ''}`}
        >
          <Zap className="brandIcon" size={22} />
          <Command.Input
            ref={inputRef}
            value={inputValue}
            onValueChange={(value) => {
              if (shortcutFor) return;
              if (isFilterableChildOpen) setChildQuery(value);
              else if (!isChildOpen) setRootQuery(value);
            }}
            placeholder={placeholder}
            readOnly={!shortcutFor && !isFilterableChildOpen && isChildOpen}
            spellCheck={false}
          />
          {renderSearchAccessory(extensionView)}
          {!isChildOpen && query ? (
            <div className="tabHint">
              <Sparkles size={12} aria-hidden="true" /> <kbd>Tab</kbd> to
              automate
            </div>
          ) : null}
        </div>

        {siblingViews.map((sib, index) => (
          <div
            key={`sibling-${index}-${sib.id || sib.title}`}
            className={`siblingPane card inertSibling siblingPane-${sib.type} ${sib.aiChat ? 'interactiveSibling siblingPane-aiChat' : ''}`}
            aria-hidden={sib.aiChat ? undefined : true}
          >
            <div className="siblingHeader">{sib.title}</div>
            <div className="siblingBody">{renderExtensionView(sib)}</div>
          </div>
        ))}

        <Command.List
          ref={resultsListRef}
          className={`results card ${isVisuallyStacked ? 'optionsCard' : 'resultsCard'} ${isLargeExtensionView ? 'largeResultsCard' : ''} ${extensionView?.isLoading ? 'loadingBorder' : ''}`}
        >
          {shortcutFor ? (
            <div className="shortcutRecorder">
              <div className="shortcutKeys">
                {(recordedShortcut
                  ? recordedShortcut.split('+')
                  : ['Press keys']
                ).map((part) => (
                  <kbd key={part}>{part}</kbd>
                ))}
              </div>
              {renderActionPanel(getShortcutRecorderRows())}
            </div>
          ) : shortcutOptionsFor ? (
            renderActionPanel(getShortcutOptionRows())
          ) : shortcutManagerOpen ? (
            renderShortcutManager()
          ) : aliasFor ? (
            renderActionPanel(getAliasActionRows())
          ) : confirmRemoveFor ? (
            renderActionPanel(getConfirmActionRows())
          ) : confirmViewActionFor ? (
            renderActionPanel(getConfirmViewActionRows())
          ) : actionSubmenuFor ? (
            renderActionPanel(
              actionPanelRows(
                actionSubmenuFor.panel,
                [],
                'action-submenu',
                true,
              ),
            )
          ) : extensionItemOptionsFor ? (
            renderActionPanel(getExtensionItemActionRows())
          ) : optionsFor ? (
            renderActionPanel(getOptionActionRows())
          ) : previewFor ? (
            <div className="previewPane">
              {previewFor.videoUrl ? (
                <video
                  className="previewImage"
                  src={previewFor.videoUrl}
                  controls
                  autoPlay
                  muted
                  loop
                  playsInline
                />
              ) : previewFor.imageDataUrl ? (
                <img
                  className="previewImage"
                  src={previewFor.imageDataUrl}
                  alt="Clipboard preview"
                />
              ) : previewFor.thumbnailUrl ? (
                <img
                  className="previewImage"
                  src={previewFor.thumbnailUrl}
                  alt={previewFor.title}
                />
              ) : previewFor.text ? (
                <pre className="previewText">{previewFor.text}</pre>
              ) : (
                <div className="previewDetails">
                  <strong>{previewFor.title}</strong>
                  <span>{previewFor.subtitle}</span>
                </div>
              )}
            </div>
          ) : extensionView ? (
            renderExtensionView(extensionView)
          ) : (
            renderActionResults()
          )}
        </Command.List>
      </Command>
    </main>
  );
}

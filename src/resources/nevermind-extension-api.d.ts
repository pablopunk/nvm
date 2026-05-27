/**
 * Nevermind Extension API
 *
 * This declaration file is the canonical public contract for generated Nevermind
 * extensions and the API reference returned by the AI builder's `read_extension_api`
 * tool. Keep extension-author guidance here, next to the types the host copies into
 * the user extension directory for validation.
 *
 * Extensions are local TypeScript files that use erasable TypeScript only: type
 * imports, interfaces, generics, and `satisfies` are fine; runtime TypeScript
 * features such as enums, decorators, parameter properties, and value namespaces
 * are not supported by Electron's native type stripping.
 *
 * @example
 * ```ts
 * import type { NevermindExtension } from './nevermind-extension-api'
 *
 * export default {
 *   id: 'my.images',
 *   title: 'My Images',
 *   permissions: ['desktop.files'],
 *   commands: [{
 *     id: 'recent-images',
 *     title: 'Recent Images',
 *     icon: 'image',
 *     async run(ctx) {
 *       const files = await ctx.desktop.files?.findImages(['~/Downloads'], { sortBy: 'added', limit: 48 }) || []
 *       return ctx.ui.grid({
 *         title: 'Recent Images',
 *         items: files.map((file) => ({
 *           id: file.path,
 *           title: file.name,
 *           subtitle: file.displayPath,
 *           image: file.url,
 *           primaryAction: ctx.actions.quickLook(file.path),
 *           actions: [ctx.actions.revealPath(file.path), ctx.actions.copyText(file.path, 'Copy Path')],
 *         })),
 *       })
 *     },
 *   }],
 * } satisfies NevermindExtension
 * ```
 */

/** Capabilities an extension must declare before the host exposes matching `ctx.*` surfaces. */
export type ExtensionPermission =
  | 'desktop.apps'
  | 'desktop.files'
  | 'clipboard.history'
  | 'ai'
  | 'extensions.ownership'
  | 'shortcuts'
  | 'system'
  | 'places'
  | 'updates'
  | 'settings.write'
  | 'camera'

/** Action panels can be visible, menu-only, or hidden while still allowing shortcuts. */
export type ActionPanelVisibility = 'visible' | 'menu' | 'hidden'
export type ViewSize = 'default' | 'large'
export type ViewPresentation = 'root' | 'stacked' | 'preview'
export type PatchMode = 'patch' | 'replace' | 'prepend' | 'append'
export type ForegroundColor = 'yellow' | 'blue' | 'purple' | 'green' | 'red' | 'orange' | 'pink'
export type ShortcutScope = 'local' | 'global'
export type LogLevel = 'debug' | 'info' | 'warn' | 'error'
export type LogSource = 'main' | 'renderer' | 'extension' | 'host'

/** Host-rendered toast result. Return this from action handlers for lightweight feedback. */
export type ExtensionToastResult = { toast: { message: string; tone?: 'default' | 'error' } }

/** In-place list/grid update returned from an action handler. */
export type ExtensionViewPatch = {
  /** How to apply item changes. Defaults to host behavior, usually patch. */
  mode?: PatchMode
  /** Items to patch, replace, prepend, or append. Existing items are matched by stable `id`. */
  items?: Array<Partial<Omit<ExtensionItem, 'id'>> & { id: string }>
  /** Stable item ids to remove from the current view. */
  removeItemIds?: string[]
  /** Explicit work-in-progress state; avoid using it for passive background refreshes. */
  isLoading?: boolean
  /** Only set when intentionally moving focus; host otherwise preserves selection. */
  selectedItemId?: string
}

/** Return shape accepted by `ctx.actions.run` handlers and command handlers. */
export type ExtensionActionResult =
  | ExtensionView
  | ExtensionAction
  | ExtensionToastResult
  | { view?: ExtensionView | null; action?: ExtensionAction; patch?: ExtensionViewPatch; navigation?: 'push' | 'replace' | 'pop'; toast?: ExtensionToastResult['toast'] }
  | void

/** A declarative action. Helpers under `ctx.actions.*` create these; they do not execute them. */
export type ExtensionAction = {
  /** Host action type. Prefer `ctx.actions.*` helpers instead of spelling this manually. */
  type?: string
  title?: string
  subtitle?: string
  shortcut?: string
  shortcutScope?: ShortcutScope
  style?: 'regular' | 'destructive'
  requiresConfirmation?: boolean
  confirmMessage?: string
  confirmLabel?: string
  cancelLabel?: string
  dismissAfterRun?: 'auto'
  /** Nested Cmd+K action panel. */
  submenu?: ExtensionActionPanel
  /** Field values injected by the host when a form's `submitAction` runs. */
  formValues?: Record<string, string | boolean>
  /** Id of the focused item injected by the host when a view's `onSelectionChange` runs. */
  selectedItemId?: string
  /** Selected accessory value injected by the host when a search accessory's `onChange` runs. */
  value?: string
  /** Legacy payload carrier for selection id / accessory value. Prefer `selectedItemId` or `value`. */
  text?: string
  [key: string]: unknown
}

export type ExtensionActionSection = {
  title?: string
  actions: ExtensionAction[]
  lazyActions?: ExtensionAction[]
  isLoading?: boolean
}

/** Grouped actions exposed through Cmd+K, local shortcuts, and host-owned action UI. */
export type ExtensionActionPanel = {
  title?: string
  sections: ExtensionActionSection[]
}

export type ExtensionItemAccessory = { text?: string; icon?: string }
export type ExtensionItemAppearance = { foreground?: ForegroundColor }

/** Item displayed in root/search providers, list views, and grid views. */
export type ExtensionItem = {
  /** Stable id used for ranking, recents, patching, and shortcut ownership. */
  id: string
  title: string
  subtitle?: string
  accessories?: ExtensionItemAccessory[]
  keywords?: string[]
  aliases?: string[]
  text?: string
  /** Any Lucide icon name in camel/Pascal/kebab case, e.g. `camera`, `volume-2`, `audio-lines`. */
  icon?: string
  /** Display image URL/data URL. For local files use `file.url` or `ctx.desktop.files.toFileUrl(path)`, not raw paths. */
  image?: string
  video?: string
  videoUrl?: string
  path?: string
  filePath?: string
  fileUrl?: string
  /** Enter/default behavior. */
  primaryAction?: ExtensionAction
  /** Secondary actions shown by Cmd+K. */
  actions?: ExtensionAction[]
  actionPanel?: ExtensionActionPanel
  actionPanelVisibility?: ActionPanelVisibility
  appearance?: ExtensionItemAppearance
  /** Ranking hint. The host caps provider scores and combines them with usage signals. */
  score?: number
  lastUsed?: number
  dismissAfterRun?: 'auto'
  [key: string]: unknown
}

export type ExtensionItemSection = { title?: string; subtitle?: string; items: ExtensionItem[] }

export type ExtensionPagination = {
  hasMore?: boolean
  pageSize?: number
  onLoadMore?: ExtensionAction
}

export type ExtensionSearchAccessory = {
  id?: string
  tooltip?: string
  value?: string
  items: Array<{ title: string; value: string }>
  onChange?: ExtensionAction
}

/** Host-rendered view. Prefer helpers such as `ctx.ui.list(...)` so the `type` is set for you. */
export type ExtensionView = {
  id?: string
  type?: 'list' | 'grid' | 'preview' | 'chat' | 'form' | 'progress' | 'webview' | 'camera'
  title: string
  size?: ViewSize
  presentation?: ViewPresentation
  subtitle?: string
  content?: string
  html?: string
  image?: string
  video?: string
  videoUrl?: string
  deviceId?: string
  showDeviceSwitcher?: boolean
  muted?: boolean
  controls?: boolean
  items?: ExtensionItem[]
  sections?: ExtensionItemSection[]
  isLoading?: boolean
  emptyView?: { title?: string; subtitle?: string }
  searchBarPlaceholder?: string
  selectedItemId?: string
  onSelectionChange?: ExtensionAction
  pagination?: ExtensionPagination
  searchAccessory?: ExtensionSearchAccessory
  /** Host polls only while visible; prefer targeted patches from explicit actions when possible. */
  refresh?: { intervalMs?: number; action?: ExtensionAction; mode?: PatchMode }
  actions?: ExtensionAction[]
  actionPanel?: ExtensionActionPanel
  actionPanelVisibility?: ActionPanelVisibility
  layout?: 'square' | 'wide' | 'compact'
  aspectRatio?: string | number
  columns?: number
  messages?: Array<{ role: 'user' | 'assistant' | 'system'; content: string }>
  fields?: Array<{ id: string; label: string; type?: string; value?: string; placeholder?: string; required?: boolean }>
  submitAction?: ExtensionAction
  steps?: Array<{ title: string; status?: string }>
  [key: string]: unknown
}

export type ExtensionFileKind = 'image' | 'video' | 'file' | string

/** File object returned by `ctx.desktop.files.*` helpers. */
export type ExtensionFile = {
  path: string
  name: string
  displayPath?: string
  /** Thumbnail-safe display URL for Electron views. */
  url?: string
  fileUrl?: string
  videoUrl?: string
  thumbnailUrl?: string
  kind?: ExtensionFileKind
  extension?: string
  mtime?: string
  mtimeMs?: number
  birthtime?: string
  birthtimeMs?: number
  dateAdded?: string
  dateAddedMs?: number
  size?: number
}

export type ExtensionFindFilesOptions = {
  limit?: number
  depth?: number
  extensions?: string[]
  kind?: 'image' | 'video' | 'media' | 'file' | string
  pattern?: string | string[]
  /** `added` is usually best for Downloads/screenshots; `recent`/`modified` use filesystem mtime. */
  sortBy?: 'recent' | 'modified' | 'added' | 'created' | 'name' | 'size'
  order?: 'asc' | 'desc'
}

export type ExtensionShellResult = { stdout: string; stderr: string; exitCode: number }
export type ExtensionShellOptions = { cwd?: string; env?: Record<string, string>; timeout?: number; shell?: boolean; outputLimit?: number }
export type ExtensionOpenWithApp = { name?: string; path?: string; [key: string]: unknown }

export type RecentLogOptions = { limit?: number; level?: LogLevel; source?: LogSource; sinceMs?: number; query?: string; extensionId?: string }
export type LogEntry = { timestamp: string; level: LogLevel; source: LogSource; scope?: string; extensionId?: string; commandId?: string; message: string; data?: unknown }

export type ExtensionStorage = {
  /** Persistent per-extension JSON state stored in app data. */
  get<T = unknown>(key: string, fallback?: T): Promise<T | undefined>
  set<T = unknown>(key: string, value: T): Promise<T>
  delete(key: string): Promise<void>
  clear(): Promise<void>
  /** Persisted OS-cache memo. Use for expensive work that can be recomputed. */
  memo<T = unknown>(key: string, ttlMs: number, loader: () => Promise<T> | T): Promise<T>
  /** Returns stale cached data while a background refresh runs when possible. */
  memoStale<T = unknown>(key: string, ttlMs: number, staleTtlMs: number, loader: () => Promise<T> | T): Promise<T>
}

export type ExtensionRuntimeCache = {
  /** Process-memory cache; not persistent. Entries are scoped to this extension. */
  get<T = unknown>(key: string): T | undefined
  getStale<T = unknown>(key: string): T | undefined
  set<T = unknown>(key: string, value: T, options?: { ttlMs?: number }): T
  has(key: string): boolean
  invalidate(key?: string): void
}

export type ExtensionSettings = {
  definitions(): Array<{ id: string; title?: string; description?: string; type?: string; value?: unknown; [key: string]: unknown }>
  get<T = unknown>(id: string): T
  set<T = unknown>(id: string, value: T): T
  toggle(id: string): unknown
}

export type ExtensionLogs = Record<LogLevel, (message: string, data?: unknown) => void> & {
  recent(options?: RecentLogOptions): Promise<LogEntry[]>
}

export type ExtensionAi = {
  /** One-shot AI call. Quota-limited per extension; declare `ai` permission. */
  ask(prompt: string, options?: { system?: string }): Promise<string>
  /** Per-extension conversational session. Session ids are scoped to the extension. */
  session(id?: string, options?: { system?: string }): { ask(prompt: string): Promise<string>; reset(): unknown }
}

export type ExtensionOwnership = {
  ownerOf(extensionFile: string): string | undefined
  filesForChat(chatId: string): string[]
  canWrite(extensionFile: string, chatId: string): boolean
  /** Host-only mutator exposed to the built-in AI Builder extension. */
  claim?(extensionFile: string, chatId: string): boolean
  /** Host-only mutator exposed to the built-in AI Builder extension. */
  reload?(): Promise<void>
}

export type ExtensionAiBuilder = {
  /** Host-only surface: available to the built-in `nevermind.ai-builder` extension. */
  startChat(input: { prompt: string; title?: string; options?: Record<string, unknown> }): ExtensionAction
  openChat(chatId: string, input?: { title?: string; options?: Record<string, unknown> }): ExtensionAction
  removeChat(chatId: string, input?: { title?: string; options?: Record<string, unknown> }): ExtensionAction
  tweakExtension(input: { extensionFile?: string; extensionId?: string; title?: string; prompt?: string; options?: Record<string, unknown> }): ExtensionAction
  openChatsList(input?: { title?: string; options?: Record<string, unknown> }): ExtensionAction
  listChats(): Array<{ id: string; title?: string; query?: string; status?: string; createdAt?: number; updatedAt?: number; extensionFiles: string[] }>
  getChat(chatId: string): { id: string; title?: string; query?: string; status?: string; messages?: unknown[]; extensionFiles: string[] } | null
}

/** Serializable snapshot of the host update manager, returned by `ctx.updates.getState()`. */
export type ExtensionUpdateState = {
  currentVersion: string
  status: string
  supported: boolean
  checking: boolean
  downloading: boolean
  installing: boolean
  availableVersion: string | null
  downloadedVersion: string | null
  errorMessage: string | null
}

export type ExtensionContext = {
  /** Runtime metadata for the current extension plus host helpers such as persistent rename. */
  extension: NevermindExtension & { rename(metadata: string | { title?: string; subtitle?: string; commandTitle?: string; commandSubtitle?: string }): Promise<unknown> }
  command?: ExtensionCommand

  /** Declarative host-owned UI primitives. Nevermind owns rendering, navigation, filtering, actions, shortcuts, and errors. */
  ui: {
    list(view: ExtensionView): ExtensionView
    grid(view: ExtensionView): ExtensionView
    /** Full preview view for markdown/text/media. */
    preview(view: ExtensionView): ExtensionView
    /** Large media preview from a file helper result. */
    preview(file: ExtensionFile, view?: Partial<ExtensionView>): ExtensionView
    /** Inline preview action, commonly for clipboard/file items. */
    preview(input: { kind?: 'clipboard' | 'image' | 'video' | 'file' | 'text'; title?: string; text?: string; imageDataUrl?: string; imagePath?: string; videoUrl?: string; filePath?: string; thumbnailUrl?: string; clipboardType?: string; shortcut?: string }): ExtensionAction
    chat(view: ExtensionView): ExtensionView
    form(view: ExtensionView): ExtensionView
    progress(input?: { title?: string; label?: string; steps?: Array<{ title: string; status?: string }>; id?: string; value?: number; total?: number; status?: string }): ExtensionView
    /** Wrap a declarative action in a host-rendered confirmation step. */
    confirm(input?: { title?: string; message?: string; confirmLabel?: string; cancelLabel?: string; destructive?: boolean; onConfirm?: ExtensionAction; action?: ExtensionAction }): ExtensionAction
    toast(input?: { message?: string; tone?: 'default' | 'error' }): ExtensionToastResult
    /** Sandboxed HTML/JS iframe with no Node access. Use only when host-owned primitives do not fit. */
    webview(view: ExtensionView): ExtensionView
    /** Host-owned live camera view. Declare `camera`; use `ctx.actions.camera.*` for switching/mute controls. */
    camera(view?: ExtensionView): ExtensionView
    item<T extends ExtensionItem>(item: T): T
    actions<T extends ExtensionAction[]>(actions: T): T
    empty(title?: string, subtitle?: string): ExtensionView
    loading(title?: string): ExtensionView
    error(title?: string, message?: string): ExtensionView
  }

  /** Declarative actions. Creating one does not run it; return it or attach it to an item/view. */
  actions: {
    openPath(filePath: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    revealPath(filePath: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    quickLook(filePath: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    openWith(filePath: string, app: ExtensionOpenWithApp | string, title?: string, options?: Record<string, unknown>): ExtensionAction
    openUrl(url: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    copyText(text: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    pasteText(text: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    copyImage(image: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    /** Destructive by default and confirmation-gated by the host. */
    trash(paths: string | string[], title?: string, options?: Record<string, unknown>): ExtensionAction
    push(title: string, view: ExtensionView | null, options?: Record<string, unknown>): ExtensionAction
    replace(title: string, view: ExtensionView | null, options?: Record<string, unknown>): ExtensionAction
    pop(title?: string, options?: Record<string, unknown>): ExtensionAction
    run(title: string, handler: (ctx: ExtensionContext, action: ExtensionAction) => ExtensionActionResult | Promise<ExtensionActionResult>, options?: Record<string, unknown>): ExtensionAction
    /** Fire-and-forget action that dismisses the palette immediately unless options override it. */
    background(title: string, handler: (ctx: ExtensionContext, action: ExtensionAction) => ExtensionActionResult | Promise<ExtensionActionResult>, options?: Record<string, unknown>): ExtensionAction
    shellExec(title: string, command: string, args?: string[], options?: ExtensionShellOptions): ExtensionAction
    shellScript(title: string, script: string, options?: ExtensionShellOptions): ExtensionAction
    toggleSetting(settingId: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    recordShortcut(input?: { actionId?: string; scope?: 'palette' | ShortcutScope; title?: string; action?: ExtensionAction }, options?: Record<string, unknown>): ExtensionAction
    removeShortcut(input?: { actionId?: string; title?: string }, options?: Record<string, unknown>): ExtensionAction
    setPaletteShortcut(title?: string, options?: Record<string, unknown>): ExtensionAction
    native(title: string, nativeAction: unknown, options?: Record<string, unknown>): ExtensionAction
    /** OS-owned system actions. Requires the `system` permission. Titles default to OS-appropriate labels. */
    system: Record<'lockScreen' | 'sleep' | 'restart' | 'openSystemSettings' | 'openKeyboardSettings' | 'quit', (title?: string, options?: Record<string, unknown>) => ExtensionAction>
    /** App update actions. Requires the `updates` permission. */
    updates: Record<'check' | 'download' | 'install', (title?: string, options?: Record<string, unknown>) => ExtensionAction>
    camera: Record<'switchDevice' | 'nextDevice' | 'previousDevice' | 'toggleMuted' | 'toggleControls', (title?: string, options?: Record<string, unknown>) => ExtensionAction>
  }

  /** Explicit return helpers from action handlers. Prefer these for imperative handler results. */
  navigation: {
    push(view: ExtensionView | null): { view: ExtensionView | null; navigation: 'push' }
    replace(view: ExtensionView | null): { view: ExtensionView | null; navigation: 'replace' }
    pop(): { navigation: 'pop' }
    run(action: ExtensionAction): { action: ExtensionAction }
  }

  /** Desktop capabilities. Optional namespaces require matching top-level permissions. */
  desktop: {
    clipboard?: {
      readText(): string
      writeText(text: string): void
      readImage(): string
      writeImage(image: unknown): unknown
      readFiles(): string[]
      read(): unknown
      write(value: unknown): unknown
    }
    selection: {
      text(): Promise<string> | string
      files(): Promise<string[]> | string[]
      read(): Promise<unknown> | unknown
    }
    apps?: {
      frontmost(): Promise<unknown> | unknown
      launch(appPath: string): unknown
    }
    files?: {
      find(roots: string[], options?: ExtensionFindFilesOptions): Promise<ExtensionFile[]>
      findImages(roots: string[], options?: Omit<ExtensionFindFilesOptions, 'kind'>): Promise<ExtensionFile[]>
      findVideos(roots: string[], options?: Omit<ExtensionFindFilesOptions, 'kind'>): Promise<ExtensionFile[]>
      findMedia(roots: string[], options?: Omit<ExtensionFindFilesOptions, 'kind'>): Promise<ExtensionFile[]>
      openWithApps(filePath: string): Promise<ExtensionOpenWithApp[]>
      open(filePath: string): unknown
      reveal(filePath: string): unknown
      preview(filePath: string): unknown
      readText(filePath: string): Promise<string>
      toFileUrl(filePath: string): string
    }
    shell?: {
      openExternal(url: string): unknown
      exec(command: string, args?: string[], options?: ExtensionShellOptions): Promise<ExtensionShellResult>
      script(script: string, options?: ExtensionShellOptions): Promise<ExtensionShellResult>
      appleScript(script: string, options?: ExtensionShellOptions): Promise<ExtensionShellResult>
      which(command: string): Promise<ExtensionShellResult>
    }
  }

  storage: ExtensionStorage
  settings: ExtensionSettings
  logs: ExtensionLogs
  cache: ExtensionRuntimeCache
  /** Current-view helpers. `refresh()` re-runs the command and patches/replaces the active view. */
  views: { refresh(): ExtensionAction; invalidate(): void }
  /** App update state. Present only with the `updates` permission. Pair with `ctx.actions.updates.*`. */
  updates?: { getState(): ExtensionUpdateState }
  state: Record<string, unknown>
  ai?: ExtensionAi
  aiBuilder?: ExtensionAiBuilder
  extensions: { ownership?: ExtensionOwnership }
}

export type ExtensionCommand = {
  id: string
  /** Stable action id for shortcuts/aliases. Defaults to the command id when omitted. */
  actionId?: string
  title: string
  subtitle?: string
  aliases?: string[]
  icon?: string
  score?: number
  shortcut?: string
  shortcutScope?: ShortcutScope
  globalShortcut?: string
  /** Dismiss immediately for fire-and-forget commands. */
  background?: boolean
  dismissAfterRun?: 'auto'
  run(ctx: ExtensionContext, action: ExtensionAction): ExtensionActionResult | Promise<ExtensionActionResult>
}

/**
 * Extension manifest. Commands appear in search automatically; provider methods
 * should contribute distinct child/status/query items, not duplicate command launchers.
 */
export type NevermindExtension = {
  id: string
  title: string
  subtitle?: string
  permissions?: ExtensionPermission[]
  commands?: ExtensionCommand[]
  /** Empty-query root contributions. Keep small, stable, cached, JSON-serializable, and bounded. */
  rootItems?(ctx: ExtensionContext): ExtensionItem[] | Promise<ExtensionItem[]> | { items: ExtensionItem[] } | Promise<{ items: ExtensionItem[] }>
  /** Query-aware root contributions. The host debounces, caps, ranks, caches, and failure-isolates providers. */
  searchItems?(ctx: ExtensionContext, query: string): ExtensionItem[] | Promise<ExtensionItem[]> | { items: ExtensionItem[] } | Promise<{ items: ExtensionItem[] }>
}

declare global {
  const module: { exports: unknown }
}

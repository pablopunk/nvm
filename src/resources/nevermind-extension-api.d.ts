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
  /** Required for shell execution, system actions, and `ctx.desktop.shell`. */
  | 'system'
  | 'places'
  | 'updates'
  | 'settings.write'
  | 'camera'
  /** Required for `ctx.ocr` image/screen/region text recognition helpers. */
  | 'ocr'

/** Action panels can be visible, menu-only, or hidden while still allowing shortcuts. */
export type ActionPanelVisibility = 'visible' | 'menu' | 'hidden'
export type ViewSize = 'default' | 'large'
export type ViewPresentation = 'root' | 'stacked' | 'preview'
export type PatchMode = 'patch' | 'replace' | 'prepend' | 'append'
export type ForegroundColor = 'yellow' | 'blue' | 'purple' | 'green' | 'red' | 'orange' | 'pink'
export type ShortcutScope = 'local' | 'global'
export type ExtensionEditorFormat = 'text' | 'markdown'
export type LogLevel = 'debug' | 'info' | 'warn' | 'error'
export type LogSource = 'main' | 'renderer' | 'extension' | 'host'
export type ExtensionFormValue = string | boolean | string[]
export type ExtensionFormFieldType = 'text' | 'textarea' | 'password' | 'email' | 'url' | 'number' | 'date' | 'checkbox' | 'dropdown' | 'select' | 'multiselect' | 'file' | 'files' | 'folder' | 'description' | 'separator'
export type ExtensionFormOption = { title: string; value: string }

export type ExtensionActionPlacement = 'search' | 'root' | 'hidden'
export type ExtensionBackgroundMode = 'view' | 'noView' | 'background'
export type ExtensionBackgroundTrigger =
  | { type: 'startup'; delayMs?: number }
  | { type: 'interval'; every: string | number; delayMs?: number }
  /** Requires `desktop.files`; roots are watched by the host with debouncing, no-overlap, and bounded launch context. */
  | { type: 'files.changed'; roots: string | string[]; debounceMs?: number; includeHidden?: boolean; extensions?: string[]; kind?: 'file' | 'image' | 'video' | 'media'; ignore?: string[] }
  /** Requires `clipboard.history`; fires after Nevermind records a changed clipboard item. */
  | { type: 'clipboard.changed'; debounceMs?: number }
  /** Requires `desktop.apps`; backed by a host frontmost-app poller. */
  | { type: 'app.frontmost.changed'; debounceMs?: number }
  | { type: 'wake' | 'login' }

export type ExtensionLaunchContext = {
  /** Declarative trigger that started this run, when known. */
  trigger?: ExtensionBackgroundTrigger
  /** Host event name, primarily for diagnostics. File watcher events are scoped per job. */
  event?: string
  /** Changed file metadata for `files.changed` triggers. Bounded by the host. */
  files?: ExtensionFile[]
  /** Absolute changed paths for `files.changed` triggers. Bounded by the host. */
  changedPaths?: string[]
  /** Run reason such as `manual`, `startup`, `interval`, or `event:*`. */
  reason?: string
  /** Epoch milliseconds for when the host started this action/job. */
  startedAt: number
}

export type ExtensionActionContribution = {
  /** Stable local id. Global shortcuts, aliases, recents, and action refs depend on this. */
  id: string
  /** Optional stable global action id. Defaults to `extension-action:${extension.id}:${id}`. */
  actionId?: string
  title: string
  subtitle?: string
  aliases?: string[]
  keywords?: string[]
  icon?: string
  image?: string
  score?: number
  shortcut?: string
  shortcutScope?: ShortcutScope
  globalShortcut?: string
  dismissAfterRun?: 'auto'
  background?: boolean
  customizable?: boolean
  /** Muted semantic color applied to the action result title and Lucide/fallback icon. Images keep their original colors. */
  appearance?: ExtensionItemAppearance
  /** Where this durable action should be discoverable. Defaults to `['search']`. */
  placement?: ExtensionActionPlacement[]
  /** Execution lifecycle. `background`/`noView` actions are eligible for host-managed jobs and diagnostics. */
  mode?: ExtensionBackgroundMode
  /** Declarative host-owned trigger intents. The host owns scheduling, permissions, no-overlap, backoff, and diagnostics. */
  triggers?: ExtensionBackgroundTrigger[]
  /** Declarative action to run, commonly `ctx.windows.toggle(...)`, `ctx.actions.pasteText(...)`, or `ctx.actions.push(...)`. */
  action?: ExtensionAction
  /** Handler to run when a simple declarative action is not enough. */
  run?: (ctx: ExtensionContext, action: ExtensionAction) => ExtensionActionResult | Promise<ExtensionActionResult>
}

export type ExtensionPasteTextOptions = {
  /** Keep the palette visible after dispatching paste. Defaults to false unless the action also overrides dismiss behavior. */
  keepPaletteOpen?: boolean
  /** Restore the previous clipboard contents shortly after paste is dispatched. */
  restoreClipboard?: boolean
  /** Write only plain text for paste, omitting HTML/RTF flavors. Defaults to true. */
  plainText?: boolean
  /** Prevent temporary paste contents from being added to Nevermind clipboard history. Implies history suppression for the paste write. */
  concealed?: boolean
  /** Delay before restoring clipboard contents, in milliseconds. Defaults to 250. */
  restoreDelayMs?: number
}

export type ExtensionTypeTextOptions = {
  /** Delay between typed characters in milliseconds where supported. */
  delayMs?: number
}

export type ExtensionClipboardContent =
  | { type: 'text'; text: string; html?: string; concealed?: boolean }
  | { type: 'html'; html: string; text?: string; concealed?: boolean }
  | { type: 'image'; imageDataUrl?: string; image?: string; path?: string; concealed?: boolean }
  | { type: 'files'; paths: string[]; concealed?: boolean }

export type ExtensionClipboardWriteOptions = {
  /** Prevent this write from being retained by Nevermind clipboard history where possible. */
  concealed?: boolean
}

export type ExtensionPasteOptions = ExtensionClipboardWriteOptions & {
  /** Restore the previous clipboard contents shortly after paste is dispatched. */
  restoreClipboard?: boolean
  /** Delay before restoring clipboard contents, in milliseconds. Defaults to 250. */
  restoreDelayMs?: number
}

export type ExtensionClipboardReadOptions = { formats?: Array<'text' | 'html' | 'image' | 'files'> }

export type ExtensionClipboardReadResult =
  | (ExtensionClipboardContent & { files?: ExtensionFile[] })
  | { type: 'empty' }

export type ExtensionWindowOptions = {
  /** Stable id for reusing and controlling a window. Defaults to the view id/title. */
  id?: string
  title?: string
  /** Native window title bar. Use `hidden` for title-less floating companion windows. */
  titleBar?: 'default' | 'hidden'
  /** Host chrome around the view. Use `none` for edge-to-edge companion tools. */
  chrome?: 'default' | 'none'
  width?: number
  height?: number
  size?: ViewSize
  alwaysOnTop?: boolean
  visibleOnAllSpaces?: boolean
  hideOnBlur?: boolean
  persistent?: boolean
  remembersFrame?: boolean
}

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
  /** Only set when intentionally moving focus to a visible item id; host otherwise preserves selection. */
  selectedItemId?: string
}

/** Return shape accepted by `ctx.actions.run` handlers and command handlers. */
export type ExtensionNavigationMode = 'root' | 'push' | 'replace' | 'pop'

export type ExtensionActionResult =
  | ExtensionView
  | ExtensionAction
  | ExtensionToastResult
  | { view?: ExtensionView | null; action?: ExtensionAction; patch?: ExtensionViewPatch; navigation?: ExtensionNavigationMode; toast?: ExtensionToastResult['toast'] }
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
  formValues?: Record<string, ExtensionFormValue>
  /** Current text injected by the host when an editor view's `submitAction` runs. */
  editorContent?: string
  /** Id of the focused item injected by the host when a view's `onSelectionChange` runs. */
  selectedItemId?: string
  /** Selected accessory value injected by the host when a search accessory's `onChange` runs. */
  value?: string
  /** Legacy payload carrier for selection id / accessory value. Prefer `selectedItemId` or `value`. */
  text?: string
  /** Prompt fields carried by `ctx.input.prompt(...)`. */
  fields?: ExtensionFormField[]
  promptMessage?: string
  submitTitle?: string
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

export type ExtensionAccessoryTone = 'default' | 'muted' | 'accent' | 'success' | 'warning' | 'danger'
export type ExtensionItemAccessory = { text?: string; icon?: string; tone?: ExtensionAccessoryTone; tooltip?: string }
export type ExtensionImage = string | { src?: string; light?: string; dark?: string; fallback?: string; alt?: string; fit?: 'cover' | 'contain'; shape?: 'square' | 'rounded' | 'circle'; tint?: ForegroundColor | string; mask?: 'none' | 'rounded' | 'circle' }
export type ExtensionMetadataItem =
  | { type?: 'text'; label: string; value: string; copyable?: boolean }
  | { type: 'link'; label: string; value: string; url: string }
  | { type: 'tag'; label?: string; value: string; tone?: ExtensionAccessoryTone }
  | { type: 'separator' }
export type ExtensionDetail = { title?: string; subtitle?: string; markdown?: string; metadata?: ExtensionMetadataItem[]; image?: ExtensionImage; actions?: ExtensionAction[] }
export type ExtensionItemAppearance = {
  /** Muted semantic color applied to the item title and Lucide/fallback icon. Images keep their original colors. */
  foreground?: ForegroundColor
}

/** Item displayed in root/search providers, list views, and grid views. */
export type ExtensionItem = {
  /** Stable id used for ranking, recents, patching, and shortcut ownership. */
  id: string
  /** Optional stable action id for shortcuts/aliases when promoted into a persistent action. */
  actionId?: string
  title: string
  subtitle?: string
  accessories?: ExtensionItemAccessory[]
  keywords?: string[]
  aliases?: string[]
  text?: string
  /** Any Lucide icon name in camel/Pascal/kebab case, e.g. `camera`, `volume-2`, `audio-lines`. */
  icon?: string
  /** Display image URL/data URL or image descriptor. For local files use `file.url` or `ctx.desktop.files.toFileUrl(path)`, not raw paths. */
  image?: ExtensionImage
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
  /** Optional detail/inspector content rendered by list views with `view.detail.visible`. */
  detail?: ExtensionDetail
  shortcut?: string
  shortcutScope?: ShortcutScope
  globalShortcut?: string
  /** Set false when an item should be searchable but not user-customizable. */
  customizable?: boolean
  /** Dismiss immediately for fire-and-forget persistent actions. */
  background?: boolean
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
  type?: 'list' | 'grid' | 'preview' | 'chat' | 'form' | 'editor' | 'progress' | 'webview' | 'camera'
  title: string
  size?: ViewSize
  presentation?: ViewPresentation
  subtitle?: string
  content?: string
  /** Placeholder for editable text surfaces such as `ctx.ui.editor(...)`. */
  placeholder?: string
  /** Text format for editable text surfaces. Markdown editors get a host-rendered preview. */
  format?: ExtensionEditorFormat
  /** Optional language hint for text/code editors. */
  language?: string
  /** Render an editor as read-only while preserving selection/copy behavior. */
  readOnly?: boolean
  html?: string
  image?: ExtensionImage
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
  /** Optional item detail pane for richer list views. */
  detail?: { placement?: 'side' | 'bottom'; visible?: boolean }
  searchBarPlaceholder?: string
  /** Initial focused item for list/grid views. Must match a stable visible item id; sorting remains independent. */
  selectedItemId?: string
  onSelectionChange?: ExtensionAction
  pagination?: ExtensionPagination
  searchAccessory?: ExtensionSearchAccessory
  /** Host polls only while visible. The renderer receives an opaque host refresh handle, never an executable action. */
  refresh?: { intervalMs?: number; mode?: PatchMode; /** @deprecated Use `ctx.views.refresh()` only for existing extensions; new refreshes should be declarative and host-owned. */ action?: ExtensionAction }
  actions?: ExtensionAction[]
  actionPanel?: ExtensionActionPanel
  actionPanelVisibility?: ActionPanelVisibility
  layout?: 'square' | 'wide' | 'compact'
  aspectRatio?: string | number
  columns?: number
  messages?: Array<{ role: 'user' | 'assistant' | 'system'; content: string }>
  fields?: ExtensionFormField[]
  submitAction?: ExtensionAction
  steps?: Array<{ title: string; status?: string }>
  /** Current progress value for progress views. Pair with `total` for a determinate bar. */
  value?: number
  /** Total progress value for progress views. Pair with `value` for a determinate bar. */
  total?: number
  /** Human-readable progress summary, e.g. `Downloading assets…`. */
  status?: string
  [key: string]: unknown
}

export type ExtensionFormField = {
  id: string
  label?: string
  type?: ExtensionFormFieldType
  value?: ExtensionFormValue
  placeholder?: string
  required?: boolean
  options?: ExtensionFormOption[]
  description?: string
  error?: string
  rows?: number
  /** File extensions allowed by file/files fields, e.g. ['png', 'jpg'] or ['.md']. */
  extensions?: string[]
  /** Display name for extension filters in native file pickers. */
  filterName?: string
  /** Picker button label for file/files/folder fields. */
  buttonLabel?: string
  /** Optional starting path for file/files/folder fields. Supports ~ expansion. */
  defaultPath?: string
  /** Whether folder pickers should allow creating folders. Defaults to true. */
  canCreateDirectories?: boolean
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
  mimeType?: string
  width?: number
  height?: number
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

export type ExtensionFileIndexOptions = Omit<ExtensionFindFilesOptions, 'sortBy' | 'order'> & {
  /** Roots to scan or filter, e.g. ['~/Downloads']. Defaults to Desktop/Documents/Downloads. */
  roots?: string | string[]
  /** Include hidden dotfiles during explicit reindex scans. Defaults to false. */
  includeHidden?: boolean
  /** Ignore basenames or simple wildcard patterns such as '*.tmp'. Defaults include node_modules, .git, Library, and Applications. */
  ignore?: string | string[]
  /** Filter an existing snapshot by query. `searchIndex(query)` sets this for you. */
  query?: string
}

export type ExtensionOcrBlock = { text: string; confidence?: number; boundingBox?: { x: number; y: number; width: number; height: number } }
export type ExtensionOcrResult = { text: string; confidence?: number | null; language?: string | null; blocks?: ExtensionOcrBlock[] }
export type ExtensionOcrOptions = { language?: string; languages?: string[]; timeoutMs?: number }
export type ExtensionOcrRegion = { x: number; y: number; width: number; height: number }

export type ExtensionShellResult = { stdout: string; stderr: string; exitCode: number }
export type ExtensionShellOptions = { cwd?: string; env?: Record<string, string>; timeout?: number; shell?: boolean; outputLimit?: number }
/** An installed application from `ctx.desktop.apps.list/search`. Use `id` for item identity and `name` for display titles. */
export type ExtensionApp = { id: string; name: string; path: string; [key: string]: unknown }
/** An app that can open a specific file. Returned by `ctx.desktop.files.openWithApps(filePath)`. */
export type ExtensionOpenWithApp = ExtensionApp
/** A host-indexed file from `ctx.desktop.files.indexSnapshot/recent/searchIndex`. */
export type ExtensionIndexedFile = { id: string; name: string; path: string; displayPath?: string; extension?: string; kind?: ExtensionFileKind }

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

export type ExtensionShortcutRecord = {
  actionId: string
  title: string
  subtitle?: string
  accelerator: string
  scope: 'global'
  source: 'user' | 'extension'
}

export type ExtensionPaletteShortcut = {
  title: string
  accelerator: string
  scope: 'palette'
}

export type ExtensionLogs = Record<LogLevel, (message: string, data?: unknown) => void> & {
  recent(options?: RecentLogOptions): Promise<LogEntry[]>
}

export type ExtensionAiAttachment =
  | string
  | { type: 'text'; text: string; title?: string }
  | { type: 'image'; path?: string; filePath?: string; dataUrl?: string; imageDataUrl?: string; data?: string; mimeType?: string; title?: string; ocr?: boolean }
  | { type: 'file'; path?: string; filePath?: string; file?: ExtensionFile; title?: string; as?: 'text' | 'image' | 'metadata' | 'ocr'; ocr?: boolean }

export type ExtensionAiResolvableAttachment = ExtensionAiAttachment | ExtensionAiAttachment[] | null | Promise<ExtensionAiAttachment | ExtensionAiAttachment[] | null>

export type ExtensionAiStreamEvent =
  | { type: 'start' }
  | { type: 'delta'; text: string }
  | { type: 'tool_start' | 'tool_end'; name?: string; isError?: boolean }
  | { type: 'done' }
  | { type: 'aborted' }
  | { type: 'error'; message?: string; data?: unknown }

export type ExtensionAiOptions = {
  system?: string
  /** Text, file, image, selected, clipboard, or OCR context. File/image attachments require `desktop.files`; clipboard helpers require `clipboard.history`; OCR requires `ocr`. */
  attachments?: ExtensionAiResolvableAttachment | ExtensionAiResolvableAttachment[]
  /** Abort the request with a standard AbortController. */
  signal?: AbortSignal
  onDelta?: (delta: string) => void
  onEvent?: (event: ExtensionAiStreamEvent) => void
}

export type ExtensionAiStream = {
  /** Resolves to the full assistant text after streaming completes. */
  result: Promise<string>
  /** Abort the in-flight request. */
  abort(): void
}

export type ExtensionAi = {
  /** One-shot AI call. Quota-limited per extension; declare `ai` permission. Supports attachments and AbortController signals. */
  ask(prompt: string, options?: ExtensionAiOptions): Promise<string>
  /** Streaming AI call. Use `onDelta`/`onEvent` for incremental updates and await `result` for the final text. */
  stream(prompt: string, options?: ExtensionAiOptions): ExtensionAiStream
  /** Attachment builders normalize host-owned context for AI prompts. */
  attachments: {
    text(text: string, title?: string): ExtensionAiAttachment
    image(pathOrDataUrl: string, options?: { title?: string; ocr?: boolean }): ExtensionAiAttachment
    file(pathOrFile: string | ExtensionFile, options?: { title?: string; as?: 'text' | 'image' | 'metadata' | 'ocr'; ocr?: boolean }): ExtensionAiAttachment
    selectedText(title?: string): Promise<ExtensionAiAttachment>
    selectedFiles(options?: { title?: string; as?: 'text' | 'image' | 'metadata' | 'ocr'; ocr?: boolean }): Promise<ExtensionAiAttachment[]>
    clipboard(options?: { title?: string; as?: 'text' | 'image' | 'metadata' | 'ocr'; ocr?: boolean }): Promise<ExtensionAiAttachment | ExtensionAiAttachment[] | null>
    ocrImage(pathOrDataUrlOrFile: string | ExtensionFile, options?: { title?: string }): Promise<ExtensionAiAttachment>
  }
  /** Per-extension conversational session. Session ids are scoped to the extension. */
  session(id?: string, options?: { system?: string }): { ask(prompt: string, options?: ExtensionAiOptions): Promise<string>; stream(prompt: string, options?: ExtensionAiOptions): ExtensionAiStream; reset(): unknown }
}

export type ExtensionOwnership = {
  ownerOf(extensionFile: string): string | undefined
  filesForChat(chatId: string): string[]
  canWrite(extensionFile: string, chatId: string): boolean
  /** Host-only mutator exposed to the built-in AI Builder extension. */
  claim?(extensionFile: string, chatId: string): boolean
  /** Host-only mutator exposed to the built-in AI Builder extension. */
  remove?(extensionFile: string, chatId: string): Promise<boolean>
  /** Host-only mutator exposed to the built-in AI Builder extension. */
  reload?(): Promise<void>
}

export type ExtensionTemplateValue = string | number | boolean | string[] | null | undefined
export type ExtensionTemplateResult = { text: string; cursor?: number; missingVariables?: string[] }
export type ExtensionTemplateOptions = {
  variables?: Record<string, ExtensionTemplateValue>
  /** Return `{ text, cursor, missingVariables }` instead of a string. */
  returnResult?: boolean
  /** Return cursor position for `{cursor}` placeholders. Alias for `returnResult`. */
  returnCursor?: boolean
  /** Internal marker used while calculating cursor position. Defaults to a private sentinel. */
  cursorToken?: string
  /** Include `{clipboard}` when the extension has `clipboard.history`. Defaults to true when permitted. */
  includeClipboard?: boolean
  /** Include `{selectedText}`. Defaults to true. */
  includeSelectedText?: boolean
  /** Include missing variable names in the result; pair with `ctx.input.prompt(...)` for prompted arguments. */
  promptMissing?: boolean
}

export type ExtensionText = {
  /**
   * Expand host-standard text templates. Variables use `{name}` or `{{name}}`.
   * Built-ins include `{date}`, `{time}`, `{datetime}`, `{uuid}`, `{selectedText}`, `{clipboard}`,
   * `{cursor}`, `{argument:name}`, and `{calculator:1 + 2}`. Explicit variables override built-ins.
   */
  template(input: string, variables?: Record<string, ExtensionTemplateValue>): Promise<string>
  template(input: string, options: ExtensionTemplateOptions & { returnResult: true }): Promise<ExtensionTemplateResult>
  template(input: string, options: ExtensionTemplateOptions & { returnCursor: true }): Promise<ExtensionTemplateResult>
  template(input: string, options: ExtensionTemplateOptions): Promise<string | ExtensionTemplateResult>
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

/** A clipboard history entry returned by `ctx.clipboard.history.list/search`. */
export type ExtensionClipboardEntry = {
  id: string
  type?: string
  text?: string
  imageDataUrl?: string
  imagePath?: string
  videoUrl?: string
  filePath?: string
  thumbnailUrl?: string
  createdAt?: number
}

/** Clipboard history access. Requires the `clipboard.history` permission. Destructive helpers are host-managed and return counts. */
export type ExtensionClipboardHistory = {
  list(options?: { limit?: number; query?: string; types?: string[] }): ExtensionClipboardEntry[]
  search(query: string, options?: { limit?: number; types?: string[] }): ExtensionClipboardEntry[]
  get(id: string): ExtensionClipboardEntry | null
  remove(idOrIds: string | string[]): Promise<{ removed: number }>
  clear(options?: { types?: string[]; olderThanMs?: number }): Promise<{ removed: number }>
}

/**
 * Read-only OS metadata. Use `capabilities.has(...)` to omit unsupported items from
 * discovery instead of duplicating platform logic, and `labels.*` for OS-appropriate titles.
 */
export type ExtensionSystem = {
  /** Display label for the host OS, e.g. `macOS`. */
  os: string
  capabilities: { has(id: string): boolean }
  labels: {
    revealInFileManager: string
    previewFile: string
    openSystemSettings: string
    keyboardSettings: string
  }
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
  /** Host-owned launch context for background/triggered runs. Undefined for normal palette executions. */
  launch?: ExtensionLaunchContext

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
    /** Editable host-owned text/markdown surface. The host injects `editorContent` into `submitAction`. */
    editor(view: ExtensionView): ExtensionView
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
    pasteText(text: string, title?: string, options?: ExtensionPasteTextOptions & Record<string, unknown>): ExtensionAction
    /** Paste text/html/image/files through the host while preserving concealed/restore behavior. */
    paste(content: ExtensionClipboardContent, title?: string, options?: ExtensionPasteOptions & Record<string, unknown>): ExtensionAction
    /** Reference a persistent action declared by `actions(ctx)` using its local contribution `id` (`registeredActionId`), not the optional global `actionId`. Use inside views so rows share aliases/shortcuts with the durable action. */
    ref(registeredActionId: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    /** Type text into the frontmost app without touching the clipboard. Check `ctx.system.capabilities.has('keyboard.type-text')` for support. */
    typeText(text: string, title?: string, options?: ExtensionTypeTextOptions & Record<string, unknown>): ExtensionAction
    copyImage(image: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    /** Destructive by default and confirmation-gated by the host. */
    trash(paths: string | string[], title?: string, options?: Record<string, unknown>): ExtensionAction
    /** Replace the palette's current navigation stack with this view. Use for top-level destinations, not drill-down rows. */
    root(title: string, view: ExtensionView | null, options?: Record<string, unknown>): ExtensionAction
    /** Drill into a child view and let Escape return to the current view. */
    push(title: string, view: ExtensionView | null, options?: Record<string, unknown>): ExtensionAction
    /** Swap the current view without adding a back-stack entry. */
    replace(title: string, view: ExtensionView | null, options?: Record<string, unknown>): ExtensionAction
    pop(title?: string, options?: Record<string, unknown>): ExtensionAction
    run(title: string, handler: (ctx: ExtensionContext, action: ExtensionAction) => ExtensionActionResult | Promise<ExtensionActionResult>, options?: Record<string, unknown>): ExtensionAction
    /** Fire-and-forget action that dismisses the palette immediately unless options override it. */
    background(title: string, handler: (ctx: ExtensionContext, action: ExtensionAction) => ExtensionActionResult | Promise<ExtensionActionResult>, options?: Record<string, unknown>): ExtensionAction
    /** Run a shell command. Requires the `system` permission. */
    shellExec(title: string, command: string, args?: string[], options?: ExtensionShellOptions): ExtensionAction
    /** Run a shell script through `/bin/bash -lc` by default. Requires the `system` permission. */
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

  /** Read-only host OS metadata: capability checks and intent-named labels. */
  system: ExtensionSystem

  /** Text and template helpers for snippets, quicklinks, prompts, and selected-text transforms. */
  text: ExtensionText

  /** Lightweight input helpers for command arguments. Prompted values are injected into the wrapped action as `formValues`. */
  input: {
    prompt(input: { title?: string; message?: string; fields: ExtensionFormField[]; action: ExtensionAction; submitTitle?: string }, options?: Record<string, unknown>): ExtensionAction
  }

  /** Independent host-owned windows for floating notes, dashboards, previews, and companions. */
  windows: {
    /** Open or update an independent window rendering the given host-owned view. */
    create(view: ExtensionView, options?: ExtensionWindowOptions): ExtensionAction
    show(id: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    hide(id: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    /** Toggle an existing window by id, or create it first when given a fallback view plus stable `options.id`. */
    toggle(idOrView: string | ExtensionView, titleOrOptions?: string | ExtensionWindowOptions, options?: ExtensionWindowOptions): ExtensionAction
    close(id: string, title?: string, options?: Record<string, unknown>): ExtensionAction
  }

  /** Clipboard history. `history` is present only with the `clipboard.history` permission. */
  clipboard: { history?: ExtensionClipboardHistory }

  /** OCR helpers. Present only with the `ocr` permission; check `ctx.system.capabilities.has('ocr')` for OS support. */
  ocr?: {
    /** Recognize text in an image path, file URL, data URL, or `ExtensionFile`. */
    image(input: string | ExtensionFile | { path?: string; filePath?: string; fileUrl?: string; url?: string }, options?: ExtensionOcrOptions): Promise<ExtensionOcrResult>
    /** Capture the screen and recognize visible text when screen recording permission is available. */
    screen(options?: ExtensionOcrOptions): Promise<ExtensionOcrResult>
    /** Capture a screen region in physical screen coordinates and recognize text. */
    region(rect: ExtensionOcrRegion, options?: ExtensionOcrOptions): Promise<ExtensionOcrResult>
  }

  /** Build a persistent action contribution for `actions(ctx)`. */
  action(action: ExtensionActionContribution): ExtensionActionContribution

  /** Explicit return helpers from action handlers. Prefer these for imperative handler results. */
  navigation: {
    /** Replace the palette's current navigation stack with this view. Use for top-level destinations. */
    root(view: ExtensionView | null): { view: ExtensionView | null; navigation: 'root' }
    /** Drill into a child view and let Escape return to the current view. */
    push(view: ExtensionView | null): { view: ExtensionView | null; navigation: 'push' }
    /** Swap the current view without adding a back-stack entry. */
    replace(view: ExtensionView | null): { view: ExtensionView | null; navigation: 'replace' }
    pop(): { navigation: 'pop' }
    run(action: ExtensionAction): { action: ExtensionAction }
  }

  /** Desktop capabilities. Optional namespaces require matching top-level permissions. */
  desktop: {
    keyboard: {
      /** Type text into the frontmost app without touching the clipboard. Check `ctx.system.capabilities.has('keyboard.type-text')` for support. */
      typeText(text: string, options?: ExtensionTypeTextOptions): Promise<unknown> | unknown
    }
    clipboard?: {
      readText(): string
      writeText(text: string, options?: ExtensionClipboardWriteOptions): void
      readHtml(): string
      writeHtml(html: string, text?: string, options?: ExtensionClipboardWriteOptions): void
      readImage(): string | null
      writeImage(image: unknown, options?: ExtensionClipboardWriteOptions): unknown
      readFiles(): Promise<ExtensionFile[]>
      writeFiles(paths: string[], options?: ExtensionClipboardWriteOptions): void
      read(options?: ExtensionClipboardReadOptions): Promise<ExtensionClipboardReadResult>
      write(value: ExtensionClipboardContent | string, options?: ExtensionClipboardWriteOptions): unknown
    }
    selection: {
      text(): Promise<string> | string
      files(): Promise<string[]> | string[]
      read(): Promise<unknown> | unknown
    }
    apps?: {
      frontmost(): Promise<unknown> | unknown
      launch(appPath: string): unknown
      /** Installed applications from the host index. */
      list(): ExtensionApp[]
      /** Installed applications whose name matches the query. */
      search(query: string): ExtensionApp[]
      /** Icon for an app path as a data URL, or null when unavailable. */
      icon(appPath: string): Promise<string | null>
    }
    files?: {
      find(roots: string[], options?: ExtensionFindFilesOptions): Promise<ExtensionFile[]>
      findImages(roots: string[], options?: Omit<ExtensionFindFilesOptions, 'kind'>): Promise<ExtensionFile[]>
      findVideos(roots: string[], options?: Omit<ExtensionFindFilesOptions, 'kind'>): Promise<ExtensionFile[]>
      findMedia(roots: string[], options?: Omit<ExtensionFindFilesOptions, 'kind'>): Promise<ExtensionFile[]>
      /** Supported apps for this file. When rendering a picker, map each app to `{ id: app.id, title: app.name, primaryAction: ctx.actions.openWith(filePath, app) }`. */
      openWithApps(filePath: string): Promise<ExtensionOpenWithApp[]>
      open(filePath: string): unknown
      reveal(filePath: string): unknown
      preview(filePath: string): unknown
      readText(filePath: string): Promise<string>
      toFileUrl(filePath: string): string
      /** Host-safe thumbnail URL for an image/video/file path when previewable. */
      thumbnail(filePath: string): string | null
      /** Canonical file metadata with host-safe URLs and image dimensions when available. */
      metadata(filePath: string): Promise<ExtensionFile>
      /** Configured default roots for the lightweight host file index. */
      indexedRoots(): string[]
      /** Snapshot of the host file index, filtered without rescanning. */
      indexSnapshot(options?: ExtensionFileIndexOptions): ExtensionIndexedFile[]
      /** Rebuild the lightweight host file index with bounded roots/depth/filter controls. */
      reindex(options?: ExtensionFileIndexOptions): Promise<{ count: number; roots: string[] }>
      /** Entries from the current host file index snapshot. */
      recent(options?: ExtensionFileIndexOptions): ExtensionIndexedFile[]
      /** Host file index entries whose name or path matches the query. */
      searchIndex(query: string, options?: ExtensionFileIndexOptions): ExtensionIndexedFile[]
    }
    /** Shell and process helpers. Present only with the `system` permission. */
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
  shortcuts: {
    /** Active global action shortcuts, including user overrides and declared extension shortcuts. */
    list(): ExtensionShortcutRecord[]
    /** Current app-wide shortcut used to open Nevermind. */
    palette(): ExtensionPaletteShortcut
  }
  logs: ExtensionLogs
  cache: ExtensionRuntimeCache
  /** Current-view helpers. `refresh()` may be placed in `view.refresh.action`; the host converts it to an opaque refresh handle before IPC. */
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
  /** Stable action id for shortcuts/aliases. Defaults to `extension:${extension.id}:${id}` for compatibility. */
  actionId?: string
  title: string
  subtitle?: string
  aliases?: string[]
  icon?: string
  score?: number
  shortcut?: string
  shortcutScope?: ShortcutScope
  globalShortcut?: string
  /** Muted semantic color applied to the command result title and Lucide/fallback icon. Images keep their original colors. */
  appearance?: ExtensionItemAppearance
  /** Dismiss immediately for fire-and-forget commands. Prefer `mode: 'background'` for durable scheduled work. */
  background?: boolean
  /** Execution lifecycle. `background`/`noView` commands are eligible for host-managed jobs and diagnostics. */
  mode?: ExtensionBackgroundMode
  /** Declarative host-owned trigger intents. The host owns scheduling, permissions, no-overlap, backoff, and diagnostics. */
  triggers?: ExtensionBackgroundTrigger[]
  dismissAfterRun?: 'auto'
  run(ctx: ExtensionContext, action: ExtensionAction): ExtensionActionResult | Promise<ExtensionActionResult>
}

/**
 * Extension manifest. `actions(ctx)` is the durable action registry. `commands` are
 * ergonomic shorthand for durable actions that appear in search automatically; the
 * host normalizes both into the same shortcut/alias/execution pipeline. Provider
 * methods should contribute distinct child/status/query items, not duplicate command launchers.
 */
export type NevermindExtension = {
  id: string
  title: string
  subtitle?: string
  permissions?: ExtensionPermission[]
  /** Shorthand for search-visible durable actions with imperative `run(ctx)`. */
  commands?: ExtensionCommand[]
  /**
   * Persistent action registry. Use this for shortcut-worthy static variants such as
   * “Compress 720p”, “Compress 1080p”, “Toggle Floating Note”, or fixed snippets.
   * These actions are searchable, aliasable, global-shortcutable, and runnable without
   * opening a view. Dynamic/state-driven rows such as calendar events, search results,
   * and files belong in views, `rootItems`, or `searchItems` instead.
   */
  actions?(ctx: ExtensionContext): ExtensionActionContribution[] | { actions: ExtensionActionContribution[] }
  /** Empty-query root contributions. Keep small, stable, cached, JSON-serializable, and bounded. */
  rootItems?(ctx: ExtensionContext): ExtensionItem[] | Promise<ExtensionItem[]> | { items: ExtensionItem[] } | Promise<{ items: ExtensionItem[] }>
  /** Query-aware root contributions. The host debounces, caps, ranks, caches, and failure-isolates providers. */
  searchItems?(ctx: ExtensionContext, query: string): ExtensionItem[] | Promise<ExtensionItem[]> | { items: ExtensionItem[] } | Promise<{ items: ExtensionItem[] }>
}

declare global {
  const module: { exports: unknown }
}

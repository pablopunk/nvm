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

export type ActionPanelVisibility = 'visible' | 'hidden'
export type ViewSize = 'default' | 'large'
export type ViewPresentation = 'root' | 'stacked' | 'preview'
export type PatchMode = 'patch' | 'replace' | 'prepend' | 'append'
export type ForegroundColor = 'yellow' | 'blue' | 'purple' | 'green' | 'red' | 'orange' | 'pink'

export type ExtensionAction = {
  type?: string
  title?: string
  shortcut?: string
  shortcutScope?: 'local' | 'global'
  style?: 'regular' | 'destructive'
  requiresConfirmation?: boolean
  submenu?: ExtensionActionPanel
  [key: string]: unknown
}

export type ExtensionActionPanel = {
  title?: string
  sections: Array<{ title?: string; actions: ExtensionAction[]; lazyActions?: ExtensionAction[]; isLoading?: boolean }>
}

export type ExtensionItem = {
  id: string
  title: string
  subtitle?: string
  accessories?: Array<{ text?: string; icon?: string }>
  keywords?: string[]
  aliases?: string[]
  text?: string
  icon?: string
  image?: string
  video?: string
  videoUrl?: string
  path?: string
  filePath?: string
  fileUrl?: string
  primaryAction?: ExtensionAction
  actions?: ExtensionAction[]
  actionPanel?: ExtensionActionPanel
  actionPanelVisibility?: ActionPanelVisibility
  appearance?: { foreground?: ForegroundColor }
  score?: number
  lastUsed?: number
  dismissAfterRun?: 'auto'
  [key: string]: unknown
}

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
  items?: ExtensionItem[]
  sections?: Array<{ title?: string; subtitle?: string; items: ExtensionItem[] }>
  isLoading?: boolean
  emptyView?: { title?: string; subtitle?: string }
  searchBarPlaceholder?: string
  selectedItemId?: string
  refresh?: { intervalMs?: number; action?: ExtensionAction; mode?: PatchMode }
  actions?: ExtensionAction[]
  actionPanel?: ExtensionActionPanel
  actionPanelVisibility?: ActionPanelVisibility
  layout?: 'square' | 'wide' | 'compact'
  aspectRatio?: string | number
  columns?: number
  [key: string]: unknown
}

export type ExtensionFile = {
  path: string
  name: string
  displayPath?: string
  url?: string
  fileUrl?: string
  videoUrl?: string
  thumbnailUrl?: string
  kind?: 'image' | 'video' | 'file' | string
  extension?: string
  mtime?: string
  mtimeMs?: number
  birthtime?: string
  birthtimeMs?: number
  dateAdded?: string
  dateAddedMs?: number
  size?: number
}

export type ExtensionContext = {
  extension: NevermindExtension & { rename(metadata: string | Record<string, unknown>): Promise<unknown> }
  command?: ExtensionCommand
  ui: {
    list(view: ExtensionView): ExtensionView
    grid(view: ExtensionView): ExtensionView
    preview(view: ExtensionView): ExtensionView
    preview(file: ExtensionFile, view?: Partial<ExtensionView>): ExtensionView
    preview(input: Record<string, unknown>): ExtensionView | ExtensionAction
    chat(view: ExtensionView): ExtensionView
    form(view: ExtensionView): ExtensionView
    progress(input?: Record<string, unknown>): ExtensionView
    confirm(input?: Record<string, unknown>): ExtensionAction
    toast(input?: { message?: string; tone?: 'default' | 'error' }): { toast: { message: string; tone?: 'default' | 'error' } }
    webview(view: ExtensionView): ExtensionView
    camera(view?: ExtensionView): ExtensionView
    item<T extends ExtensionItem>(item: T): T
    actions<T extends ExtensionAction[]>(actions: T): T
    empty(title?: string, subtitle?: string): ExtensionView
    loading(title?: string): ExtensionView
    error(title?: string, message?: string): ExtensionView
  }
  actions: {
    openPath(filePath: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    revealPath(filePath: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    quickLook(filePath: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    openWith(filePath: string, app: unknown, title?: string, options?: Record<string, unknown>): ExtensionAction
    openUrl(url: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    copyText(text: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    pasteText(text: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    copyImage(image: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    trash(paths: string | string[], title?: string, options?: Record<string, unknown>): ExtensionAction
    push(title: string, view: ExtensionView | null, options?: Record<string, unknown>): ExtensionAction
    replace(title: string, view: ExtensionView | null, options?: Record<string, unknown>): ExtensionAction
    pop(title?: string, options?: Record<string, unknown>): ExtensionAction
    run(title: string, handler: (ctx: ExtensionContext) => unknown | Promise<unknown>, options?: Record<string, unknown>): ExtensionAction
    background(title: string, handler: (ctx: ExtensionContext) => unknown | Promise<unknown>, options?: Record<string, unknown>): ExtensionAction
    shellExec(title: string, command: string, args?: string[], options?: Record<string, unknown>): ExtensionAction
    shellScript(title: string, script: string, options?: Record<string, unknown>): ExtensionAction
    toggleSetting(settingId: string, title?: string, options?: Record<string, unknown>): ExtensionAction
    recordShortcut(input?: Record<string, unknown>, options?: Record<string, unknown>): ExtensionAction
    removeShortcut(input?: Record<string, unknown>, options?: Record<string, unknown>): ExtensionAction
    setPaletteShortcut(title?: string, options?: Record<string, unknown>): ExtensionAction
    native(title: string, nativeAction: unknown, options?: Record<string, unknown>): ExtensionAction
    camera: Record<'switchDevice' | 'nextDevice' | 'previousDevice' | 'toggleMuted' | 'toggleControls', (title?: string, options?: Record<string, unknown>) => ExtensionAction>
  }
  navigation: {
    push(view: ExtensionView | null): unknown
    replace(view: ExtensionView | null): unknown
    pop(): unknown
    run(action: ExtensionAction): unknown
  }
  desktop: {
    clipboard?: Record<string, (...args: any[]) => any>
    selection: Record<string, (...args: any[]) => any>
    apps?: Record<string, (...args: any[]) => any>
    files?: {
      find(roots: string[], options?: Record<string, unknown>): Promise<ExtensionFile[]>
      findImages(roots: string[], options?: Record<string, unknown>): Promise<ExtensionFile[]>
      findVideos(roots: string[], options?: Record<string, unknown>): Promise<ExtensionFile[]>
      findMedia(roots: string[], options?: Record<string, unknown>): Promise<ExtensionFile[]>
      openWithApps(filePath: string): Promise<any[]>
      open(filePath: string): unknown
      reveal(filePath: string): unknown
      preview(filePath: string): unknown
      readText(filePath: string): Promise<string>
      toFileUrl(filePath: string): string
    }
    shell?: {
      openExternal(url: string): unknown
      exec(command: string, args?: string[], options?: Record<string, unknown>): Promise<{ stdout: string; stderr: string; exitCode: number }>
      script(script: string, options?: Record<string, unknown>): Promise<{ stdout: string; stderr: string; exitCode: number }>
      appleScript(script: string, options?: Record<string, unknown>): Promise<{ stdout: string; stderr: string; exitCode: number }>
      which(command: string): Promise<{ stdout: string; stderr: string; exitCode: number }>
    }
  }
  storage: {
    get<T = unknown>(key: string): Promise<T | undefined>
    set<T = unknown>(key: string, value: T): Promise<T>
    delete(key: string): Promise<void>
    clear(): Promise<void>
    memo<T = unknown>(key: string, ttlMs: number, loader: () => Promise<T> | T): Promise<T>
    memoStale<T = unknown>(key: string, ttlMs: number, staleTtlMs: number, loader: () => Promise<T> | T): Promise<T>
  }
  settings: Record<string, (...args: any[]) => any>
  logs: Record<'debug' | 'info' | 'warn' | 'error', (message: string, data?: unknown) => void>
  cache: Record<string, (...args: any[]) => any>
  views: { refresh(): ExtensionAction; invalidate(): void }
  state: Record<string, unknown>
  ai?: Record<string, (...args: any[]) => any>
  aiBuilder?: Record<string, (...args: any[]) => any>
  extensions: Record<string, unknown>
}

export type ExtensionCommand = {
  id: string
  actionId?: string
  title: string
  subtitle?: string
  aliases?: string[]
  icon?: string
  score?: number
  shortcut?: string
  shortcutScope?: 'local' | 'global'
  globalShortcut?: string
  background?: boolean
  dismissAfterRun?: 'auto'
  run(ctx: ExtensionContext): unknown | Promise<unknown>
}

export type NevermindExtension = {
  id: string
  title: string
  subtitle?: string
  permissions?: ExtensionPermission[]
  commands?: ExtensionCommand[]
  rootItems?(ctx: ExtensionContext): ExtensionItem[] | Promise<ExtensionItem[]> | { items: ExtensionItem[] } | Promise<{ items: ExtensionItem[] }>
  searchItems?(ctx: ExtensionContext, query: string): ExtensionItem[] | Promise<ExtensionItem[]> | { items: ExtensionItem[] } | Promise<{ items: ExtensionItem[] }>
}

declare global {
  const module: { exports: unknown }
}

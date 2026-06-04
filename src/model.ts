import type { ReactNode } from 'react'

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

export type ActionDismissBehavior = 'manual' | 'immediate' | 'after-success'
export type ActionLoadingBehavior = 'view' | 'none'
export type ActionExecutionLocation = 'main' | 'renderer'

export type CommandActionDefinition = {
  description: string
  dismiss: ActionDismissBehavior
  loading: ActionLoadingBehavior
  execute: ActionExecutionLocation
  inline?: boolean
}

export const ACTION_DEFINITIONS = {
  openPath: { description: 'Open with the default app', dismiss: 'immediate', loading: 'none', execute: 'main' },
  revealPath: { description: 'Reveal in file manager', dismiss: 'immediate', loading: 'none', execute: 'main' },
  quickLook: { description: 'Preview this file', dismiss: 'manual', loading: 'view', execute: 'main' },
  openWith: { description: 'Open with another app', dismiss: 'immediate', loading: 'none', execute: 'main' },
  openUrl: { description: 'Open URL', dismiss: 'immediate', loading: 'none', execute: 'main' },
  copyText: { description: 'Copy text to the clipboard', dismiss: 'immediate', loading: 'none', execute: 'main' },
  copyImage: { description: 'Copy image to the clipboard', dismiss: 'immediate', loading: 'none', execute: 'main' },
  pasteText: { description: 'Paste into the frontmost app', dismiss: 'immediate', loading: 'none', execute: 'main' },
  trash: { description: 'Move to Trash', dismiss: 'manual', loading: 'view', execute: 'main' },
  pushView: { description: 'Open nested view', dismiss: 'manual', loading: 'view', execute: 'main' },
  replaceView: { description: 'Open nested view', dismiss: 'manual', loading: 'view', execute: 'main' },
  popView: { description: 'Go back', dismiss: 'manual', loading: 'view', execute: 'main' },
  previewClipboardItem: { description: 'Preview clipboard item', dismiss: 'manual', loading: 'none', execute: 'renderer' },
  removeClipboardHistory: { description: 'Remove clipboard history entries', dismiss: 'manual', loading: 'none', execute: 'main' },
  runExtensionAction: { description: 'Run action', dismiss: 'immediate', loading: 'none', execute: 'main' },
  shellExec: { description: 'Run system command', dismiss: 'manual', loading: 'view', execute: 'main' },
  shellScript: { description: 'Run system command', dismiss: 'manual', loading: 'view', execute: 'main' },
  checkForUpdates: { description: 'Check for updates', dismiss: 'manual', loading: 'view', execute: 'main' },
  downloadUpdate: { description: 'Download update', dismiss: 'manual', loading: 'view', execute: 'main' },
  installUpdate: { description: 'Install update', dismiss: 'manual', loading: 'view', execute: 'main' },
  lockScreen: { description: 'Lock screen', dismiss: 'immediate', loading: 'none', execute: 'main' },
  sleepSystem: { description: 'Sleep system', dismiss: 'immediate', loading: 'none', execute: 'main' },
  restartSystem: { description: 'Restart system', dismiss: 'immediate', loading: 'none', execute: 'main' },
  quitApp: { description: 'Quit app', dismiss: 'immediate', loading: 'none', execute: 'main' },
  openSystemSettings: { description: 'Open system settings', dismiss: 'immediate', loading: 'none', execute: 'main' },
  openKeyboardSettings: { description: 'Open keyboard settings', dismiss: 'immediate', loading: 'none', execute: 'main' },
  toggleSetting: { description: 'Change setting', dismiss: 'immediate', loading: 'none', execute: 'main', inline: true },
  recordShortcut: { description: 'Record shortcut', dismiss: 'manual', loading: 'none', execute: 'renderer' },
  setActionShortcut: { description: 'Set shortcut', dismiss: 'manual', loading: 'none', execute: 'main', inline: true },
  setSettingShortcut: { description: 'Set shortcut setting', dismiss: 'manual', loading: 'none', execute: 'main', inline: true },
  removeShortcut: { description: 'Remove shortcut', dismiss: 'immediate', loading: 'none', execute: 'main', inline: true },
  setActionAlias: { description: 'Set alias', dismiss: 'manual', loading: 'none', execute: 'main', inline: true },
  removeActionAlias: { description: 'Remove alias', dismiss: 'manual', loading: 'none', execute: 'main', inline: true },
  duplicateCreatedAction: { description: 'Duplicate action', dismiss: 'manual', loading: 'view', execute: 'main' },
  removeCreatedAction: { description: 'Remove action', dismiss: 'manual', loading: 'view', execute: 'main' },
  clearActionOverride: { description: 'Restore original action', dismiss: 'manual', loading: 'none', execute: 'main', inline: true },
  nativeAction: { description: 'Run command', dismiss: 'manual', loading: 'none', execute: 'main' },
} as const satisfies Record<string, CommandActionDefinition>

export type CommandActionType = keyof typeof ACTION_DEFINITIONS

export type CommandApp = { name?: string; path?: string }

export type CommandAction = {
  type: CommandActionType
  title: string
  subtitle?: string
  description?: string
  path?: string
  paths?: string[]
  app?: CommandApp
  appPath?: string
  url?: string
  text?: string
  imageDataUrl?: string
  imagePath?: string
  view?: CommandView
  handlerId?: string
  shortcut?: string
  shortcutScope?: 'local' | 'global'
  nativeAction?: unknown
  settingId?: string
  action?: unknown
  actionId?: string
  targetAction?: unknown
  alias?: string
  accelerator?: string
  clipboardType?: string
  clipboardHistoryRange?: 'item' | 'last-hour' | 'last-day' | 'all'
  clipboardHistoryItemId?: string
  videoUrl?: string
  filePath?: string
  thumbnailUrl?: string
  aiChatId?: string
  query?: string
  extensionFile?: string
  command?: string
  args?: string[]
  script?: string
  options?: Record<string, unknown>
  formValues?: Record<string, string | boolean>
  selectedItemId?: string
  value?: string
  submenu?: CommandActionPanel
  lazySubmenu?: boolean
  style?: 'regular' | 'destructive'
  requiresConfirmation?: boolean
  confirmMessage?: string
  confirmLabel?: string
  cancelLabel?: string
  dismissAfterRun?: 'auto'
  executionId?: string
}

export type CommandActionSection = {
  title?: string
  actions: CommandAction[]
  lazyActions?: CommandAction[]
  isLoading?: boolean
}

export type CommandActionPanel = {
  title?: string
  sections: CommandActionSection[]
}

export type CommandItemAccessory = { text?: string; icon?: string }
export type CommandItemForeground = 'yellow' | 'blue' | 'purple' | 'green' | 'red' | 'orange' | 'pink'
export type CommandItemAppearance = { foreground?: CommandItemForeground }

export type CommandItemPatch = Partial<Omit<CommandItem, 'id'>> & { id: string }

export type CommandItem = {
  id: string
  title: string
  subtitle?: string
  accessories?: CommandItemAccessory[]
  shortcut?: string
  keywords?: string[]
  text?: string
  icon?: string
  image?: string
  video?: string
  videoUrl?: string
  path?: string
  filePath?: string
  fileUrl?: string
  primaryAction?: CommandAction
  actions?: CommandAction[]
  actionPanel?: CommandActionPanel
  actionPanelVisibility?: 'visible' | 'menu' | 'hidden'
  appearance?: CommandItemAppearance
}

export type CommandItemSection = {
  title?: string
  subtitle?: string
  items: CommandItem[]
}

export type CommandViewPatch = {
  items?: CommandItemPatch[]
  mode?: 'patch' | 'replace' | 'prepend' | 'append'
  removeItemIds?: string[]
  isLoading?: boolean
  selectedItemId?: string
}

export type CommandView = {
  id?: string
  type: 'list' | 'grid' | 'preview' | 'chat' | 'form' | 'progress' | 'webview' | 'camera'
  title: string
  size?: 'default' | 'large'
  image?: string
  video?: string
  videoUrl?: string
  deviceId?: string
  showDeviceSwitcher?: boolean
  muted?: boolean
  controls?: boolean
  aiChat?: boolean
  chatId?: string
  initialPrompt?: string
  subtitle?: string
  content?: string
  html?: string
  items?: CommandItem[]
  sections?: CommandItemSection[]
  isLoading?: boolean
  emptyView?: { title?: string; subtitle?: string }
  searchBarPlaceholder?: string
  presentation?: 'root' | 'stacked' | 'preview'
  selectedItemId?: string
  onSelectionChange?: CommandAction
  pagination?: { hasMore?: boolean; pageSize?: number; onLoadMore?: CommandAction }
  searchAccessory?: { id?: string; tooltip?: string; value?: string; items: { title: string; value: string }[]; onChange?: CommandAction }
  refresh?: { intervalMs?: number; action?: CommandAction; mode?: CommandViewPatch['mode'] }
  messages?: { role: 'user' | 'assistant' | 'system'; content: string }[]
  fields?: { id: string; label: string; type?: string; value?: string; placeholder?: string; required?: boolean }[]
  submitAction?: CommandAction
  steps?: { title: string; status?: string }[]
  actions?: CommandAction[]
  actionPanel?: CommandActionPanel
  actionPanelVisibility?: 'visible' | 'menu' | 'hidden'
  layout?: 'square' | 'wide' | 'compact'
  aspectRatio?: string | number
  columns?: number
}

export type RowModel = {
  value: string
  icon: ReactNode
  title: string
  subtitle?: string
  shortcut?: string
  extras?: string[]
  className?: string
  onSelect: () => void
}

export type CustomizableCommandAction = { kind?: string; customizable?: boolean }

const CUSTOMIZABLE_ACTION_KINDS = new Set(['app', 'builtin', 'clipboard-history', 'extension-command'])

export function canCustomizeCommandAction(action: CustomizableCommandAction | null | undefined) {
  return Boolean(action?.customizable) || CUSTOMIZABLE_ACTION_KINDS.has(String(action?.kind || ''))
}

export function actionPanelFromActions(actions?: CommandAction[], title?: string): CommandActionPanel | undefined {
  if (!actions?.length) return undefined
  return { title, sections: [{ actions }] }
}

export function actionsFromPanel(panel?: CommandActionPanel, fallbackActions: CommandAction[] = []) {
  return panel?.sections.flatMap((section) => section.actions) || fallbackActions
}

export function actionDefinition(action: Pick<CommandAction, 'type'> | null | undefined) {
  return action?.type ? ACTION_DEFINITIONS[action.type] : undefined
}

export function actionDescription(action: CommandAction) {
  if (action.subtitle || action.description) return action.subtitle || action.description
  if (action.type === 'quickLook' || action.type === 'revealPath') return action.title || actionDefinition(action)?.description || 'Run action'
  if (action.type === 'openWith') return action.app?.name ? `Open with ${action.app.name}` : actionDefinition(action)?.description || 'Run action'
  return actionDefinition(action)?.description || 'Run action'
}

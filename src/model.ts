import type { ReactNode } from 'react'

export type CommandActionType = 'openPath' | 'revealPath' | 'quickLook' | 'openWith' | 'openUrl' | 'copyText' | 'copyImage' | 'pasteText' | 'trash' | 'pushView' | 'replaceView' | 'popView' | 'previewClipboardItem' | 'runExtensionAction' | 'shellExec' | 'shellScript' | 'checkForUpdates' | 'downloadUpdate' | 'installUpdate' | 'lockScreen' | 'sleepSystem' | 'restartSystem' | 'quitApp' | 'openSystemSettings' | 'openAiChats' | 'openAiChat' | 'startAiBuilderChat' | 'tweakExtensionWithAi' | 'removeAiChat' | 'toggleSetting' | 'recordShortcut' | 'removeShortcut' | 'nativeAction'

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
  clipboardType?: string
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
  submenu?: CommandActionPanel
  lazySubmenu?: boolean
  style?: 'regular' | 'destructive'
  requiresConfirmation?: boolean
  dismissAfterRun?: 'auto'
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

export type CommandItemPatch = Partial<Omit<CommandItem, 'id'>> & { id: string }

export type CommandItem = {
  id: string
  title: string
  subtitle?: string
  accessories?: CommandItemAccessory[]
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
  actionPanelVisibility?: 'visible' | 'hidden'
}

export type CommandItemSection = {
  title?: string
  subtitle?: string
  items: CommandItem[]
}

export type CommandViewPatch = {
  items?: CommandItemPatch[]
  mode?: 'patch' | 'replace' | 'prepend'
}

export type CommandView = {
  id?: string
  type: 'list' | 'grid' | 'preview' | 'chat' | 'form' | 'progress' | 'webview'
  title: string
  size?: 'default' | 'large'
  image?: string
  video?: string
  videoUrl?: string
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
  actionPanelVisibility?: 'visible' | 'hidden'
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

export function actionPanelFromActions(actions?: CommandAction[], title?: string): CommandActionPanel | undefined {
  if (!actions?.length) return undefined
  return { title, sections: [{ actions }] }
}

export function actionsFromPanel(panel?: CommandActionPanel, fallbackActions: CommandAction[] = []) {
  return panel?.sections.flatMap((section) => section.actions) || fallbackActions
}

export function actionDescription(action: CommandAction) {
  if (action.subtitle || action.description) return action.subtitle || action.description
  if (action.type === 'quickLook') return action.title || 'Preview this file'
  if (action.type === 'openWith') return action.app?.name ? `Open with ${action.app.name}` : 'Open with another app'
  if (action.type === 'openPath') return 'Open with the default app'
  if (action.type === 'revealPath') return action.title || 'Reveal in file manager'
  if (action.type === 'copyText') return 'Copy text to the clipboard'
  if (action.type === 'copyImage') return 'Copy image to the clipboard'
  if (action.type === 'pasteText') return 'Paste into the frontmost app'
  if (action.type === 'trash') return 'Move to Trash'
  if (action.type === 'pushView' || action.type === 'replaceView') return 'Open nested view'
  if (action.type === 'popView') return 'Go back'
  if (action.type === 'previewClipboardItem') return 'Preview clipboard item'
  if (action.type === 'shellExec' || action.type === 'shellScript') return 'Run system command'
  if (action.type === 'checkForUpdates') return 'Check for updates'
  if (action.type === 'downloadUpdate') return 'Download update'
  if (action.type === 'installUpdate') return 'Install update'
  if (action.type === 'lockScreen') return 'Lock screen'
  if (action.type === 'sleepSystem') return 'Sleep system'
  if (action.type === 'restartSystem') return 'Restart system'
  if (action.type === 'quitApp') return 'Quit app'
  if (action.type === 'openSystemSettings') return 'Open system settings'
  if (action.type === 'openAiChats') return 'Open AI chats'
  if (action.type === 'openAiChat') return 'Open AI chat'
  if (action.type === 'startAiBuilderChat') return 'Start AI builder chat'
  if (action.type === 'tweakExtensionWithAi') return 'Tweak extension with AI'
  if (action.type === 'removeAiChat') return 'Remove AI chat'
  if (action.type === 'toggleSetting') return 'Change setting'
  if (action.type === 'recordShortcut') return 'Record shortcut'
  if (action.type === 'removeShortcut') return 'Remove shortcut'
  if (action.type === 'nativeAction') return 'Run command'
  return 'Run action'
}

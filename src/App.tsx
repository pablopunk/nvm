import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Command } from 'cmdk'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  AppWindow,
  Calculator,
  Clipboard,
  CornerDownLeft,
  Folder,
  Globe,
  Grid2X2,
  Keyboard,
  Lock,
  Moon,
  Power,
  RotateCcw,
  Search,
  Settings,
  Sparkles,
  Square,
  Tag,
  Trash2,
  Wand2,
  Zap,
} from 'lucide-react'
import { ActionPanelView, ChatView, CommandRow, CommandTile, DetailView, EmptyState, FormView, GridView, ListView, ProgressView, Toast, shortcutLabel } from './ui'
import { acceleratorFromKeyboardEvent, keyNameForShortcut, normalizedShortcut } from './shortcuts'
import { actionDescription, actionsFromPanel, actionPanelFromActions, type CommandAction, type CommandActionPanel, type CommandItem, type CommandView } from './model'

type ActionKind =
  | 'open-url'
  | 'web-search'
  | 'app'
  | 'clipboard'
  | 'clipboard-history'
  | 'keyboard-shortcuts'
  | 'file'
  | 'ai-placeholder'
  | 'ai-chat'
  | 'builtin'
  | 'calculate'
  | 'extension-command'

type ActionIcon =
  | 'globe'
  | 'search'
  | 'app'
  | 'clipboard'
  | 'sparkles'
  | 'lock'
  | 'moon'
  | 'restart'
  | 'settings'
  | 'folder'
  | 'power'
  | 'calculator'
  | 'bolt'
  | 'grid'
  | 'keyboard'

type AppInfo = {
  name?: string
  path?: string
}

type Action = {
  id: string
  kind: ActionKind
  title: string
  subtitle: string
  icon: ActionIcon
  score: number
  iconUrl?: string | null
  url?: string
  query?: string
  result?: string
  text?: string
  clipboardType?: 'text' | 'image'
  imageDataUrl?: string
  thumbnailUrl?: string
  filePath?: string
  defaultActionId?: string
  isOverridden?: boolean
  overrideSummary?: string
  app?: AppInfo
  extensionId?: string
  commandId?: string
  aiChatId?: string
  removable?: boolean
  shortcut?: string
}

type ExtensionViewAction = CommandAction
type ExtensionViewItem = CommandItem
type ExtensionView = CommandView

type SaveResult = {
  ok: boolean
  message: string
}

type ShortcutRecord = {
  actionId: string
  accelerator: string
  action: Action
}

declare global {
  interface Window {
    nvm: {
      search: (query: string, options?: { clipboardOnly?: boolean }) => Promise<Action[]>
      execute: (action: Action) => Promise<{ view?: ExtensionView } | void>
      runViewAction: (action: ExtensionViewAction) => Promise<{ view?: ExtensionView; navigation?: 'push' | 'replace' | 'pop'; toast?: { message: string; tone?: 'default' | 'error' } } | void>
      startFileDrag: (filePath: string) => void
      sendAiMessage: (message: string, chatId?: string) => Promise<void>
      abortAiChat: (chatId?: string) => Promise<void>
      resetAiChat: (chatId?: string) => Promise<void>
      setAlias: (action: Action, alias: string) => Promise<SaveResult>
      setShortcut: (action: Action, shortcut: string) => Promise<SaveResult>
      getShortcuts: () => Promise<ShortcutRecord[]>
      removeShortcut: (actionId: string) => Promise<SaveResult>
      suspendShortcuts: () => Promise<void>
      resumeShortcuts: () => Promise<void>
      setOverride: (action: Action, instruction: string) => Promise<SaveResult>
      clearOverride: (action: Action) => Promise<SaveResult>
      removeCreatedAction: (action: Action) => Promise<SaveResult>
      getAppIcon: (appPath: string) => Promise<string | null>
      setPaletteMode: (mode: 'default' | 'ai-chat') => Promise<void>
      hide: () => Promise<void>
      shortcutReady: () => Promise<void>
      onShown: (callback: () => void) => () => void
      onShortcutShown: (callback: () => void) => () => void
      onHidden: (callback: () => void) => () => void
      onAppsIndexed: (callback: (count: number) => void) => () => void
      onClipboardChanged: (callback: () => void) => () => void
      onOpenClipboardHistory: (callback: (payload?: { openedFromHidden?: boolean }) => void) => () => void
      onOpenActionView: (callback: (payload?: { view?: ExtensionView }) => void) => () => void
      onAiChatEvent: (callback: (event: { type: string; text?: string; message?: string; name?: string; chatId?: string; label?: string; data?: unknown }) => void) => () => void
    }
  }
}

const SEARCH_PLACEHOLDERS = [
  'Watcha gonna do?',
  'I cannot do that... Nevermind, I can now',
  'Make it happen',
]

const iconFor = {
  globe: Globe,
  search: Search,
  app: AppWindow,
  clipboard: Clipboard,
  sparkles: Sparkles,
  lock: Lock,
  moon: Moon,
  restart: RotateCcw,
  settings: Settings,
  folder: Folder,
  power: Power,
  calculator: Calculator,
  bolt: Zap,
  grid: Grid2X2,
  keyboard: Keyboard,
}

export function App() {
  const inputRef = useRef<HTMLInputElement>(null)
  const chatMessagesRef = useRef<HTMLDivElement>(null)
  const aiInputRef = useRef<HTMLTextAreaElement>(null)
  const requestedIcons = useRef(new Set<string>())
  const aiChatOpenRef = useRef(false)
  const aiChatIdRef = useRef<string | undefined>(undefined)
  const runningViewActionsRef = useRef(new Set<string>())
  const lastLocalShortcutRef = useRef<string | null>(null)
  const [query, setQuery] = useState('')
  const [actions, setActions] = useState<Action[]>([])
  const [iconUrls, setIconUrls] = useState<Record<string, string | null>>({})
  const [selectedValue, setSelectedValue] = useState('')
  const [optionsFor, setOptionsFor] = useState<Action | null>(null)
  const [extensionItemOptionsFor, setExtensionItemOptionsFor] = useState<ExtensionViewItem | null>(null)
  const [actionSubmenuFor, setActionSubmenuFor] = useState<{ title: string; panel: CommandActionPanel } | null>(null)
  const [confirmRemoveFor, setConfirmRemoveFor] = useState<Action | null>(null)
  const [previewFor, setPreviewFor] = useState<Action | null>(null)
  const [extensionView, setExtensionView] = useState<ExtensionView | null>(null)
  const [extensionViewBackStack, setExtensionViewBackStack] = useState<ExtensionView[]>([])
  const [aiMessages, setAiMessages] = useState<NonNullable<ExtensionView['messages']>>([])
  const [aiInput, setAiInput] = useState('')
  const [aiBusy, setAiBusy] = useState(false)
  const [toast, setToast] = useState<{ message: string; tone?: 'default' | 'error' } | null>(null)
  const [placeholderIndex, setPlaceholderIndex] = useState(SEARCH_PLACEHOLDERS.length - 1)
  const [refreshNonce, setRefreshNonce] = useState(0)
  const [clipboardMode, setClipboardMode] = useState(false)
  const [clipboardOpenedFromHidden, setClipboardOpenedFromHidden] = useState(false)
  const [clipboardQuery, setClipboardQuery] = useState('')
  const [pendingShortcutReveal, setPendingShortcutReveal] = useState(false)
  const [childQuery, setChildQuery] = useState('')
  const [shortcutFor, setShortcutFor] = useState<Action | null>(null)
  const [recordedShortcut, setRecordedShortcut] = useState('')
  const [shortcutManagerOpen, setShortcutManagerOpen] = useState(false)
  const [shortcutRecords, setShortcutRecords] = useState<ShortcutRecord[]>([])
  const [shortcutOptionsFor, setShortcutOptionsFor] = useState<ShortcutRecord | null>(null)
  const [formValues, setFormValues] = useState<Record<string, string | boolean>>({})

  useEffect(() => {
    const focusInput = () => {
      const input = inputRef.current
      if (!input) return
      input.focus()
      if (!input.readOnly) input.select()
      else input.setSelectionRange(input.value.length, input.value.length)
    }

    const stopShown = window.nvm.onShown(() => {
      setOptionsFor(null)
      setExtensionItemOptionsFor(null)
      setConfirmRemoveFor(null)
      setPreviewFor(null)
      setChildQuery('')
      setShortcutFor(null)
      setRecordedShortcut('')
      setShortcutManagerOpen(false)
      setShortcutOptionsFor(null)
      if (!aiChatOpenRef.current) {
        setExtensionView(null)
        setExtensionViewBackStack([])
        setAiMessages([])
      }
      setClipboardMode(false)
      setClipboardQuery('')
      setRefreshNonce((nonce) => nonce + 1)
      setPlaceholderIndex((index) => (index + 1) % SEARCH_PLACEHOLDERS.length)
      if (!aiChatOpenRef.current) setExtensionViewBackStack([])
      setClipboardOpenedFromHidden(false)
      requestAnimationFrame(focusInput)
      window.setTimeout(focusInput, 50)
    })
    const stopShortcutShown = window.nvm.onShortcutShown(() => {
      requestAnimationFrame(focusInput)
      window.setTimeout(focusInput, 50)
    })
    const stopHidden = window.nvm.onHidden(() => {
      setQuery('')
      setOptionsFor(null)
      setExtensionItemOptionsFor(null)
      setConfirmRemoveFor(null)
      setPreviewFor(null)
      setChildQuery('')
      setShortcutFor(null)
      setRecordedShortcut('')
      setShortcutManagerOpen(false)
      setShortcutOptionsFor(null)
      if (!aiChatOpenRef.current) {
        setExtensionView(null)
        setExtensionViewBackStack([])
        setAiMessages([])
      }
      setExtensionViewBackStack([])
      setClipboardMode(false)
      setClipboardOpenedFromHidden(false)
      setClipboardQuery('')
    })
    const stopApps = window.nvm.onAppsIndexed(() => {})
    const stopClipboard = window.nvm.onClipboardChanged(() => setRefreshNonce((nonce) => nonce + 1))
    const stopOpenClipboardHistory = window.nvm.onOpenClipboardHistory((payload) => {
      openClipboardHistory({ openedFromHidden: Boolean(payload?.openedFromHidden) })
      markShortcutReady(Boolean(payload?.revealWhenReady))
    })
    const stopOpenActionView = window.nvm.onOpenActionView(async (payload) => {
      if (!payload?.view) return
      setOptionsFor(null)
      setPreviewFor(null)
      if (payload.view.aiChat) await openAiChat(payload.view)
      else showExtensionView(payload.view, 'root')
      markShortcutReady(Boolean(payload?.revealWhenReady))
    })
    const stopAi = window.nvm.onAiChatEvent((event) => {
      if (event.type === 'debug') console.debug('[Nevermind AI]', event.label, event.data)
      if (event.chatId && event.chatId !== aiChatIdRef.current) return
      if (event.type === 'start') setAiBusy(true)
      if (event.type === 'done' || event.type === 'error' || event.type === 'aborted') setAiBusy(false)
      if (event.type === 'delta' && event.text) appendAiDelta(event.text)
      if (event.type === 'tool_start' && event.name) appendAiMessage('system', `Using ${event.name}…`)
      if (event.type === 'error' && event.message) appendAiMessage('system', event.message)
    })
    return () => {
      stopShown()
      stopShortcutShown()
      stopHidden()
      stopApps()
      stopClipboard()
      stopOpenClipboardHistory()
      stopOpenActionView()
      stopAi()
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const timer = window.setTimeout(async () => {
      const next = await window.nvm.search(clipboardMode ? clipboardQuery : query, { clipboardOnly: clipboardMode })
      if (!cancelled) setActions(next)
    }, 20)

    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [query, clipboardQuery, refreshNonce, clipboardMode])

  useEffect(() => {
    if (shortcutFor) setSelectedValue('shortcut:save')
    else if (shortcutOptionsFor) setSelectedValue(getShortcutOptionRows()[0]?.value ?? '')
    else if (shortcutManagerOpen) setSelectedValue(getShortcutRows()[0]?.value ?? '')
    else if (clipboardMode) setSelectedValue(actions[0]?.id ?? '')
    else if (confirmRemoveFor) setSelectedValue(getConfirmActionRows()[0]?.value ?? '')
    else if (actionSubmenuFor) setSelectedValue(actionPanelRows(actionSubmenuFor.panel, [], 'action-submenu', true).find((row) => !row.sectionHeader)?.value ?? '')
    else if (extensionItemOptionsFor) setSelectedValue(getExtensionItemActionRows()[0]?.value ?? '')
    else if (optionsFor) setSelectedValue(getOptionActionRows()[0]?.value ?? '')
    else if (previewFor) setSelectedValue('preview')
    else if (extensionView && isFilterableExtensionView) setSelectedValue(extensionView.selectedItemId || filterExtensionItems(allViewItems(extensionView))[0]?.id || '')
    else if (extensionView?.actions?.length) setSelectedValue(`extension-view:0:${extensionView.actions[0].type}:${extensionView.actions[0].title}`)
    else if (extensionView) setSelectedValue('preview')
    else setSelectedValue(actions[0]?.id ?? '')
  }, [actions, actionSubmenuFor, childQuery, clipboardMode, confirmRemoveFor, extensionItemOptionsFor, optionsFor, previewFor, extensionView, shortcutFor, shortcutManagerOpen, shortcutRecords, shortcutOptionsFor])

  useEffect(() => {
    setChildQuery('')
  }, [actionSubmenuFor?.title, confirmRemoveFor?.id, extensionItemOptionsFor?.id, extensionView?.title, extensionView?.type, optionsFor?.id, previewFor?.id])

  useEffect(() => {
    if (extensionView?.type !== 'form') return
    setFormValues(Object.fromEntries((extensionView.fields || []).map((field) => [field.id, field.type === 'checkbox' ? Boolean(field.value) : field.value || ''])))
  }, [extensionView])

  useEffect(() => {
    if (!pendingShortcutReveal || (!extensionView && !clipboardMode)) return
    const frame = requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        window.nvm.shortcutReady()
        setPendingShortcutReveal(false)
      })
    })
    return () => cancelAnimationFrame(frame)
  }, [pendingShortcutReveal, extensionView, clipboardMode])

  useEffect(() => {
    const isAiChat = Boolean(extensionView?.aiChat)
    aiChatOpenRef.current = isAiChat
    aiChatIdRef.current = extensionView?.aiChat ? extensionView.chatId : undefined
    window.nvm.setPaletteMode(isAiChat ? 'ai-chat' : 'default')
  }, [extensionView])

  useEffect(() => {
    if (extensionView?.aiChat) {
      chatMessagesRef.current?.scrollTo({ top: chatMessagesRef.current.scrollHeight })
    }
  }, [aiMessages, aiBusy, extensionView])

  useLayoutEffect(() => {
    resizeAiInput()
  }, [aiInput, extensionView?.aiChat])

  useEffect(() => {
    if (!shortcutFor) return
    window.nvm.suspendShortcuts()
    return () => {
      window.nvm.resumeShortcuts()
    }
  }, [shortcutFor?.id])

  useEffect(() => {
    for (const action of actions) {
      if (action.kind !== 'app' || !action.app?.path || requestedIcons.current.has(action.id)) continue

      requestedIcons.current.add(action.id)
      window.nvm.getAppIcon(action.app.path).then((iconUrl) => {
        setIconUrls((current) => ({ ...current, [action.id]: iconUrl }))
      })
    }
  }, [actions])

  const selectedAction = useMemo(
    () => actions.find((action) => action.id === selectedValue),
    [actions, selectedValue],
  )
  const createAction = useMemo(
    () => actions.find((action) => action.kind === 'ai-placeholder'),
    [actions],
  )
  const isFilterableExtensionView = extensionView?.type === 'list' || extensionView?.type === 'grid'
  const isRootLikeExtensionView = extensionView?.id === 'clipboard-history'
  const isFilterableChildOpen = Boolean(actionSubmenuFor || confirmRemoveFor || extensionItemOptionsFor || optionsFor || shortcutManagerOpen || isFilterableExtensionView)
  const isChildOpen = Boolean(shortcutFor || shortcutOptionsFor || shortcutManagerOpen || actionSubmenuFor || confirmRemoveFor || extensionItemOptionsFor || optionsFor || previewFor || extensionView)
  const isVisuallyStacked = isChildOpen && !isRootLikeExtensionView
  const childPlaceholder = actionSubmenuFor ? `Filter ${actionSubmenuFor.title}` : shortcutOptionsFor ? `Actions for “${shortcutOptionsFor.action.title}”` : shortcutManagerOpen ? 'Filter keyboard shortcuts' : confirmRemoveFor ? 'Filter confirmation actions' : extensionItemOptionsFor ? `Filter actions for “${extensionItemOptionsFor.title}”` : optionsFor ? `Filter actions for “${optionsFor.title}”` : extensionView ? `Filter ${extensionView.title}` : ''
  const inputValue = shortcutFor ? recordedShortcut : clipboardMode ? clipboardQuery : isFilterableChildOpen ? childQuery : extensionView ? extensionView.title : previewFor ? previewFor.title : optionsFor && !query ? optionsFor.title : query
  const placeholder = shortcutFor ? 'Press a keyboard shortcut' : clipboardMode ? 'Search Clipboard History' : isFilterableChildOpen ? (extensionView?.searchBarPlaceholder || childPlaceholder) : SEARCH_PLACEHOLDERS[placeholderIndex]

  useEffect(() => {
    if (!isFilterableChildOpen && !shortcutFor) return
    requestAnimationFrame(() => inputRef.current?.focus())
  }, [confirmRemoveFor?.id, extensionItemOptionsFor?.id, extensionView?.title, extensionView?.type, isFilterableChildOpen, optionsFor?.id, shortcutFor?.id])

  function markShortcutReady(shouldReveal: boolean) {
    if (shouldReveal) setPendingShortcutReveal(true)
  }

  function showToast(message: string, tone: 'default' | 'error' = 'default') {
    setToast({ message, tone })
    window.setTimeout(() => setToast((current) => current?.message === message ? null : current), 2200)
  }

  function appendAiMessage(role: 'user' | 'assistant' | 'system', content: string) {
    setAiMessages((messages) => [...messages, { role, content }])
  }

  function appendAiDelta(text: string) {
    setAiMessages((messages) => {
      const last = messages[messages.length - 1]
      if (last?.role === 'assistant') {
        return [...messages.slice(0, -1), { ...last, content: `${last.content}${text}` }]
      }
      return [...messages, { role: 'assistant', content: text }]
    })
  }

  function resizeAiInput(textarea = aiInputRef.current) {
    if (!textarea) return
    textarea.style.height = 'auto'
    textarea.style.height = `${Math.min(textarea.scrollHeight, 120)}px`
    textarea.style.overflowY = textarea.scrollHeight > 120 ? 'auto' : 'hidden'
  }

  async function sendAiPrompt(message: string, chatId = extensionView?.chatId) {
    const trimmed = message.trim()
    if (!trimmed || aiBusy) return
    appendAiMessage('user', trimmed)
    setAiInput('')
    await window.nvm.sendAiMessage(trimmed, chatId)
  }

  function showExtensionView(view: ExtensionView, navigation: 'root' | 'push' | 'replace' = 'replace') {
    if (navigation === 'root') setExtensionViewBackStack([])
    if (navigation === 'push' && extensionView) setExtensionViewBackStack((stack) => [...stack, extensionView])
    setExtensionView(view)
  }

  function popExtensionView() {
    setExtensionItemOptionsFor(null)
    setExtensionViewBackStack((stack) => {
      const previous = stack[stack.length - 1]
      if (previous) setExtensionView(previous)
      else setExtensionView(null)
      return stack.slice(0, -1)
    })
  }

  async function handleViewActionResult(result?: { view?: ExtensionView; navigation?: 'push' | 'replace' | 'pop'; toast?: { message: string; tone?: 'default' | 'error' } } | void) {
    if (!result) return
    if (result.toast) showToast(result.toast.message, result.toast.tone || 'default')
    if (result.navigation === 'pop') popExtensionView()
    else if (result.view?.aiChat) await openAiChat(result.view)
    else if (result.view) showExtensionView(result.view, result.navigation || 'push')
  }

  async function runViewAction(action: ExtensionViewAction) {
    if (action.requiresConfirmation && !window.confirm(`Run “${action.title}”?`)) return
    const actionKey = action.handlerId || `${action.type}:${action.title}:${action.path || action.url || action.text || ''}`
    if (runningViewActionsRef.current.has(actionKey)) return
    runningViewActionsRef.current.add(actionKey)
    try {
      const result = await window.nvm.runViewAction(action)
      await handleViewActionResult(result)
    } finally {
      runningViewActionsRef.current.delete(actionKey)
    }
  }

  function focusAiChatInput() {
    requestAnimationFrame(() => {
      chatMessagesRef.current?.scrollTo({ top: chatMessagesRef.current.scrollHeight })
      aiInputRef.current?.focus()
    })
  }

  async function openAiChat(view: ExtensionView) {
    showExtensionView(view, 'root')
    setAiMessages(view.messages || [])
    setAiInput('')
    focusAiChatInput()
    if (view.initialPrompt) {
      await window.nvm.resetAiChat(view.chatId)
      await sendAiPrompt(view.initialPrompt, view.chatId)
    }
    focusAiChatInput()
  }

  function openClipboardHistory(options: { openedFromHidden?: boolean } = {}) {
    setClipboardMode(true)
    setClipboardOpenedFromHidden(Boolean(options.openedFromHidden))
    setClipboardQuery('')
    setRefreshNonce((nonce) => nonce + 1)
    requestAnimationFrame(() => inputRef.current?.focus())
  }

  function closeClipboardHistory() {
    const shouldHide = clipboardOpenedFromHidden
    setClipboardMode(false)
    setClipboardOpenedFromHidden(false)
    setClipboardQuery('')
    if (shouldHide) window.nvm.hide()
    else requestAnimationFrame(() => inputRef.current?.focus())
  }

  async function run(action: Action) {
    if (action.kind === 'keyboard-shortcuts') {
      await openShortcutManager()
      return
    }
    const result = await window.nvm.execute(action)
    if (result?.view) {
      setOptionsFor(null)
      setPreviewFor(null)
      if (result.view.aiChat) await openAiChat(result.view)
      else showExtensionView(result.view, 'root')
    }
  }

  function acceleratorFromEvent(event: React.KeyboardEvent) {
    return acceleratorFromKeyboardEvent(event.nativeEvent)
  }

  async function refreshShortcuts() {
    setShortcutRecords(await window.nvm.getShortcuts())
  }

  async function openShortcutManager() {
    await refreshShortcuts()
    setShortcutManagerOpen(true)
    setShortcutFor(null)
    setOptionsFor(null)
    setPreviewFor(null)
  }

  function startShortcutRecorder(action: Action) {
    setShortcutFor(action)
    setRecordedShortcut('')
  }

  async function saveRecordedShortcut() {
    if (!shortcutFor || !recordedShortcut) return
    const result = await window.nvm.setShortcut(shortcutFor, recordedShortcut)
    showToast(result.message, result.ok ? 'default' : 'error')
    if (!result.ok) return
    setShortcutFor(null)
    setOptionsFor(null)
    setRefreshNonce((nonce) => nonce + 1)
    if (shortcutManagerOpen) await refreshShortcuts()
  }

  async function removeShortcut(record: ShortcutRecord) {
    const result = await window.nvm.removeShortcut(record.actionId)
    showToast(result.message, result.ok ? 'default' : 'error')
    if (result.ok) {
      setShortcutOptionsFor(null)
      await refreshShortcuts()
    }
  }

  async function setAlias() {
    if (!optionsFor) return
    const alias = window.prompt(`Alias for “${optionsFor.title}”`)
    if (!alias?.trim()) return
    const result = await window.nvm.setAlias(optionsFor, alias)
    if (!result.ok) showToast(result.message, 'error')
    else showToast(result.message)
    setOptionsFor(null)
  }

  async function setShortcut() {
    if (!optionsFor) return
    startShortcutRecorder(optionsFor)
  }

  async function quickLookOptionsAction() {
    if (!optionsFor?.filePath) return
    await window.nvm.runViewAction({ type: 'quickLook', title: 'Quick Look', path: optionsFor.filePath })
  }

  async function setOverride() {
    if (!optionsFor) return
    const instruction = window.prompt(
      `How should AI override “${optionsFor.title}”?`,
      optionsFor.overrideSummary || '',
    )
    if (!instruction?.trim()) return
    const result = await window.nvm.setOverride(optionsFor, instruction)
    showToast(result.message, result.ok ? 'default' : 'error')
    if (result.ok) setOptionsFor(null)
  }

  async function tweakActionWithAi(action: Action | null | undefined) {
    if (!action?.aiChatId) return
    const result = await window.nvm.execute({
      id: `ai-chat:${action.aiChatId}`,
      kind: 'ai-chat',
      title: action.title,
      subtitle: 'Tweak AI-created action',
      icon: 'sparkles',
      score: 0,
      aiChatId: action.aiChatId,
    })
    if (result?.view) {
      setOptionsFor(null)
      await openAiChat(result.view)
    }
  }

  async function tweakWithAi() {
    await tweakActionWithAi(optionsFor)
  }

  async function restoreOriginal() {
    if (!optionsFor) return
    const result = await window.nvm.clearOverride(optionsFor)
    showToast(result.message, result.ok ? 'default' : 'error')
    if (result.ok) setOptionsFor(null)
  }

  function askRemoveCreatedAction() {
    if (!optionsFor) return
    setConfirmRemoveFor(optionsFor)
  }

  async function confirmRemoveCreatedAction() {
    if (!confirmRemoveFor) return
    const result = await window.nvm.removeCreatedAction(confirmRemoveFor)
    showToast(result.message, result.ok ? 'default' : 'error')
    if (result.ok) {
      setConfirmRemoveFor(null)
      setOptionsFor(null)
      setRefreshNonce((nonce) => nonce + 1)
    }
  }

  const canOverride = Boolean(optionsFor?.defaultActionId)
  const canTweakWithAi = Boolean(optionsFor?.aiChatId && optionsFor.kind === 'extension-command')
  const canRemoveCreatedAction = Boolean(optionsFor?.kind === 'ai-chat' || optionsFor?.removable)
  const canCustomizeAction = Boolean(optionsFor && ['app', 'builtin', 'clipboard-history', 'extension-command'].includes(optionsFor.kind))
  const canQuickLookAction = Boolean(optionsFor?.filePath)
  const selectedExtensionItem = useMemo(() => {
    if (!extensionView) return null
    return allViewItems(extensionView).find((item) => item.id === selectedValue) || null
  }, [extensionView, selectedValue])
  const selectedShortcutRecord = useMemo(
    () => shortcutRecords.find((record) => `shortcut:${record.actionId}` === selectedValue) || null,
    [selectedValue, shortcutRecords],
  )
  const activeAction = optionsFor || previewFor || selectedAction

  useEffect(() => {
    if (!extensionView?.onSelectionChange || !selectedValue || !allViewItems(extensionView).some((item) => item.id === selectedValue)) return
    runViewAction({ ...extensionView.onSelectionChange, text: selectedValue })
  }, [extensionView?.onSelectionChange, selectedValue])

  function childScore(value: string | undefined, filter: string) {
    const text = value?.toLowerCase() || ''
    if (!filter) return 1
    if (text === filter) return 100
    if (text.startsWith(filter)) return 80
    if (text.includes(filter)) return 50
    let position = 0
    for (const character of filter) {
      position = text.indexOf(character, position)
      if (position === -1) return 0
      position += 1
    }
    return 20
  }

  function childMatches(...values: Array<string | undefined>) {
    const filter = childQuery.trim().toLowerCase()
    if (!filter) return true
    return Math.max(...values.map((value) => childScore(value, filter))) > 0
  }

  function allViewItems(view: ExtensionView) {
    return view.sections?.flatMap((section) => section.items) || view.items || []
  }

  function filterViewSections(view: ExtensionView) {
    if (view.sections?.length) return view.sections.map((section) => ({ ...section, items: filterExtensionItems(section.items) })).filter((section) => section.items.length > 0)
    return undefined
  }

  function filterExtensionItems(items: ExtensionViewItem[] = []) {
    return items.filter((item) => childMatches(
      item.title,
      item.subtitle,
      item.text,
      ...actionsFromPanel(item.actionPanel, item.actions || []).map((action) => action.title),
    ))
  }

  function getConfirmActionRows() {
    return [
      {
        value: 'confirm:remove',
        icon: <Trash2 size={18} />,
        title: 'Remove action',
        subtitle: confirmRemoveFor ? `Delete “${confirmRemoveFor.title}” from Nevermind` : '',
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
    ].filter((row) => childMatches(row.title, row.subtitle))
  }

  function iconForViewAction(action: ExtensionViewAction) {
    if (action.type === 'copyText' || action.type === 'copyImage' || action.type === 'pasteText') return <Clipboard size={18} />
    if (action.type === 'trash') return <Trash2 size={18} />
    if (action.type === 'revealPath' || action.type === 'openPath' || action.type === 'quickLook' || action.type === 'openWith') return <Folder size={18} />
    return <Globe size={18} />
  }

  function actionPanelRows(panel = extensionItemOptionsFor?.actionPanel, fallbackActions = extensionItemOptionsFor?.actions || [], prefix = 'extension-item', closeAfterSelect = true) {
    const sections = panel?.sections?.length ? panel.sections : [{ actions: fallbackActions }]
    return sections.flatMap((section, sectionIndex) => [
      ...(section.title ? [{ value: `${prefix}:section:${sectionIndex}`, title: section.title, subtitle: '', sectionHeader: true, onSelect: () => {}, className: 'actionSectionHeader' }] : []),
      ...(section.actions || []).map((action, index) => ({
        value: `${prefix}:${sectionIndex}:${index}:${action.type}:${action.title}`,
        icon: iconForViewAction(action),
        title: action.title,
        subtitle: action.submenu ? 'Open submenu' : actionDescription(action),
        shortcut: action.shortcut,
        onSelect: async () => {
          if (action.submenu) {
            setActionSubmenuFor({ title: action.title, panel: action.submenu })
            return
          }
          await runViewAction(action)
          if (closeAfterSelect) setExtensionItemOptionsFor(null)
        },
        className: action.style === 'destructive' ? 'result dangerResult' : 'result',
      })),
    ]).filter((row) => row.sectionHeader || childMatches(row.title, row.subtitle))
  }

  function getExtensionItemActionRows() {
    return actionPanelRows()
  }

  function getShortcutRows() {
    return shortcutRecords.map((record) => ({
      value: `shortcut:${record.actionId}`,
      icon: <Keyboard size={18} />,
      title: record.action.title,
      subtitle: record.accelerator,
      onSelect: () => startShortcutRecorder(record.action),
      className: 'result',
    })).filter((row) => childMatches(row.title, row.subtitle))
  }

  function getShortcutOptionRows() {
    if (!shortcutOptionsFor) return []
    return [
      {
        value: 'shortcut-option:change',
        icon: <Keyboard size={18} />,
        title: 'Change shortcut',
        subtitle: shortcutOptionsFor.accelerator,
        onSelect: () => {
          startShortcutRecorder(shortcutOptionsFor.action)
          setShortcutOptionsFor(null)
        },
        className: 'result',
      },
      {
        value: 'shortcut-option:remove',
        icon: <Trash2 size={18} />,
        title: 'Remove shortcut',
        subtitle: shortcutOptionsFor.action.title,
        onSelect: () => removeShortcut(shortcutOptionsFor),
        className: 'result dangerResult',
      },
    ].filter((row) => childMatches(row.title, row.subtitle))
  }

  function getShortcutRecorderRows() {
    return [
      {
        value: 'shortcut:save',
        icon: <Keyboard size={18} />,
        title: recordedShortcut || 'Press a shortcut',
        subtitle: recordedShortcut ? `Save shortcut for “${shortcutFor?.title}”` : 'Use at least one modifier, then press Enter',
        onSelect: saveRecordedShortcut,
        className: 'result',
      },
      {
        value: 'shortcut:cancel',
        icon: <RotateCcw size={18} />,
        title: 'Cancel',
        subtitle: 'Keep the current shortcut settings',
        onSelect: () => setShortcutFor(null),
        className: 'result',
      },
    ]
  }

  function getOptionActionRows() {
    return [
      {
        value: 'option:shortcut',
        icon: <Keyboard size={18} />,
        title: 'Set keyboard shortcut',
        subtitle: 'Run this action globally without opening Nevermind',
        onSelect: setShortcut,
        show: canCustomizeAction,
      },
      {
        value: 'option:preview',
        icon: <Search size={18} />,
        title: 'Preview',
        subtitle: 'Press → to preview an item',
        onSelect: () => optionsFor && setPreviewFor(optionsFor),
        show: true,
      },
      {
        value: 'option:quick-look',
        icon: <Search size={18} />,
        title: 'Quick Look',
        subtitle: 'Open native macOS Quick Look for this file',
        onSelect: quickLookOptionsAction,
        show: canQuickLookAction,
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
    ].filter((row) => row.show && childMatches(row.title, row.subtitle))
  }

  function renderChildEmpty(message = 'No actions found', subtitle?: string) {
    return <EmptyState icon={<Search size={24} />} title={message} subtitle={subtitle} />
  }

  function renderViewEmpty(view: ExtensionView, fallback = 'No items found') {
    return renderChildEmpty(view.emptyView?.title || fallback, view.emptyView?.subtitle)
  }

  function commandItemFromAction(action: Action): CommandItem {
    return {
      id: action.id,
      title: action.title,
      subtitle: action.isOverridden ? `AI override: ${action.overrideSummary}` : action.subtitle,
      image: action.thumbnailUrl || action.iconUrl || iconUrls[action.id],
      actionPanel: actionPanelFromActions([{ type: 'nativeAction', title: action.title, shortcut: action.shortcut, nativeAction: action }]),
    }
  }

  function renderNativeCommandItem(item: CommandItem, action: Action) {
    const Icon = iconFor[action.icon] ?? Sparkles
    const primaryAction = actionsFromPanel(item.actionPanel)[0]
    return <CommandRow
      key={item.id}
      value={item.id}
      icon={<span className={action.thumbnailUrl ? 'thumbnailIcon' : ''}>{item.image ? <img src={item.image} alt="" /> : <Icon size={18} />}</span>}
      title={item.title}
      subtitle={item.subtitle}
      shortcut={primaryAction?.shortcut}
      extras={action.kind === 'extension-command' && action.aiChatId ? ['Tab tweak'] : []}
      onSelect={() => run(action)}
    />
  }

  function renderActionResults() {
    return actions.map((action) => renderNativeCommandItem(commandItemFromAction(action), action))
  }

  function renderActionPanel(rows: ReturnType<typeof getOptionActionRows> | ReturnType<typeof getConfirmActionRows> | ReturnType<typeof getExtensionItemActionRows>, emptyMessage = 'No actions found') {
    return <ActionPanelView rows={rows} renderEmpty={() => renderChildEmpty(emptyMessage)} />
  }

  function renderMarkdown(content: string) {
    return (
      <div className="markdownContent">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
            a: ({ children, href }) => <a href={href} target="_blank" rel="noreferrer">{children}</a>,
          }}
        >
          {content}
        </ReactMarkdown>
      </div>
    )
  }

  function runDefaultViewAction(item: ExtensionViewItem) {
    const action = item.primaryAction || actionsFromPanel(item.actionPanel, item.actions || [])[0]
    if (action) runViewAction(action)
  }

  function renderTileActionHint(actions: ExtensionViewAction[] = []) {
    if (actions.length === 0) return null
    const shortcut = actions.find((action) => action.shortcut)?.shortcut
    return <span className="tileActionHint">{shortcut ? shortcutLabel(shortcut) : '⌘K'}</span>
  }

  function dragPathForItem(item: ExtensionViewItem) {
    if (item.path || item.filePath) return item.path || item.filePath
    const actions = [item.primaryAction, ...actionsFromPanel(item.actionPanel, item.actions || [])].filter(Boolean) as ExtensionViewAction[]
    return actions.find((action) => action.path)?.path || null
  }

  function startItemDrag(event: React.DragEvent, item: ExtensionViewItem) {
    const filePath = dragPathForItem(item)
    if (!filePath) return
    event.preventDefault()
    window.nvm.startFileDrag(filePath)
  }

  function renderSearchAccessory(view: ExtensionView | null) {
    if (!view?.searchAccessory?.items?.length) return null
    return <select
      className="searchAccessory"
      aria-label={view.searchAccessory.tooltip || 'View filter'}
      value={view.searchAccessory.value || view.searchAccessory.items[0]?.value || ''}
      onChange={(event) => {
        const action = view.searchAccessory?.onChange
        if (!action) return
        runViewAction({ ...action, text: event.target.value })
      }}
    >
      {view.searchAccessory.items.map((item) => <option key={item.value} value={item.value}>{item.title}</option>)}
    </select>
  }

  function renderPagination(view: ExtensionView) {
    if (!view.pagination?.hasMore || !view.pagination.onLoadMore) return null
    return <button className="loadMoreButton" type="button" onClick={() => runViewAction(view.pagination!.onLoadMore!)}>Load More</button>
  }

  function gridStyle(view: ExtensionView) {
    return {
      ...(view.columns ? { '--grid-columns': String(view.columns) } : {}),
      ...(view.aspectRatio ? { '--tile-aspect-ratio': String(view.aspectRatio) } : {}),
    } as React.CSSProperties
  }

  function renderExtensionView(view: ExtensionView) {
    if (view.type === 'grid') {
      const items = filterExtensionItems(view.items || [])
      return <GridView
        items={items}
        sections={filterViewSections(view)}
        subtitle={view.subtitle}
        layout={view.layout || 'square'}
        style={gridStyle(view)}
        empty={renderViewEmpty(view)}
        isLoading={view.isLoading}
        pagination={renderPagination(view)}
        renderItem={(item) => <CommandTile
          key={item.id}
          value={item.id}
          title={item.title}
          subtitle={item.subtitle}
          image={item.image}
          video={item.video || item.videoUrl}
          actionHint={renderTileActionHint(actionsFromPanel(item.actionPanel, item.actions || []))}
          draggable={Boolean(dragPathForItem(item))}
          onDragStart={(event) => startItemDrag(event, item)}
          onSelect={() => runDefaultViewAction(item)}
        />}
      />
    }

    if (view.type === 'list') {
      const items = filterExtensionItems(view.items || [])
      return <ListView
        items={items}
        sections={filterViewSections(view)}
        empty={renderViewEmpty(view)}
        isLoading={view.isLoading}
        pagination={renderPagination(view)}
        renderItem={(item) => <CommandRow
          key={item.id}
          value={item.id}
          className="result extensionListItem"
          icon={item.image ? <span className="thumbnailIcon"><img src={item.image} alt="" /></span> : (() => { const Icon = iconFor[item.icon || 'sparkles'] ?? Sparkles; return <Icon size={18} /> })()}
          title={item.title}
          subtitle={item.subtitle || item.text}
          accessories={item.accessories}
          shortcut={actionsFromPanel(item.actionPanel, item.actions || []).find((action) => action.shortcut)?.shortcut}
          selectedOnlyShortcut={view.id === 'clipboard-history'}
          onSelect={() => runDefaultViewAction(item)}
        />}
      />
    }

    if (view.type === 'chat') {
      const messages = (view.aiChat ? aiMessages : view.messages || []).map((message) => ({ ...message, content: renderMarkdown(message.content) }))
      const input = view.aiChat ? (
        <form
          className="chatInputRow"
          onSubmit={(event) => {
            event.preventDefault()
            sendAiPrompt(aiInput)
          }}
        >
          <textarea
            ref={aiInputRef}
            rows={1}
            value={aiInput}
            onChange={(event) => setAiInput(event.target.value)}
            onInput={(event) => resizeAiInput(event.currentTarget)}
            onKeyDown={(event) => {
              if (event.key !== 'Enter') return
              event.stopPropagation()
              if (!event.shiftKey) {
                event.preventDefault()
                sendAiPrompt(aiInput)
              }
            }}
            placeholder={aiBusy ? 'Thinking…' : 'Message AI'}
          />
          {aiBusy ? (
            <button className="chatIconButton chatStopButton" type="button" aria-label="Stop" title="Stop" onClick={() => window.nvm.abortAiChat(extensionView?.chatId)}>
              <Square size={14} fill="currentColor" />
            </button>
          ) : (
            <button className="chatIconButton chatEnterButton" type="submit" aria-label="Enter" title="Enter" disabled={!aiInput.trim()}>
              <CornerDownLeft size={16} />
            </button>
          )}
        </form>
      ) : null
      return <ChatView messages={messages} isBusy={aiBusy} input={input} messagesRef={view.aiChat ? chatMessagesRef : undefined} />
    }

    if (view.type === 'form') return <FormView
      fields={view.fields || []}
      values={formValues}
      onChange={(id, value) => setFormValues((current) => ({ ...current, [id]: value }))}
      onSubmit={view.submitAction ? () => runViewAction({ ...view.submitAction!, formValues }) : undefined}
      submitTitle={view.submitAction?.title}
    />

    if (view.type === 'progress') return <ProgressView steps={view.steps || []} />

    const detailActions = renderActionPanel(actionPanelRows(view.actionPanel, view.actions || [], 'extension-view', false))

    return <DetailView content={view.content || view.subtitle || ''} image={view.image} video={view.video || view.videoUrl} actions={detailActions} />
  }

  function runLocalShortcut(accelerator: string) {
    if (!extensionView || extensionItemOptionsFor || optionsFor || previewFor || confirmRemoveFor || shortcutManagerOpen || shortcutFor) return false
    const normalized = normalizedShortcut(accelerator)
    if (extensionViewBackStack.length > 0 && normalizedShortcut(lastLocalShortcutRef.current || '') === normalized) {
      popExtensionView()
      lastLocalShortcutRef.current = null
      return true
    }
    const selectedItem = selectedExtensionItem
    const itemActions = selectedItem ? actionsFromPanel(selectedItem.actionPanel, selectedItem.actions || []) : []
    const viewActions = actionsFromPanel(extensionView.actionPanel, extensionView.actions || [])
    const actions = selectedItem ? [selectedItem.primaryAction, ...itemActions].filter(Boolean) as ExtensionViewAction[] : viewActions
    const action = actions.find((item) => normalizedShortcut(item.shortcut) === normalized)
    if (!action) return false
    lastLocalShortcutRef.current = accelerator
    runViewAction(action)
    return true
  }

  function moveGridSelection(key: string) {
    if (extensionView?.type !== 'grid' || clipboardMode || confirmRemoveFor || extensionItemOptionsFor || optionsFor || previewFor) return false
    const items = filterExtensionItems(allViewItems(extensionView))
    if (items.length === 0) return false
    const currentIndex = Math.max(0, items.findIndex((item) => item.id === selectedValue))
    const grid = document.querySelector<HTMLElement>('.extensionGrid')
    const columns = grid ? Math.max(1, getComputedStyle(grid).gridTemplateColumns.split(' ').filter(Boolean).length) : 4
    const delta = key === 'ArrowRight' ? 1 : key === 'ArrowLeft' ? -1 : key === 'ArrowDown' ? columns : -columns
    const nextIndex = Math.max(0, Math.min(items.length - 1, currentIndex + delta))
    const next = items[nextIndex]
    if (!next) return false
    setSelectedValue(next.id)
    requestAnimationFrame(() => document.querySelector(`[data-extension-item-id="${CSS.escape(next.id)}"]`)?.scrollIntoView({ block: 'nearest', inline: 'nearest' }))
    return true
  }

  function onCommandKeyDown(event: React.KeyboardEvent) {
    if (shortcutFor) {
      event.preventDefault()
      event.stopPropagation()
      if (event.key === 'Escape') {
        setShortcutFor(null)
        setRecordedShortcut('')
        return
      }
      if (event.key === 'Enter') {
        saveRecordedShortcut()
        return
      }
      if (event.key === 'Backspace' || event.key === 'Delete') {
        setRecordedShortcut('')
        return
      }
      const accelerator = acceleratorFromEvent(event)
      if (accelerator) setRecordedShortcut(accelerator)
      return
    }

    const localAccelerator = acceleratorFromEvent(event)
    if (localAccelerator && runLocalShortcut(localAccelerator)) {
      event.preventDefault()
      return
    }

    if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key) && moveGridSelection(event.key)) {
      event.preventDefault()
      return
    }

    if (event.key === 'Escape') {
      event.preventDefault()
      if (clipboardMode) closeClipboardHistory()
      else if (shortcutOptionsFor) setShortcutOptionsFor(null)
      else if (shortcutManagerOpen) setShortcutManagerOpen(false)
      else if (confirmRemoveFor) setConfirmRemoveFor(null)
      else if (actionSubmenuFor) setActionSubmenuFor(null)
      else if (extensionItemOptionsFor) setExtensionItemOptionsFor(null)
      else if (optionsFor) setOptionsFor(null)
      else if (previewFor) setPreviewFor(null)
      else if (extensionView) popExtensionView()
      else window.nvm.hide()
      return
    }

    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault()
      if (selectedShortcutRecord && shortcutManagerOpen && !shortcutOptionsFor) {
        setShortcutOptionsFor(selectedShortcutRecord)
        return
      }
      if (selectedExtensionItem && extensionView && !confirmRemoveFor && !extensionItemOptionsFor && !optionsFor && !previewFor) {
        setExtensionItemOptionsFor(selectedExtensionItem)
        return
      }
      if (activeAction && !shortcutManagerOpen && !confirmRemoveFor && !extensionItemOptionsFor && !optionsFor && !extensionView) {
        setPreviewFor(null)
        setOptionsFor(activeAction)
      }
      return
    }

    if (event.key === 'ArrowLeft' && previewFor) {
      event.preventDefault()
      setPreviewFor(null)
      return
    }

    if (event.key === 'ArrowRight' && !isChildOpen && selectedAction) {
      const input = event.target instanceof HTMLInputElement ? event.target : null
      if (input && input.selectionStart !== input.value.length) return
      event.preventDefault()
      setPreviewFor(selectedAction)
      setOptionsFor(null)
      return
    }

    if (!isChildOpen && event.key === 'Tab' && selectedAction?.kind === 'extension-command' && selectedAction.aiChatId) {
      event.preventDefault()
      tweakActionWithAi(selectedAction)
      return
    }

    if (!isChildOpen && event.key === 'Tab' && query) {
      event.preventDefault()
      if (createAction) {
        run(createAction)
      } else {
        run({
          id: `ai:${query}`,
          kind: 'ai-placeholder',
          title: `Automate "${query}"`,
          subtitle: 'Automate with AI',
          query,
          icon: 'bolt',
          score: 90,
        })
      }
    }
  }

  return (
    <main className="shell">
      {toast ? <Toast message={toast.message} tone={toast.tone} /> : null}
      <Command
        className={`palette ${isVisuallyStacked ? 'isStacked' : ''}`}
        label="Nevermind"
        loop
        shouldFilter={false}
        value={selectedValue}
        onValueChange={setSelectedValue}
        onKeyDown={onCommandKeyDown}
      >
        <div className={`searchRow card searchCard ${isVisuallyStacked ? 'stackParentCard' : ''}`}>
          <Zap className="brandIcon" size={22} />
          <Command.Input
            ref={inputRef}
            value={inputValue}
            onValueChange={(value) => {
              if (shortcutFor) return
              if (clipboardMode) setClipboardQuery(value)
              else if (isFilterableChildOpen) setChildQuery(value)
              else if (!isChildOpen) setQuery(value)
            }}
            placeholder={placeholder}
            readOnly={!shortcutFor && !clipboardMode && !isFilterableChildOpen && isChildOpen}
            spellCheck={false}
          />
          {renderSearchAccessory(extensionView)}
          {!isChildOpen && query ? <div className="tabHint">✨ <kbd>Tab</kbd> to automate</div> : null}
        </div>

        <Command.List className={`results card ${isVisuallyStacked ? 'optionsCard' : 'resultsCard'}`}>
          {shortcutFor ? (
            <div className="shortcutRecorder">
              <div className="shortcutKeys">
                {(recordedShortcut ? recordedShortcut.split('+') : ['Press keys']).map((part) => <kbd key={part}>{part}</kbd>)}
              </div>
              {renderActionPanel(getShortcutRecorderRows())}
            </div>
          ) : shortcutOptionsFor ? (
            renderActionPanel(getShortcutOptionRows())
          ) : shortcutManagerOpen ? (
            renderActionPanel(getShortcutRows(), 'No keyboard shortcuts found')
          ) : confirmRemoveFor ? (
            renderActionPanel(getConfirmActionRows())
          ) : actionSubmenuFor ? (
            renderActionPanel(actionPanelRows(actionSubmenuFor.panel, [], 'action-submenu', true))
          ) : extensionItemOptionsFor ? (
            renderActionPanel(getExtensionItemActionRows())
          ) : extensionView ? (
            renderExtensionView(extensionView)
          ) : previewFor ? (
            <div className="previewPane">
              {previewFor.imageDataUrl ? (
                <img className="previewImage" src={previewFor.imageDataUrl} alt="Clipboard preview" />
              ) : previewFor.text ? (
                <pre className="previewText">{previewFor.text}</pre>
              ) : (
                <div className="previewDetails">
                  <strong>{previewFor.title}</strong>
                  <span>{previewFor.subtitle}</span>
                </div>
              )}
            </div>
          ) : optionsFor ? (
            renderActionPanel(getOptionActionRows())
          ) : (
            renderActionResults()
          )}

          {!isChildOpen && actions.length === 0 ? (
            <Command.Empty asChild>
              <EmptyState
                icon={clipboardMode ? <Clipboard size={24} /> : <Zap size={24} />}
                title={clipboardMode ? 'No clipboard items found.' : 'Type anything.'}
                subtitle={clipboardMode ? 'Try a different clipboard search.' : 'Nevermind starts with local actions; AI planning comes next.'}
              />
            </Command.Empty>
          ) : null}
        </Command.List>

        {clipboardMode && isChildOpen && !previewFor && !optionsFor ? (
          <Command.List className="results card optionsCard clipboardSiblingCard">
            {actions.length > 0 ? renderActionResults() : renderChildEmpty('No clipboard items found')}
          </Command.List>
        ) : null}
      </Command>
    </main>
  )
}

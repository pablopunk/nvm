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

type ExtensionViewAction = {
  type: 'openPath' | 'revealPath' | 'quickLook' | 'openUrl' | 'copyText' | 'copyImage' | 'pushView' | 'replaceView' | 'popView' | 'runExtensionAction'
  title: string
  path?: string
  url?: string
  text?: string
  imageDataUrl?: string
  view?: ExtensionView
  handlerId?: string
  shortcut?: string
}

type ExtensionViewItem = {
  id: string
  title: string
  subtitle?: string
  image?: string
  video?: string
  videoUrl?: string
  text?: string
  path?: string
  filePath?: string
  fileUrl?: string
  primaryAction?: ExtensionViewAction
  actions?: ExtensionViewAction[]
}

type ExtensionView = {
  type: 'list' | 'grid' | 'detail' | 'chat' | 'form' | 'progress'
  title: string
  image?: string
  video?: string
  videoUrl?: string
  aiChat?: boolean
  chatId?: string
  initialPrompt?: string
  subtitle?: string
  content?: string
  items?: ExtensionViewItem[]
  messages?: { role: 'user' | 'assistant' | 'system'; content: string }[]
  fields?: { id: string; label: string; type?: string; value?: string }[]
  steps?: { title: string; status?: string }[]
  actions?: ExtensionViewAction[]
  layout?: 'square' | 'wide' | 'compact'
  aspectRatio?: string | number
  columns?: number
}

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
    else if (extensionItemOptionsFor) setSelectedValue(getExtensionItemActionRows()[0]?.value ?? '')
    else if (optionsFor) setSelectedValue(getOptionActionRows()[0]?.value ?? '')
    else if (previewFor) setSelectedValue('preview')
    else if (extensionView && isFilterableExtensionView) setSelectedValue(filterExtensionItems(extensionView.items)[0]?.id ?? '')
    else if (extensionView?.actions?.length) setSelectedValue(`extension-view:0:${extensionView.actions[0].type}:${extensionView.actions[0].title}`)
    else if (extensionView) setSelectedValue('preview')
    else setSelectedValue(actions[0]?.id ?? '')
  }, [actions, childQuery, clipboardMode, confirmRemoveFor, extensionItemOptionsFor, optionsFor, previewFor, extensionView, shortcutFor, shortcutManagerOpen, shortcutRecords, shortcutOptionsFor])

  useEffect(() => {
    setChildQuery('')
  }, [confirmRemoveFor?.id, extensionItemOptionsFor?.id, extensionView?.title, extensionView?.type, optionsFor?.id, previewFor?.id])

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
  const isFilterableChildOpen = Boolean(confirmRemoveFor || extensionItemOptionsFor || optionsFor || shortcutManagerOpen || isFilterableExtensionView)
  const isChildOpen = Boolean(shortcutFor || shortcutOptionsFor || shortcutManagerOpen || confirmRemoveFor || extensionItemOptionsFor || optionsFor || previewFor || extensionView)
  const childPlaceholder = shortcutOptionsFor ? `Actions for “${shortcutOptionsFor.action.title}”` : shortcutManagerOpen ? 'Filter keyboard shortcuts' : confirmRemoveFor ? 'Filter confirmation actions' : extensionItemOptionsFor ? `Filter actions for “${extensionItemOptionsFor.title}”` : optionsFor ? `Filter actions for “${optionsFor.title}”` : extensionView ? `Filter ${extensionView.title}` : ''
  const inputValue = shortcutFor ? recordedShortcut : clipboardMode ? clipboardQuery : isFilterableChildOpen ? childQuery : extensionView ? extensionView.title : previewFor ? previewFor.title : optionsFor && !query ? optionsFor.title : query
  const placeholder = shortcutFor ? 'Press a keyboard shortcut' : clipboardMode ? 'Search Clipboard History' : isFilterableChildOpen ? childPlaceholder : SEARCH_PLACEHOLDERS[placeholderIndex]

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
    if (action.kind === 'clipboard-history') {
      openClipboardHistory()
      return
    }
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

  function keyNameForShortcut(event: React.KeyboardEvent) {
    const key = event.key
    if (!key || ['Meta', 'Control', 'Alt', 'Shift'].includes(key)) return ''
    if (key === ' ') return 'Space'
    if (key === 'ArrowUp') return 'Up'
    if (key === 'ArrowDown') return 'Down'
    if (key === 'ArrowLeft') return 'Left'
    if (key === 'ArrowRight') return 'Right'
    if (/^Digit\d$/.test(event.code)) return event.code.slice('Digit'.length)
    if (/^Key[A-Z]$/.test(event.code)) return event.code.slice('Key'.length)
    if (key.length === 1) return key.toUpperCase()
    return key[0].toUpperCase() + key.slice(1)
  }

  function acceleratorFromEvent(event: React.KeyboardEvent) {
    const key = keyNameForShortcut(event)
    if (!key) return ''
    const parts = []
    if (event.metaKey) parts.push('Command')
    if (event.ctrlKey) parts.push('Control')
    if (event.altKey) parts.push('Alt')
    if (event.shiftKey) parts.push('Shift')
    parts.push(key)
    if (parts.length < 2 && !key.startsWith('F')) return ''
    return parts.join('+')
  }

  function shortcutLabel(shortcut?: string) {
    return String(shortcut || '')
      .split('+')
      .map((part) => ({ Command: '⌘', Cmd: '⌘', Control: '⌃', Ctrl: '⌃', Alt: '⌥', Option: '⌥', Shift: '⇧', Enter: '↵', Return: '↵', Space: '␣', Escape: 'Esc' }[part] || part))
      .join('')
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
    if (!extensionView?.items) return null
    return extensionView.items.find((item) => item.id === selectedValue) || null
  }, [extensionView, selectedValue])
  const selectedShortcutRecord = useMemo(
    () => shortcutRecords.find((record) => `shortcut:${record.actionId}` === selectedValue) || null,
    [selectedValue, shortcutRecords],
  )
  const activeAction = optionsFor || previewFor || selectedAction

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

  function filterExtensionItems(items: ExtensionViewItem[] = []) {
    return items.filter((item) => childMatches(
      item.title,
      item.subtitle,
      item.text,
      ...(item.actions || []).map((action) => action.title),
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

  function getExtensionItemActionRows() {
    return (extensionItemOptionsFor?.actions || []).map((action, index) => ({
      value: `extension-item:${index}:${action.type}:${action.title}`,
      icon: action.type === 'copyText' || action.type === 'copyImage' ? <Clipboard size={18} /> : action.type === 'revealPath' || action.type === 'openPath' || action.type === 'quickLook' ? <Folder size={18} /> : <Globe size={18} />,
      title: action.title,
      subtitle: action.shortcut ? `${action.type} · ${action.shortcut}` : action.type,
      onSelect: async () => {
        await runViewAction(action)
        setExtensionItemOptionsFor(null)
      },
      className: 'result',
    })).filter((row) => childMatches(row.title, row.subtitle))
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

  function renderChildEmpty(message = 'No actions found') {
    return (
      <div className="empty">
        <Search size={24} />
        <strong>{message}</strong>
        <span>Try a different filter.</span>
      </div>
    )
  }

  function renderActionResults() {
    return actions.map((action) => {
      const Icon = iconFor[action.icon] ?? Sparkles
      const iconUrl = action.thumbnailUrl ?? action.iconUrl ?? iconUrls[action.id]
      return (
        <Command.Item
          key={action.id}
          value={action.id}
          className="result"
          onSelect={() => run(action)}
        >
          <span className={`resultIcon ${action.thumbnailUrl ? 'thumbnailIcon' : ''}`}>
            {iconUrl ? <img src={iconUrl} alt="" /> : <Icon size={18} />}
          </span>
          <span className="resultText">
            <strong>{action.title}</strong>
            <small>{action.isOverridden ? `AI override: ${action.overrideSummary}` : action.subtitle}</small>
          </span>
          <span className="keyHints">
            {action.kind === 'extension-command' && action.aiChatId ? <span className="shortcutHint selectedOnlyEnter">Tab tweak</span> : null}
            {action.shortcut ? <span className="shortcutHint">{shortcutLabel(action.shortcut)}</span> : null}
            <span className="enterHint selectedOnlyEnter">↵</span>
          </span>
        </Command.Item>
      )
    })
  }

  function renderActionRow(row: ReturnType<typeof getOptionActionRows>[number] | ReturnType<typeof getConfirmActionRows>[number] | ReturnType<typeof getExtensionItemActionRows>[number]) {
    return (
      <Command.Item key={row.value} value={row.value} className={row.className || 'result'} onSelect={row.onSelect}>
        <span className="resultIcon">{row.icon}</span>
        <span className="resultText">
          <strong>{row.title}</strong>
          <small>{row.subtitle}</small>
        </span>
        <span className="keyHints">
          {'shortcut' in row && row.shortcut ? <span className="shortcutHint">{shortcutLabel(row.shortcut)}</span> : null}
          <span className="enterHint selectedOnlyEnter">↵</span>
        </span>
      </Command.Item>
    )
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
    const action = item.primaryAction || item.actions?.[0]
    if (action) runViewAction(action)
  }

  function renderViewActions(actions: ExtensionViewAction[] = []) {
    if (actions.length === 0) return null
    const shortcut = actions.find((action) => action.shortcut)?.shortcut
    return <span className="enterHint">{shortcut ? shortcutLabel(shortcut) : '⌘K'}</span>
  }

  function renderTileActionHint(actions: ExtensionViewAction[] = []) {
    if (actions.length === 0) return null
    const shortcut = actions.find((action) => action.shortcut)?.shortcut
    return <span className="tileActionHint">{shortcut ? shortcutLabel(shortcut) : '⌘K'}</span>
  }

  function dragPathForItem(item: ExtensionViewItem) {
    if (item.path || item.filePath) return item.path || item.filePath
    const actions = [item.primaryAction, ...(item.actions || [])].filter(Boolean) as ExtensionViewAction[]
    return actions.find((action) => action.path)?.path || null
  }

  function startItemDrag(event: React.DragEvent, item: ExtensionViewItem) {
    const filePath = dragPathForItem(item)
    if (!filePath) return
    event.preventDefault()
    window.nvm.startFileDrag(filePath)
  }

  function gridStyle(view: ExtensionView) {
    return {
      ...(view.columns ? { '--grid-columns': String(view.columns) } : {}),
      ...(view.aspectRatio ? { '--tile-aspect-ratio': String(view.aspectRatio) } : {}),
    } as React.CSSProperties
  }

  function renderExtensionView(view: ExtensionView) {
    if (view.type === 'grid') {
      const items = filterExtensionItems(view.items)
      const layout = view.layout || 'square'
      return (
        <div className="extensionView">
          {view.subtitle ? <div className="extensionSubtitle">{view.subtitle}</div> : null}
          {items.length > 0 ? (
            <div className={`extensionGrid extensionGrid-${layout}`} style={gridStyle(view)}>
              {items.map((item) => (
                <Command.Item
                  key={item.id}
                  value={item.id}
                  className="extensionTile"
                  data-extension-item-id={item.id}
                  draggable={Boolean(dragPathForItem(item))}
                  onDragStart={(event) => startItemDrag(event, item)}
                  onSelect={() => runDefaultViewAction(item)}
                >
                  <span className="tileMedia">
                    {item.video || item.videoUrl ? (
                      <video src={item.video || item.videoUrl} poster={item.image} draggable={false} muted loop playsInline preload="metadata" onMouseEnter={(event) => event.currentTarget.play().catch(() => {})} onMouseLeave={(event) => event.currentTarget.pause()} />
                    ) : item.image ? <img src={item.image} alt="" draggable={false} loading="lazy" decoding="async" /> : <span className="tileIcon"><Folder size={20} /></span>}
                    {renderTileActionHint(item.actions)}
                  </span>
                  <strong>{item.title}</strong>
                  {item.subtitle ? <small>{item.subtitle}</small> : null}
                </Command.Item>
              ))}
            </div>
          ) : renderChildEmpty('No items found')}
        </div>
      )
    }

    if (view.type === 'list') {
      const items = filterExtensionItems(view.items)
      return (
        <div className="extensionView">
          {items.length > 0 ? items.map((item) => (
            <Command.Item key={item.id} value={item.id} className="result extensionListItem" onSelect={() => runDefaultViewAction(item)}>
              <span className="resultIcon"><Sparkles size={18} /></span>
              <span className="resultText">
                <strong>{item.title}</strong>
                <small>{item.subtitle || item.text}</small>
              </span>
              <span className="viewActions">{renderViewActions(item.actions)}</span>
            </Command.Item>
          )) : renderChildEmpty('No items found')}
        </div>
      )
    }

    if (view.type === 'chat') {
      const messages = view.aiChat ? aiMessages : view.messages || []
      return (
        <div className="extensionView chatView">
          <div className="chatMessages" ref={view.aiChat ? chatMessagesRef : undefined}>
            {messages.map((message, index) => (
              <div key={index} className={`chatBubble ${message.role}`}>{renderMarkdown(message.content)}</div>
            ))}
            {aiBusy ? <div className="chatBubble system">Thinking…</div> : null}
          </div>
          {view.aiChat ? (
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
          ) : null}
        </div>
      )
    }

    if (view.type === 'form') {
      return (
        <div className="extensionView formView">
          {(view.fields || []).map((field) => (
            <label key={field.id}>
              <span>{field.label}</span>
              <input defaultValue={field.value || ''} type={field.type || 'text'} />
            </label>
          ))}
        </div>
      )
    }

    if (view.type === 'progress') {
      return (
        <div className="extensionView progressView">
          {(view.steps || []).map((step, index) => (
            <div key={index} className="progressStep">
              <strong>{step.title}</strong>
              <small>{step.status || 'Pending'}</small>
            </div>
          ))}
        </div>
      )
    }

    return (
      <div className="extensionView">
        {view.video || view.videoUrl ? <video className="detailMedia" src={view.video || view.videoUrl} poster={view.image} controls autoPlay muted loop playsInline /> : null}
        {!view.video && !view.videoUrl && view.image ? <img className="detailMedia" src={view.image} alt="" /> : null}
        <pre className="previewText">{view.content || view.subtitle || ''}</pre>
        {(view.actions || []).map((action, index) => (
          <Command.Item key={`${action.type}:${action.title}`} value={`extension-view:${index}:${action.type}:${action.title}`} className="result" onSelect={() => runViewAction(action)}>
            <span className="resultIcon"><Wand2 size={18} /></span>
            <span className="resultText">
              <strong>{action.title}</strong>
              <small>{action.type}</small>
            </span>
            <span className="enterHint">↵</span>
          </Command.Item>
        ))}
      </div>
    )
  }

  function normalizedShortcut(value?: string) {
    return String(value || '').replace(/\s+/g, '').toLowerCase()
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
    const actions = selectedItem ? [selectedItem.primaryAction, ...(selectedItem.actions || [])].filter(Boolean) as ExtensionViewAction[] : (extensionView.actions || [])
    const action = actions.find((item) => normalizedShortcut(item.shortcut) === normalized)
    if (!action) return false
    lastLocalShortcutRef.current = accelerator
    runViewAction(action)
    return true
  }

  function moveGridSelection(key: string) {
    if (extensionView?.type !== 'grid' || clipboardMode || confirmRemoveFor || extensionItemOptionsFor || optionsFor || previewFor) return false
    const items = filterExtensionItems(extensionView.items)
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
      {toast ? <div className={`toast ${toast.tone === 'error' ? 'toastError' : ''}`}>{toast.message}</div> : null}
      <Command
        className={`palette ${isChildOpen ? 'isStacked' : ''}`}
        label="Nevermind"
        loop
        shouldFilter={false}
        value={selectedValue}
        onValueChange={setSelectedValue}
        onKeyDown={onCommandKeyDown}
      >
        <div className={`searchRow card searchCard ${isChildOpen ? 'stackParentCard' : ''}`}>
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
          {!isChildOpen && query ? <div className="tabHint">✨ <kbd>Tab</kbd> to automate</div> : null}
        </div>

        <Command.List className={`results card ${isChildOpen ? 'optionsCard' : 'resultsCard'}`}>
          {shortcutFor ? (
            <div className="shortcutRecorder">
              <div className="shortcutKeys">
                {(recordedShortcut ? recordedShortcut.split('+') : ['Press keys']).map((part) => <kbd key={part}>{part}</kbd>)}
              </div>
              {getShortcutRecorderRows().map(renderActionRow)}
            </div>
          ) : shortcutOptionsFor ? (
            getShortcutOptionRows().length > 0 ? getShortcutOptionRows().map(renderActionRow) : renderChildEmpty()
          ) : shortcutManagerOpen ? (
            getShortcutRows().length > 0 ? getShortcutRows().map(renderActionRow) : renderChildEmpty('No keyboard shortcuts found')
          ) : confirmRemoveFor ? (
            getConfirmActionRows().length > 0 ? getConfirmActionRows().map(renderActionRow) : renderChildEmpty()
          ) : extensionItemOptionsFor ? (
            getExtensionItemActionRows().length > 0 ? getExtensionItemActionRows().map(renderActionRow) : renderChildEmpty()
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
            getOptionActionRows().length > 0 ? getOptionActionRows().map(renderActionRow) : renderChildEmpty()
          ) : (
            renderActionResults()
          )}

          {!isChildOpen && actions.length === 0 ? (
            <Command.Empty className="empty">
              {clipboardMode ? <Clipboard size={24} /> : <Zap size={24} />}
              <strong>{clipboardMode ? 'No clipboard items found.' : 'Type anything.'}</strong>
              <span>{clipboardMode ? 'Try a different clipboard search.' : 'Nevermind starts with local actions; AI planning comes next.'}</span>
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

import { useEffect, useMemo, useRef, useState } from 'react'
import { Command } from 'cmdk'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  AppWindow,
  Calculator,
  Clipboard,
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
  Tag,
  Wand2,
  Trash2,
  Zap,
} from 'lucide-react'

type ActionKind =
  | 'open-url'
  | 'web-search'
  | 'app'
  | 'clipboard'
  | 'clipboard-history'
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
}

type ExtensionViewAction = {
  type: 'openPath' | 'revealPath' | 'openUrl' | 'copyText' | 'copyImage' | 'pushView' | 'replaceView' | 'popView' | 'runExtensionAction'
  title: string
  path?: string
  url?: string
  text?: string
  imageDataUrl?: string
  view?: ExtensionView
  handlerId?: string
}

type ExtensionViewItem = {
  id: string
  title: string
  subtitle?: string
  image?: string
  text?: string
  primaryAction?: ExtensionViewAction
  actions?: ExtensionViewAction[]
}

type ExtensionView = {
  type: 'list' | 'grid' | 'detail' | 'chat' | 'form' | 'progress'
  title: string
  aiChat?: boolean
  chatId?: string
  initialPrompt?: string
  subtitle?: string
  content?: string
  items?: ExtensionViewItem[]
  messages?: { role: 'user' | 'assistant' | 'system'; content: string }[]
  fields?: { id: string; label: string; type?: string; value?: string }[]
  steps?: { title: string; status?: string }[]
}

type SaveResult = {
  ok: boolean
  message: string
}

declare global {
  interface Window {
    nvm: {
      search: (query: string, options?: { clipboardOnly?: boolean }) => Promise<Action[]>
      execute: (action: Action) => Promise<{ view?: ExtensionView } | void>
      runViewAction: (action: ExtensionViewAction) => Promise<{ view?: ExtensionView; navigation?: 'push' | 'replace' | 'pop'; toast?: { message: string; tone?: 'default' | 'error' } } | void>
      sendAiMessage: (message: string, chatId?: string) => Promise<void>
      abortAiChat: (chatId?: string) => Promise<void>
      resetAiChat: (chatId?: string) => Promise<void>
      setAlias: (action: Action, alias: string) => Promise<SaveResult>
      setShortcut: (action: Action, shortcut: string) => Promise<SaveResult>
      setOverride: (action: Action, instruction: string) => Promise<SaveResult>
      clearOverride: (action: Action) => Promise<SaveResult>
      removeCreatedAction: (action: Action) => Promise<SaveResult>
      getAppIcon: (appPath: string) => Promise<string | null>
      setPaletteMode: (mode: 'default' | 'ai-chat') => Promise<void>
      hide: () => Promise<void>
      onShown: (callback: () => void) => () => void
      onHidden: (callback: () => void) => () => void
      onAppsIndexed: (callback: (count: number) => void) => () => void
      onClipboardChanged: (callback: () => void) => () => void
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
}

export function App() {
  const inputRef = useRef<HTMLInputElement>(null)
  const chatMessagesRef = useRef<HTMLDivElement>(null)
  const requestedIcons = useRef(new Set<string>())
  const aiChatOpenRef = useRef(false)
  const aiChatIdRef = useRef<string | undefined>(undefined)
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
  const [childQuery, setChildQuery] = useState('')

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
      if (!aiChatOpenRef.current) {
        setExtensionView(null)
        setExtensionViewBackStack([])
        setAiMessages([])
      }
      setClipboardMode(false)
      setRefreshNonce((nonce) => nonce + 1)
      setPlaceholderIndex((index) => (index + 1) % SEARCH_PLACEHOLDERS.length)
      if (!aiChatOpenRef.current) setExtensionViewBackStack([])
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
      if (!aiChatOpenRef.current) {
        setExtensionView(null)
        setExtensionViewBackStack([])
        setAiMessages([])
      }
      setExtensionViewBackStack([])
      setClipboardMode(false)
    })
    const stopApps = window.nvm.onAppsIndexed(() => {})
    const stopClipboard = window.nvm.onClipboardChanged(() => setRefreshNonce((nonce) => nonce + 1))
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
      stopHidden()
      stopApps()
      stopClipboard()
      stopAi()
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const timer = window.setTimeout(async () => {
      const next = await window.nvm.search(query, { clipboardOnly: clipboardMode })
      if (!cancelled) setActions(next)
    }, 20)

    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [query, refreshNonce, clipboardMode])

  useEffect(() => {
    if (confirmRemoveFor) setSelectedValue(getConfirmActionRows()[0]?.value ?? '')
    else if (extensionItemOptionsFor) setSelectedValue(getExtensionItemActionRows()[0]?.value ?? '')
    else if (optionsFor) setSelectedValue(getOptionActionRows()[0]?.value ?? '')
    else if (previewFor) setSelectedValue('preview')
    else if (extensionView && isFilterableExtensionView) setSelectedValue(filterExtensionItems(extensionView.items)[0]?.id ?? '')
    else if (extensionView) setSelectedValue('preview')
    else setSelectedValue(actions[0]?.id ?? '')
  }, [actions, childQuery, confirmRemoveFor, extensionItemOptionsFor, optionsFor, previewFor, extensionView])

  useEffect(() => {
    setChildQuery('')
  }, [confirmRemoveFor?.id, extensionItemOptionsFor?.id, extensionView?.title, extensionView?.type, optionsFor?.id, previewFor?.id])

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
  const isFilterableChildOpen = Boolean(confirmRemoveFor || extensionItemOptionsFor || optionsFor || isFilterableExtensionView)
  const isChildOpen = Boolean(confirmRemoveFor || extensionItemOptionsFor || optionsFor || previewFor || extensionView)
  const childPlaceholder = confirmRemoveFor ? 'Filter confirmation actions' : extensionItemOptionsFor ? `Filter actions for “${extensionItemOptionsFor.title}”` : optionsFor ? `Filter actions for “${optionsFor.title}”` : extensionView ? `Filter ${extensionView.title}` : ''
  const inputValue = isFilterableChildOpen ? childQuery : extensionView ? extensionView.title : previewFor ? previewFor.title : clipboardMode ? 'Clipboard History' : optionsFor && !query ? optionsFor.title : query
  const placeholder = isFilterableChildOpen ? childPlaceholder : SEARCH_PLACEHOLDERS[placeholderIndex]

  useEffect(() => {
    if (!isFilterableChildOpen) return
    requestAnimationFrame(() => inputRef.current?.focus())
  }, [confirmRemoveFor?.id, extensionItemOptionsFor?.id, extensionView?.title, extensionView?.type, isFilterableChildOpen, optionsFor?.id])

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

  function handleViewActionResult(result?: { view?: ExtensionView; navigation?: 'push' | 'replace' | 'pop'; toast?: { message: string; tone?: 'default' | 'error' } } | void) {
    if (!result) return
    if (result.toast) showToast(result.toast.message, result.toast.tone || 'default')
    if (result.navigation === 'pop') popExtensionView()
    else if (result.view) showExtensionView(result.view, result.navigation || 'push')
  }

  async function runViewAction(action: ExtensionViewAction) {
    const result = await window.nvm.runViewAction(action)
    handleViewActionResult(result)
  }

  async function openAiChat(view: ExtensionView) {
    showExtensionView(view, 'root')
    setAiMessages(view.messages || [])
    if (view.initialPrompt) {
      await window.nvm.resetAiChat(view.chatId)
      await sendAiPrompt(view.initialPrompt, view.chatId)
    }
  }

  async function run(action: Action) {
    if (action.kind === 'clipboard-history') {
      setClipboardMode(true)
      setOptionsFor(null)
      setPreviewFor(null)
      setExtensionView(null)
      setExtensionViewBackStack([])
      setAiMessages([])
      setQuery('')
      setRefreshNonce((nonce) => nonce + 1)
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
    const shortcut = window.prompt(`Keyboard shortcut for “${optionsFor.title}”`, 'Command+Shift+')
    if (!shortcut?.trim()) return
    const result = await window.nvm.setShortcut(optionsFor, shortcut)
    showToast(result.message, result.ok ? 'default' : 'error')
    if (result.ok) setOptionsFor(null)
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

  async function tweakWithAi() {
    if (!optionsFor?.aiChatId) return
    const result = await window.nvm.execute({
      id: `ai-chat:${optionsFor.aiChatId}`,
      kind: 'ai-chat',
      title: optionsFor.title,
      subtitle: 'Tweak AI-created action',
      icon: 'sparkles',
      score: 0,
      aiChatId: optionsFor.aiChatId,
    })
    if (result?.view) {
      setOptionsFor(null)
      await openAiChat(result.view)
    }
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
  const selectedExtensionItem = useMemo(() => {
    if (!extensionView?.items) return null
    return extensionView.items.find((item) => item.id === selectedValue) || null
  }, [extensionView, selectedValue])
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
      icon: action.type === 'copyText' || action.type === 'copyImage' ? <Clipboard size={18} /> : action.type === 'revealPath' || action.type === 'openPath' ? <Folder size={18} /> : <Globe size={18} />,
      title: action.title,
      subtitle: action.type,
      onSelect: async () => {
        await runViewAction(action)
        setExtensionItemOptionsFor(null)
      },
      className: 'result',
    })).filter((row) => childMatches(row.title, row.subtitle))
  }

  function getOptionActionRows() {
    return [
      {
        value: 'option:shortcut',
        icon: <Keyboard size={18} />,
        title: 'Set keyboard shortcut',
        subtitle: 'Run this action globally without opening Nevermind',
        onSelect: setShortcut,
        show: true,
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
        value: 'option:alias',
        icon: <Tag size={18} />,
        title: 'Set alias',
        subtitle: 'Make this action appear for another phrase',
        onSelect: setAlias,
        show: true,
      },
      {
        value: 'option:tweak',
        icon: <Wand2 size={18} />,
        title: 'Tweak with AI',
        subtitle: 'Open the original creation chat for this action',
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

  function renderActionRow(row: ReturnType<typeof getOptionActionRows>[number] | ReturnType<typeof getConfirmActionRows>[number] | ReturnType<typeof getExtensionItemActionRows>[number]) {
    return (
      <Command.Item key={row.value} value={row.value} className={row.className || 'result'} onSelect={row.onSelect}>
        <span className="resultIcon">{row.icon}</span>
        <span className="resultText">
          <strong>{row.title}</strong>
          <small>{row.subtitle}</small>
        </span>
        <span className="enterHint">↵</span>
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
    return <span className="enterHint">⌘K</span>
  }

  function renderExtensionView(view: ExtensionView) {
    if (view.type === 'grid') {
      const items = filterExtensionItems(view.items)
      return (
        <div className="extensionView">
          {view.subtitle ? <div className="extensionSubtitle">{view.subtitle}</div> : null}
          {items.length > 0 ? (
            <div className="extensionGrid">
              {items.map((item) => (
                <Command.Item
                  key={item.id}
                  value={item.id}
                  className="extensionTile"
                  onSelect={() => runDefaultViewAction(item)}
                >
                  {item.image ? <img src={item.image} alt="" loading="lazy" decoding="async" /> : <span className="tileIcon"><Folder size={20} /></span>}
                  <strong>{item.title}</strong>
                  {item.subtitle ? <small>{item.subtitle}</small> : null}
                  <div className="viewActions">{renderViewActions(item.actions)}</div>
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
                rows={1}
                value={aiInput}
                onChange={(event) => setAiInput(event.target.value)}
                onInput={(event) => {
                  const textarea = event.currentTarget
                  textarea.style.height = 'auto'
                  textarea.style.height = `${textarea.scrollHeight}px`
                }}
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
                <button className="chatStopButton" type="button" onClick={() => window.nvm.abortAiChat(extensionView?.chatId)}>
                  Stop
                </button>
              ) : (
                <span className="chatEnterHint">↵</span>
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

    return <pre className="previewText">{view.content || view.subtitle || ''}</pre>
  }

  function onCommandKeyDown(event: React.KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault()
      if (confirmRemoveFor) setConfirmRemoveFor(null)
      else if (extensionItemOptionsFor) setExtensionItemOptionsFor(null)
      else if (optionsFor) setOptionsFor(null)
      else if (previewFor) setPreviewFor(null)
      else if (extensionView) popExtensionView()
      else if (clipboardMode) setClipboardMode(false)
      else window.nvm.hide()
      return
    }

    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault()
      if (selectedExtensionItem && extensionView && !confirmRemoveFor && !extensionItemOptionsFor && !optionsFor && !previewFor) {
        setExtensionItemOptionsFor(selectedExtensionItem)
        return
      }
      if (activeAction && !confirmRemoveFor && !extensionItemOptionsFor && !optionsFor && !extensionView && !clipboardMode) {
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
              if (isFilterableChildOpen) setChildQuery(value)
              else if (!isChildOpen && !clipboardMode) setQuery(value)
            }}
            placeholder={placeholder}
            readOnly={(!isFilterableChildOpen && isChildOpen) || clipboardMode}
            spellCheck={false}
          />
          {!isChildOpen && query ? <div className="tabHint">✨ <kbd>Tab</kbd> to automate</div> : null}
        </div>

        <Command.List className={`results card ${isChildOpen ? 'optionsCard' : 'resultsCard'}`}>
          {confirmRemoveFor ? (
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
            actions.map((action) => {
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
                  <span className="enterHint">↵</span>
                </Command.Item>
              )
            })
          )}

          {!isChildOpen && actions.length === 0 ? (
            <Command.Empty className="empty">
              <Zap size={24} />
              <strong>Type anything.</strong>
              <span>Nevermind starts with local actions; AI planning comes next.</span>
            </Command.Empty>
          ) : null}
        </Command.List>
      </Command>
    </main>
  )
}

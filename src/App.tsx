import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Command } from 'cmdk'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  Clipboard,
  Keyboard,
  RotateCcw,
  Search,
  Sparkles,
  Tag,
  Trash2,
  Wand2,
  Zap,
} from 'lucide-react'
import { EmptyState, SearchAccessory, Toast, type ActionPanelRow } from './ui'
import { RootCommandList } from './command-list'
import { acceleratorFromKeyboardEvent, keyNameForShortcut, normalizedShortcut } from './shortcuts'
import { allViewItems, filterCommandItems, filterCommandSections, valuesMatch } from './filtering'
import { iconFor, iconForAction, iconForItem, type CommandIconName } from './command-icons'
import { useExtensionNavigation } from './use-extension-navigation'
import { useAiChat } from './use-ai-chat'
import { useSearchResults } from './use-search-results'
import { ActionPanel } from './action-panel'
import { ExtensionViewRenderer } from './extension-view'
import { ShortcutManagerView, shortcutItems, shortcutOptionRows, shortcutRecorderRows, type ShortcutRecordLike } from './shortcut-manager'
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
      setPaletteMode: (mode: 'default' | 'ai-chat' | 'stacked') => Promise<void>
      hide: () => Promise<void>
      shortcutReady: () => Promise<void>
      onShown: (callback: () => void) => () => void
      onShortcutShown: (callback: () => void) => () => void
      onHidden: (callback: () => void) => () => void
      onAppsIndexed: (callback: (count: number) => void) => () => void
      onClipboardChanged: (callback: () => void) => () => void
      onOpenActionView: (callback: (payload?: { view?: ExtensionView; revealWhenReady?: boolean; asSibling?: boolean }) => void) => () => void
      onAiChatEvent: (callback: (event: { type: string; text?: string; message?: string; name?: string; chatId?: string; label?: string; data?: unknown }) => void) => () => void
    }
  }
}

const SEARCH_PLACEHOLDERS = [
  'Watcha gonna do?',
  'I cannot do that... Nevermind, I can now',
  'Make it happen',
]

export function App() {
  const inputRef = useRef<HTMLInputElement>(null)
  const resultsListRef = useRef<HTMLDivElement>(null)
  const paletteRef = useRef<HTMLDivElement>(null)
  const requestedIcons = useRef(new Set<string>())
  const aiChatOpenRef = useRef(false)
  const aiChatIdRef = useRef<string | undefined>(undefined)
  const runningViewActionsRef = useRef(new Set<string>())
  const lastLocalShortcutRef = useRef<string | null>(null)
  const [query, setQuery] = useState('')
  const [refreshNonce, setRefreshNonce] = useState(0)
  const [actions, setActions] = useSearchResults<Action>(window.nvm.search, query, refreshNonce)
  const [iconUrls, setIconUrls] = useState<Record<string, string | null>>({})
  const [selectedValue, setSelectedValue] = useState('')
  const [optionsFor, setOptionsFor] = useState<Action | null>(null)
  const [extensionItemOptionsFor, setExtensionItemOptionsFor] = useState<ExtensionViewItem | null>(null)
  const [actionSubmenuFor, setActionSubmenuFor] = useState<{ title: string; panel: CommandActionPanel } | null>(null)
  const [confirmRemoveFor, setConfirmRemoveFor] = useState<Action | null>(null)
  const [previewFor, setPreviewFor] = useState<Action | null>(null)
  const extensionNavigation = useExtensionNavigation()
  const extensionView = extensionNavigation.view
  const extensionViewBackStack = extensionNavigation.backStack
  const aiChat = useAiChat(window.nvm.sendAiMessage, window.nvm.resetAiChat)
  const [toast, setToast] = useState<{ message: string; tone?: 'default' | 'error' } | null>(null)
  const [placeholderIndex, setPlaceholderIndex] = useState(SEARCH_PLACEHOLDERS.length - 1)
  const [pendingShortcutReveal, setPendingShortcutReveal] = useState(false)
  const [childQuery, setChildQuery] = useState('')
  const [shortcutFor, setShortcutFor] = useState<Action | null>(null)
  const [recordedShortcut, setRecordedShortcut] = useState('')
  const [shortcutManagerOpen, setShortcutManagerOpen] = useState(false)
  const [shortcutRecords, setShortcutRecords] = useState<ShortcutRecord[]>([])
  const [shortcutOptionsFor, setShortcutOptionsFor] = useState<ShortcutRecord | null>(null)
  const [formValues, setFormValues] = useState<Record<string, string | boolean>>({})
  const [siblingViews, setSiblingViews] = useState<ExtensionView[]>([])
  const extensionViewRef = useRef<ExtensionView | null>(null)
  useEffect(() => { extensionViewRef.current = extensionView }, [extensionView])

  useLayoutEffect(() => {
    const palette = paletteRef.current
    if (!palette) return
    if (siblingViews.length === 0) {
      palette.style.removeProperty('--spine-top')
      palette.style.removeProperty('--spine-height')
      return
    }
    const updateSpine = () => {
      const searchRow = palette.querySelector('.searchRow') as HTMLElement | null
      const activePane = palette.querySelector('.results') as HTMLElement | null
      if (!searchRow || !activePane) return
      const paletteRect = palette.getBoundingClientRect()
      const top = searchRow.getBoundingClientRect().bottom - paletteRect.top
      const bottom = activePane.getBoundingClientRect().top - paletteRect.top - 11
      palette.style.setProperty('--spine-top', `${top}px`)
      palette.style.setProperty('--spine-height', `${Math.max(0, bottom - top)}px`)
    }
    updateSpine()
    const observer = new ResizeObserver(updateSpine)
    observer.observe(palette)
    palette.querySelectorAll('.siblingPane').forEach((node) => observer.observe(node))
    return () => observer.disconnect()
  }, [siblingViews, extensionView])

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
        extensionNavigation.clearView()
        aiChat.setMessages([])
      }
      setRefreshNonce((nonce) => nonce + 1)
      setPlaceholderIndex((index) => (index + 1) % SEARCH_PLACEHOLDERS.length)
      if (!aiChatOpenRef.current) {
        extensionNavigation.setBackStack([])
        setSiblingViews([])
      }
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
        extensionNavigation.clearView()
        aiChat.setMessages([])
      }
      extensionNavigation.setBackStack([])
      setSiblingViews([])
    })
    const stopApps = window.nvm.onAppsIndexed(() => {})
    const stopClipboard = window.nvm.onClipboardChanged(() => setRefreshNonce((nonce) => nonce + 1))
    const stopOpenActionView = window.nvm.onOpenActionView(async (payload) => {
      if (!payload?.view) return
      setOptionsFor(null)
      setPreviewFor(null)
      const current = extensionViewRef.current
      if (payload.asSibling && current && current.id !== payload.view.id) {
        setSiblingViews((siblings) => [...siblings, current])
      } else if (!payload.asSibling) {
        setSiblingViews([])
      }
      if (payload.view.aiChat) await openAiChat(payload.view)
      else showExtensionView(payload.view, 'root')
      markShortcutReady(Boolean(payload?.revealWhenReady))
    })
    const stopAi = window.nvm.onAiChatEvent((event) => {
      if (event.type === 'debug') console.debug('[Nevermind AI]', event.label, event.data)
      if (event.chatId && event.chatId !== aiChatIdRef.current) return
      if (event.type === 'start') aiChat.setBusy(true)
      if (event.type === 'done' || event.type === 'error' || event.type === 'aborted') aiChat.setBusy(false)
      if (event.type === 'delta' && event.text) aiChat.appendDelta(event.text)
      if (event.type === 'tool_start' && event.name) aiChat.appendMessage('system', `Using ${event.name}…`)
      if (event.type === 'error' && event.message) aiChat.appendMessage('system', event.message)
    })
    return () => {
      stopShown()
      stopShortcutShown()
      stopHidden()
      stopApps()
      stopClipboard()
      stopOpenActionView()
      stopAi()
    }
  }, [])

  useEffect(() => {
    if (shortcutFor) setSelectedValue('shortcut:save')
    else if (shortcutOptionsFor) setSelectedValue(getShortcutOptionRows()[0]?.value ?? '')
    else if (shortcutManagerOpen) setSelectedValue(getShortcutRows()[0]?.value ?? '')
    else if (confirmRemoveFor) setSelectedValue(getConfirmActionRows()[0]?.value ?? '')
    else if (actionSubmenuFor) setSelectedValue(actionPanelRows(actionSubmenuFor.panel, [], 'action-submenu', true).find((row) => !row.sectionHeader)?.value ?? '')
    else if (extensionItemOptionsFor) setSelectedValue(getExtensionItemActionRows()[0]?.value ?? '')
    else if (optionsFor) setSelectedValue(getOptionActionRows()[0]?.value ?? '')
    else if (previewFor) setSelectedValue('preview')
    else if (extensionView && isFilterableExtensionView) setSelectedValue(extensionView.selectedItemId || filterExtensionItems(allViewItems(extensionView))[0]?.id || '')
    else if (extensionView?.actions?.length) setSelectedValue(`extension-view:0:${extensionView.actions[0].type}:${extensionView.actions[0].title}`)
    else if (extensionView) setSelectedValue('preview')
    else setSelectedValue(actions[0]?.id ?? '')
  }, [actions, actionSubmenuFor, childQuery, confirmRemoveFor, extensionItemOptionsFor, optionsFor, previewFor, extensionView, shortcutFor, shortcutManagerOpen, shortcutRecords, shortcutOptionsFor])

  useEffect(() => {
    setChildQuery('')
  }, [actionSubmenuFor?.title, confirmRemoveFor?.id, extensionItemOptionsFor?.id, extensionView?.title, extensionView?.type, optionsFor?.id, previewFor?.id])

  useEffect(() => {
    if (extensionView?.type !== 'form') return
    setFormValues(Object.fromEntries((extensionView.fields || []).map((field) => [field.id, field.type === 'checkbox' ? Boolean(field.value) : field.value || ''])))
  }, [extensionView])

  useEffect(() => {
    if (!pendingShortcutReveal || !extensionView) return
    const frame = requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        window.nvm.shortcutReady()
        setPendingShortcutReveal(false)
      })
    })
    return () => cancelAnimationFrame(frame)
  }, [pendingShortcutReveal, extensionView])

  useEffect(() => {
    const isAiChat = Boolean(extensionView?.aiChat)
    aiChatOpenRef.current = isAiChat
    aiChatIdRef.current = extensionView?.aiChat ? extensionView.chatId : undefined
    const mode = siblingViews.length > 0 ? 'stacked' : isAiChat ? 'ai-chat' : 'default'
    window.nvm.setPaletteMode(mode)
  }, [extensionView, siblingViews.length])

  useEffect(() => {
    if (extensionView?.aiChat) {
      aiChat.messagesRef.current?.scrollTo({ top: aiChat.messagesRef.current.scrollHeight })
    }
  }, [aiChat.messages, aiChat.busy, extensionView])

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
  const isVisuallyStacked = (isChildOpen && !isRootLikeExtensionView) || siblingViews.length > 0
  const childPlaceholder = actionSubmenuFor ? `Filter ${actionSubmenuFor.title}` : shortcutOptionsFor ? `Actions for “${shortcutOptionsFor.action.title}”` : shortcutManagerOpen ? 'Filter keyboard shortcuts' : confirmRemoveFor ? 'Filter confirmation actions' : extensionItemOptionsFor ? `Filter actions for “${extensionItemOptionsFor.title}”` : optionsFor ? `Filter actions for “${optionsFor.title}”` : extensionView ? `Filter ${extensionView.title}` : ''
  const inputValue = shortcutFor ? recordedShortcut : isFilterableChildOpen ? childQuery : extensionView ? extensionView.title : previewFor ? previewFor.title : optionsFor && !query ? optionsFor.title : query
  const placeholder = shortcutFor ? 'Press a keyboard shortcut' : isFilterableChildOpen ? (extensionView?.searchBarPlaceholder || childPlaceholder) : SEARCH_PLACEHOLDERS[placeholderIndex]

  useEffect(() => {
    if (!isFilterableChildOpen && !shortcutFor) return
    requestAnimationFrame(() => inputRef.current?.focus())
  }, [confirmRemoveFor?.id, extensionItemOptionsFor?.id, extensionView?.title, extensionView?.type, isFilterableChildOpen, optionsFor?.id, shortcutFor?.id])

  useEffect(() => {
    if (isChildOpen) return
    requestAnimationFrame(() => resultsListRef.current?.scrollTo({ top: 0 }))
  }, [query, actions, isChildOpen])

  function markShortcutReady(shouldReveal: boolean) {
    if (shouldReveal) setPendingShortcutReveal(true)
  }

  function showToast(message: string, tone: 'default' | 'error' = 'default') {
    setToast({ message, tone })
    window.setTimeout(() => setToast((current) => current?.message === message ? null : current), 2200)
  }

  async function sendAiPrompt(message: string, chatId = extensionView?.chatId) {
    await aiChat.sendPrompt(message, chatId)
  }

  function showExtensionView(view: ExtensionView, navigation: 'root' | 'push' | 'replace' = 'replace') {
    extensionNavigation.showView(view, navigation)
  }

  function popExtensionView() {
    setExtensionItemOptionsFor(null)
    lastLocalShortcutRef.current = null
    if (extensionViewBackStack.length === 0 && siblingViews.length > 0) {
      const next = siblingViews[siblingViews.length - 1]
      setSiblingViews((siblings) => siblings.slice(0, -1))
      extensionNavigation.showView(next, 'root')
      return
    }
    extensionNavigation.popView()
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
      if (action.dismissAfterRun === 'auto' && !result?.view && result?.navigation !== 'pop') {
        if (extensionNavigation.backStack.length > 0) popExtensionView()
        else window.nvm.hide()
      }
    } finally {
      runningViewActionsRef.current.delete(actionKey)
    }
  }

  async function openAiChat(view: ExtensionView) {
    showExtensionView(view, 'root')
    await aiChat.openChat(view)
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

  function childMatches(...values: Array<string | undefined>) {
    return valuesMatch(childQuery, ...values)
  }

  function filterViewSections(view: ExtensionView) {
    return filterCommandSections(view, childQuery)
  }

  function filterExtensionItems(items: ExtensionViewItem[] = []) {
    return filterCommandItems(items, childQuery)
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

  function actionPanelRows(panel = extensionItemOptionsFor?.actionPanel, fallbackActions = extensionItemOptionsFor?.actions || [], prefix = 'extension-item', closeAfterSelect = true): ActionPanelRow[] {
    const sections = panel?.sections?.length ? panel.sections : [{ actions: fallbackActions }]
    return sections.flatMap((section, sectionIndex) => [
      ...(section.title ? [{ value: `${prefix}:section:${sectionIndex}`, title: section.title, subtitle: '', sectionHeader: true, onSelect: () => {}, className: 'actionSectionHeader' }] : []),
      ...(section.actions || []).map((action, index) => ({
        value: `${prefix}:${sectionIndex}:${index}:${action.type}:${action.title}`,
        icon: iconForAction(action),
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
    ]).filter((row) => 'sectionHeader' in row || childMatches(row.title, row.subtitle))
  }

  function getExtensionItemActionRows() {
    return actionPanelRows()
  }

  function getShortcutRows() {
    return shortcutItems(shortcutRecords, childMatches).map((item) => ({
      value: item.id,
      icon: <Keyboard size={18} />,
      title: item.title,
      subtitle: item.subtitle,
      onSelect: () => {
        const record = shortcutRecords.find((candidate) => `shortcut:${candidate.actionId}` === item.id)
        if (record) startShortcutRecorder(record.action)
      },
      className: 'result',
    }))
  }

  function renderShortcutManager() {
    return <ShortcutManagerView records={shortcutRecords} matches={childMatches} onSelect={(record) => setShortcutOptionsFor(record as ShortcutRecord)} />
  }

  function getShortcutOptionRows() {
    return shortcutOptionRows(
      shortcutOptionsFor as ShortcutRecordLike | null,
      (action) => { startShortcutRecorder(action as Action); setShortcutOptionsFor(null) },
      (record) => removeShortcut(record as ShortcutRecord),
      childMatches,
    )
  }

  function getShortcutRecorderRows() {
    return shortcutRecorderRows(recordedShortcut, shortcutFor, saveRecordedShortcut, () => setShortcutFor(null))
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
      icon: action.icon,
      image: action.thumbnailUrl || action.iconUrl || iconUrls[action.id] || undefined,
      primaryAction: { type: 'nativeAction', title: action.title, shortcut: action.shortcut, nativeAction: action },
      actionPanel: actionPanelFromActions([{ type: 'nativeAction', title: action.title, shortcut: action.shortcut, nativeAction: action }]),
    }
  }

  function primaryCommandAction(item: CommandItem) {
    return item.primaryAction || actionsFromPanel(item.actionPanel, item.actions || [])[0]
  }

  function actionFromCommandItem(item: CommandItem) {
    return primaryCommandAction(item)?.nativeAction as Action | undefined
  }

  async function runCommandItem(item: CommandItem) {
    const action = primaryCommandAction(item)
    if (!action) return
    if (action.type === 'nativeAction' && actionFromCommandItem(item)?.kind === 'keyboard-shortcuts') {
      await openShortcutManager()
      return
    }
    await runViewAction(action)
  }

  function iconForCommandItem(item: CommandItem) {
    const action = actionFromCommandItem(item)
    return iconForItem({ ...item, icon: (action?.icon || item.icon) as CommandIconName, image: item.image })
  }

  function renderActionResults() {
    const items = actions.map(commandItemFromAction)
    return <RootCommandList
      items={items}
      iconForItem={iconForCommandItem}
      onSelect={runCommandItem}
      extraForItem={(item) => actionFromCommandItem(item)?.kind === 'extension-command' && actionFromCommandItem(item)?.aiChatId ? ['Tab tweak'] : []}
    />
  }

  function renderActionPanel(rows: ActionPanelRow[] | unknown[], emptyMessage = 'No actions found') {
    return <ActionPanel rows={rows as ActionPanelRow[]} emptyMessage={emptyMessage} />
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
    return <SearchAccessory
      tooltip={view.searchAccessory.tooltip}
      value={view.searchAccessory.value}
      items={view.searchAccessory.items}
      onChange={(value) => {
        const action = view.searchAccessory?.onChange
        if (action) runViewAction({ ...action, text: value })
      }}
    />
  }

  function renderExtensionView(view: ExtensionView) {
    return <ExtensionViewRenderer
      view={view}
      aiChat={aiChat}
      formValues={formValues}
      setFormValues={setFormValues}
      filterItems={filterExtensionItems}
      filterSections={filterViewSections}
      renderMarkdown={renderMarkdown}
      renderActionPanel={renderActionPanel}
      actionPanelRows={actionPanelRows}
      renderRootIcon={iconForCommandItem}
      renderEmpty={renderViewEmpty}
      runDefaultAction={runDefaultViewAction}
      runAction={runViewAction}
      sendAiPrompt={sendAiPrompt}
      abortAiChat={window.nvm.abortAiChat}
      dragPathForItem={dragPathForItem}
      startItemDrag={startItemDrag}
    />
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
    if (extensionView?.type !== 'grid' || confirmRemoveFor || extensionItemOptionsFor || optionsFor || previewFor) return false
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
      if (shortcutOptionsFor) setShortcutOptionsFor(null)
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
        ref={paletteRef}
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
              if (isFilterableChildOpen) setChildQuery(value)
              else if (!isChildOpen) {
                resultsListRef.current?.scrollTo({ top: 0 })
                setQuery(value)
              }
            }}
            placeholder={placeholder}
            readOnly={!shortcutFor && !isFilterableChildOpen && isChildOpen}
            spellCheck={false}
          />
          {renderSearchAccessory(extensionView)}
          {!isChildOpen && query ? <div className="tabHint">✨ <kbd>Tab</kbd> to automate</div> : null}
        </div>

        {siblingViews.map((sib, index) => (
          <div key={`sibling-${index}-${sib.id || sib.title}`} className="siblingPane card inertSibling" aria-hidden="true">
            <div className="siblingHeader">{sib.title}</div>
            <div className="siblingBody">{renderExtensionView(sib)}</div>
          </div>
        ))}

        <Command.List ref={resultsListRef} className={`results card ${isVisuallyStacked ? 'optionsCard' : 'resultsCard'}`}>
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
            renderShortcutManager()
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
                icon={<Zap size={24} />}
                title="Type anything."
                subtitle="Nevermind starts with local actions; AI planning comes next."
              />
            </Command.Empty>
          ) : null}
        </Command.List>

      </Command>
    </main>
  )
}

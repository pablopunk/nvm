import { useEffect, useMemo, useRef, useState } from 'react'
import { Command } from 'cmdk'
import {
  AppWindow,
  Calculator,
  Clipboard,
  Folder,
  Globe,
  Keyboard,
  Lock,
  Moon,
  Power,
  RotateCcw,
  Search,
  Settings,
  Sparkles,
  Tag,
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
  | 'builtin'
  | 'calculate'

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
}

type SaveResult = {
  ok: boolean
  message: string
}

declare global {
  interface Window {
    mvm: {
      search: (query: string, options?: { clipboardOnly?: boolean }) => Promise<Action[]>
      execute: (action: Action) => Promise<void>
      setAlias: (action: Action, alias: string) => Promise<SaveResult>
      setShortcut: (action: Action, shortcut: string) => Promise<SaveResult>
      setOverride: (action: Action, instruction: string) => Promise<SaveResult>
      clearOverride: (action: Action) => Promise<SaveResult>
      getAppIcon: (appPath: string) => Promise<string | null>
      hide: () => Promise<void>
      onShown: (callback: () => void) => () => void
      onHidden: (callback: () => void) => () => void
      onAppsIndexed: (callback: (count: number) => void) => () => void
      onClipboardChanged: (callback: () => void) => () => void
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
}

export function App() {
  const inputRef = useRef<HTMLInputElement>(null)
  const requestedIcons = useRef(new Set<string>())
  const [query, setQuery] = useState('')
  const [actions, setActions] = useState<Action[]>([])
  const [iconUrls, setIconUrls] = useState<Record<string, string | null>>({})
  const [selectedValue, setSelectedValue] = useState('')
  const [optionsFor, setOptionsFor] = useState<Action | null>(null)
  const [previewFor, setPreviewFor] = useState<Action | null>(null)
  const [placeholderIndex, setPlaceholderIndex] = useState(SEARCH_PLACEHOLDERS.length - 1)
  const [refreshNonce, setRefreshNonce] = useState(0)
  const [clipboardMode, setClipboardMode] = useState(false)

  useEffect(() => {
    const focusInput = () => {
      inputRef.current?.focus()
      inputRef.current?.select()
    }

    const stopShown = window.mvm.onShown(() => {
      setOptionsFor(null)
      setPreviewFor(null)
      setClipboardMode(false)
      setRefreshNonce((nonce) => nonce + 1)
      setPlaceholderIndex((index) => (index + 1) % SEARCH_PLACEHOLDERS.length)
      requestAnimationFrame(focusInput)
      window.setTimeout(focusInput, 50)
    })
    const stopHidden = window.mvm.onHidden(() => {
      setQuery('')
      setOptionsFor(null)
      setPreviewFor(null)
      setClipboardMode(false)
    })
    const stopApps = window.mvm.onAppsIndexed(() => {})
    const stopClipboard = window.mvm.onClipboardChanged(() => setRefreshNonce((nonce) => nonce + 1))
    return () => {
      stopShown()
      stopHidden()
      stopApps()
      stopClipboard()
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const timer = window.setTimeout(async () => {
      const next = await window.mvm.search(query, { clipboardOnly: clipboardMode })
      if (!cancelled) setActions(next)
    }, 20)

    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [query, refreshNonce, clipboardMode])

  useEffect(() => {
    if (optionsFor) setSelectedValue('option:shortcut')
    else if (previewFor) setSelectedValue('preview')
    else setSelectedValue(actions[0]?.id ?? '')
  }, [actions, optionsFor, previewFor])

  useEffect(() => {
    for (const action of actions) {
      if (action.kind !== 'app' || !action.app?.path || requestedIcons.current.has(action.id)) continue

      requestedIcons.current.add(action.id)
      window.mvm.getAppIcon(action.app.path).then((iconUrl) => {
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
  const inputValue = previewFor ? previewFor.title : clipboardMode ? 'Clipboard History' : optionsFor && !query ? optionsFor.title : query
  const placeholder = SEARCH_PLACEHOLDERS[placeholderIndex]

  async function run(action: Action) {
    if (action.kind === 'clipboard-history') {
      setClipboardMode(true)
      setOptionsFor(null)
      setPreviewFor(null)
      setQuery('')
      setRefreshNonce((nonce) => nonce + 1)
      return
    }
    await window.mvm.execute(action)
  }

  async function setAlias() {
    if (!optionsFor) return
    const alias = window.prompt(`Alias for “${optionsFor.title}”`)
    if (!alias?.trim()) return
    const result = await window.mvm.setAlias(optionsFor, alias)
    if (!result.ok) window.alert(result.message)
    setOptionsFor(null)
  }

  async function setShortcut() {
    if (!optionsFor) return
    const shortcut = window.prompt(`Keyboard shortcut for “${optionsFor.title}”`, 'Command+Shift+')
    if (!shortcut?.trim()) return
    const result = await window.mvm.setShortcut(optionsFor, shortcut)
    window.alert(result.message)
    if (result.ok) setOptionsFor(null)
  }

  async function setOverride() {
    if (!optionsFor) return
    const instruction = window.prompt(
      `How should AI override “${optionsFor.title}”?`,
      optionsFor.overrideSummary || '',
    )
    if (!instruction?.trim()) return
    const result = await window.mvm.setOverride(optionsFor, instruction)
    window.alert(result.message)
    if (result.ok) setOptionsFor(null)
  }

  async function restoreOriginal() {
    if (!optionsFor) return
    const result = await window.mvm.clearOverride(optionsFor)
    window.alert(result.message)
    if (result.ok) setOptionsFor(null)
  }

  const canOverride = Boolean(optionsFor?.defaultActionId)
  const activeAction = optionsFor || previewFor || selectedAction
  const isChildOpen = Boolean(optionsFor || previewFor)

  function onCommandKeyDown(event: React.KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault()
      if (optionsFor) setOptionsFor(null)
      else if (previewFor) setPreviewFor(null)
      else if (clipboardMode) setClipboardMode(false)
      else window.mvm.hide()
      return
    }

    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault()
      if (activeAction && !optionsFor && !clipboardMode) {
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

    if (!isChildOpen && event.key === 'Tab' && createAction) {
      event.preventDefault()
      run(createAction)
    }
  }

  return (
    <main className="shell">
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
              if (!isChildOpen && !clipboardMode) setQuery(value)
            }}
            placeholder={placeholder}
            readOnly={isChildOpen || clipboardMode}
            spellCheck={false}
          />
          {!isChildOpen && createAction ? <div className="tabHint">✨ <kbd>Tab</kbd> to automate</div> : null}
        </div>

        <Command.List className={`results card ${isChildOpen ? 'optionsCard' : 'resultsCard'}`}>
          {previewFor ? (
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
            <>
              <Command.Item value="option:shortcut" className="result" onSelect={setShortcut}>
                <span className="resultIcon"><Keyboard size={18} /></span>
                <span className="resultText">
                  <strong>Set keyboard shortcut</strong>
                  <small>Run this action globally without opening Nevermind</small>
                </span>
                <span className="enterHint">↵</span>
              </Command.Item>
              <Command.Item value="option:preview" className="result" onSelect={() => optionsFor && setPreviewFor(optionsFor)}>
                <span className="resultIcon"><Search size={18} /></span>
                <span className="resultText">
                  <strong>Preview</strong>
                  <small>Press → to preview an item</small>
                </span>
                <span className="enterHint">↵</span>
              </Command.Item>
              <Command.Item value="option:alias" className="result" onSelect={setAlias}>
                <span className="resultIcon"><Tag size={18} /></span>
                <span className="resultText">
                  <strong>Set alias</strong>
                  <small>Make this action appear for another phrase</small>
                </span>
                <span className="enterHint">↵</span>
              </Command.Item>
              {canOverride ? (
                <Command.Item value="option:override" className="result" onSelect={setOverride}>
                  <span className="resultIcon"><Sparkles size={18} /></span>
                  <span className="resultText">
                    <strong>Override with AI</strong>
                    <small>Customize this action to do what you want</small>
                  </span>
                  <span className="enterHint">↵</span>
                </Command.Item>
              ) : null}
              {optionsFor.isOverridden ? (
                <Command.Item value="option:restore" className="result" onSelect={restoreOriginal}>
                  <span className="resultIcon"><RotateCcw size={18} /></span>
                  <span className="resultText">
                    <strong>Restore original</strong>
                    <small>Remove the AI override and use the built-in behavior</small>
                  </span>
                  <span className="enterHint">↵</span>
                </Command.Item>
              ) : null}
            </>
          ) : (
            actions.map((action) => {
              const Icon = iconFor[action.icon]
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

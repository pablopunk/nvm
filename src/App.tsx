import { useEffect, useMemo, useRef, useState } from 'react'
import { Command } from 'cmdk'
import { AppWindow, Clipboard, Globe, Search, Sparkles } from 'lucide-react'

type ActionKind = 'open-url' | 'web-search' | 'app' | 'clipboard' | 'ai-placeholder'

type AppInfo = {
  path?: string
}

type Action = {
  id: string
  kind: ActionKind
  title: string
  subtitle: string
  icon: 'globe' | 'search' | 'app' | 'clipboard' | 'sparkles'
  score: number
  iconUrl?: string | null
  url?: string
  query?: string
  text?: string
  app?: AppInfo
}

declare global {
  interface Window {
    mvm: {
      search: (query: string) => Promise<Action[]>
      execute: (action: Action) => Promise<void>
      getAppIcon: (appPath: string) => Promise<string | null>
      hide: () => Promise<void>
      onShown: (callback: () => void) => () => void
      onHidden: (callback: () => void) => () => void
      onAppsIndexed: (callback: (count: number) => void) => () => void
    }
  }
}

const iconFor = {
  globe: Globe,
  search: Search,
  app: AppWindow,
  clipboard: Clipboard,
  sparkles: Sparkles,
}

export function App() {
  const inputRef = useRef<HTMLInputElement>(null)
  const requestedIcons = useRef(new Set<string>())
  const [query, setQuery] = useState('')
  const [actions, setActions] = useState<Action[]>([])
  const [iconUrls, setIconUrls] = useState<Record<string, string | null>>({})

  useEffect(() => {
    const focusInput = () => {
      inputRef.current?.focus()
      inputRef.current?.select()
    }

    const stopShown = window.mvm.onShown(() => {
      requestAnimationFrame(focusInput)
      window.setTimeout(focusInput, 50)
    })
    const stopHidden = window.mvm.onHidden(() => setQuery(''))
    const stopApps = window.mvm.onAppsIndexed(() => {})
    return () => {
      stopShown()
      stopHidden()
      stopApps()
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const timer = window.setTimeout(async () => {
      const next = await window.mvm.search(query)
      if (!cancelled) setActions(next)
    }, 20)

    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [query])

  useEffect(() => {
    for (const action of actions) {
      if (action.kind !== 'app' || !action.app?.path || requestedIcons.current.has(action.id)) continue

      requestedIcons.current.add(action.id)
      window.mvm.getAppIcon(action.app.path).then((iconUrl) => {
        setIconUrls((current) => ({ ...current, [action.id]: iconUrl }))
      })
    }
  }, [actions])

  const createAction = useMemo(
    () => actions.find((action) => action.kind === 'ai-placeholder'),
    [actions],
  )
  const placeholder = 'What do you wanna do?'

  async function run(action: Action) {
    await window.mvm.execute(action)
  }

  function onCommandKeyDown(event: React.KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault()
      window.mvm.hide()
      return
    }

    if (event.key === 'Tab' && createAction) {
      event.preventDefault()
      run(createAction)
    }
  }

  return (
    <main className="shell">
      <Command
        className="palette"
        label="Nevermind"
        loop
        shouldFilter={false}
        onKeyDown={onCommandKeyDown}
      >
        <div className="searchRow">
          <Sparkles className="brandIcon" size={22} />
          <Command.Input
            ref={inputRef}
            value={query}
            onValueChange={setQuery}
            placeholder={placeholder}
            spellCheck={false}
          />
          {createAction ? <div className="tabHint">✨ <kbd>Tab</kbd> to automate</div> : null}
        </div>

        <Command.List className="results">
          {actions.map((action) => {
            const Icon = iconFor[action.icon]
            const iconUrl = action.iconUrl ?? iconUrls[action.id]
            return (
              <Command.Item
                key={action.id}
                value={action.id}
                className="result"
                onSelect={() => run(action)}
              >
                <span className="resultIcon">
                  {iconUrl ? <img src={iconUrl} alt="" /> : <Icon size={18} />}
                </span>
                <span className="resultText">
                  <strong>{action.title}</strong>
                  <small>{action.subtitle}</small>
                </span>
                <span className="enterHint">↵</span>
              </Command.Item>
            )
          })}

          {actions.length === 0 ? (
            <Command.Empty className="empty">
              <Sparkles size={24} />
              <strong>Type anything.</strong>
              <span>Nevermind starts with local actions; AI planning comes next.</span>
            </Command.Empty>
          ) : null}
        </Command.List>
      </Command>
    </main>
  )
}

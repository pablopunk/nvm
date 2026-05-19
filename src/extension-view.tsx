import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { CornerDownLeft, Search, Square } from 'lucide-react'
import { actionsFromPanel, type CommandAction, type CommandItem, type CommandView } from './model'
import { ChatView, CommandRow, CommandTile, EmptyState, FormView, GridView, ListView, PreviewView, ProgressView, shortcutLabel, EMPTY_ITEMS_TITLE } from './ui'
import { RootCommandList } from './command-list'
import { iconForItem } from './command-icons'

type AiChatState = {
  messages: NonNullable<CommandView['messages']>
  input: string
  setInput: (value: string) => void
  busy: boolean
  inputRef: React.RefObject<HTMLTextAreaElement | null>
  messagesRef: React.RefObject<HTMLDivElement | null>
  resizeInput: (textarea?: HTMLTextAreaElement | null) => void
}

export type ExtensionViewRendererProps = {
  view: CommandView
  aiChat: AiChatState
  formValues: Record<string, string | boolean>
  setFormValues: React.Dispatch<React.SetStateAction<Record<string, string | boolean>>>
  filterItems: (items?: CommandItem[]) => CommandItem[]
  filterSections: (view: CommandView) => CommandView['sections']
  renderMarkdown: (content: string) => ReactNode
  renderActionPanel: (rows: unknown[], emptyMessage?: string) => ReactNode
  actionPanelRows: (panel?: CommandView['actionPanel'], fallbackActions?: CommandAction[], prefix?: string, closeAfterSelect?: boolean) => unknown[]
  renderRootIcon: (item: CommandItem) => ReactNode
  renderEmpty?: (view: CommandView, fallback?: string) => ReactNode
  runDefaultAction: (item: CommandItem) => void
  runAction: (action: CommandAction) => void
  sendAiPrompt: (message: string) => void
  abortAiChat: (chatId?: string) => void
  dragPathForItem: (item: CommandItem) => string | null | undefined
  startItemDrag: (event: React.DragEvent, item: CommandItem) => void
}

function tileActionHint(item: CommandItem) {
  if (item.actionPanelVisibility === 'hidden') return null
  const actions = actionsFromPanel(item.actionPanel, item.actions || [])
  if (actions.length === 0) return null
  const shortcut = actions.find((action) => action.shortcut)?.shortcut
  return <span className="tileActionHint">{shortcut ? shortcutLabel(shortcut) : '⌘K'}</span>
}

function visibleActionPanelRows(view: CommandView, rows: unknown[]) {
  return view.actionPanelVisibility === 'hidden' ? [] : rows
}

function gridStyle(view: CommandView) {
  return {
    ...(view.columns ? { '--grid-columns': String(view.columns) } : {}),
    ...(view.aspectRatio ? { '--tile-aspect-ratio': String(view.aspectRatio) } : {}),
  } as CSSProperties
}

function fallbackEmpty(view: CommandView, fallback = EMPTY_ITEMS_TITLE) {
  return <EmptyState icon={<Search size={24} />} title={view.emptyView?.title || fallback} subtitle={view.emptyView?.subtitle} />
}

function CameraView({ view, actions }: { view: CommandView; actions: ReactNode }) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const storageKey = `camera-device:${view.id || view.title}`
  const [selectedDeviceId, setSelectedDeviceId] = useState(() => view.deviceId || localStorage.getItem(storageKey) || '')
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([])
  const [activeDeviceLabel, setActiveDeviceLabel] = useState('')
  const [status, setStatus] = useState('Initializing…')
  const [error, setError] = useState('')

  useEffect(() => setSelectedDeviceId(view.deviceId || localStorage.getItem(storageKey) || ''), [storageKey, view.deviceId])

  useEffect(() => {
    let stream: MediaStream | null = null
    let cancelled = false

    async function start() {
      setStatus('Requesting camera…')
      setError('')
      try {
        const access = await window.nvm.requestCameraAccess()
        if (access.status === 'denied' || access.status === 'restricted' || access.status === 'unsupported') throw new Error(access.status === 'denied' ? 'Camera access is denied in System Settings.' : `Camera access is ${access.status}.`)
        const constraints: MediaTrackConstraints = selectedDeviceId
          ? { deviceId: { exact: selectedDeviceId }, width: { ideal: 1280 }, height: { ideal: 720 } }
          : { width: { ideal: 1280 }, height: { ideal: 720 } }
        stream = await navigator.mediaDevices.getUserMedia({ video: constraints, audio: false })
        if (cancelled) {
          stream.getTracks().forEach((track) => track.stop())
          return
        }
        const videoElement = videoRef.current
        if (!videoElement) throw new Error('Camera view is unavailable.')
        streamRef.current?.getTracks().forEach((track) => track.stop())
        streamRef.current = stream
        setActiveDeviceLabel(stream.getVideoTracks()[0]?.label || '')
        setDevices((await navigator.mediaDevices.enumerateDevices()).filter((device) => device.kind === 'videoinput'))
        setStatus('Stream acquired…')
        videoElement.srcObject = stream
        videoElement.onloadedmetadata = () => videoElement.play().catch(() => {})
        videoElement.onplaying = () => setStatus('Live')
        window.setTimeout(() => {
          if (!cancelled && streamRef.current === stream && videoElement.readyState > 0) setStatus('Live')
        }, 1_000)
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err)
        setStatus('Camera unavailable')
        setError(message)
      }
    }

    start()
    return () => {
      cancelled = true
      if (streamRef.current === stream) {
        streamRef.current = null
        if (videoRef.current?.srcObject === stream) videoRef.current.srcObject = null
      }
      stream?.getTracks().forEach((track) => track.stop())
    }
  }, [selectedDeviceId, storageKey])

  function switchDevice() {
    if (devices.length < 2) return
    const activeIndex = activeDeviceLabel ? devices.findIndex((device) => device.label === activeDeviceLabel) : -1
    const selectedIndex = devices.findIndex((device) => device.deviceId === selectedDeviceId)
    const currentIndex = activeIndex >= 0 ? activeIndex : selectedIndex >= 0 ? selectedIndex : 0
    const next = devices[(currentIndex + 1) % devices.length]
    localStorage.setItem(storageKey, next.deviceId)
    setSelectedDeviceId(next.deviceId)
  }

  const currentDevice = devices.find((device) => device.deviceId === selectedDeviceId) || devices.find((device) => device.label === activeDeviceLabel) || devices[0]
  const switcher = view.showDeviceSwitcher === false || devices.length < 2 ? null : <button className="cameraSwitchButton" type="button" onClick={switchDevice}>Switch Camera{currentDevice?.label ? ` · ${currentDevice.label}` : ''}</button>

  return <div className={`cameraSurface ${view.size === 'large' || view.presentation === 'preview' ? 'cameraLarge' : ''}`}><div className="cameraFrame"><video ref={videoRef} className="cameraVideo" autoPlay playsInline muted={view.muted !== false} controls={Boolean(view.controls)} />{switcher}{error ? <div className="cameraError"><strong>Camera Issue</strong><span>{error}</span></div> : null}<div className="cameraStatus">{status}</div></div>{actions}</div>
}

export function ExtensionViewRenderer({ view, aiChat, formValues, setFormValues, filterItems, filterSections, renderMarkdown, renderActionPanel, actionPanelRows, renderRootIcon, renderEmpty = fallbackEmpty, runDefaultAction, runAction, sendAiPrompt, abortAiChat, dragPathForItem, startItemDrag }: ExtensionViewRendererProps) {
  function pagination() {
    if (!view.pagination?.hasMore || !view.pagination.onLoadMore) return null
    return <button className="loadMoreButton" type="button" onClick={() => runAction(view.pagination!.onLoadMore!)}>Load More</button>
  }

  if (view.type === 'grid') {
    return <GridView
      items={filterItems(view.items)}
      sections={filterSections(view)}
      subtitle={view.subtitle}
      layout={view.layout || 'square'}
      style={gridStyle(view)}
      empty={renderEmpty(view)}
      isLoading={view.isLoading}
      pagination={pagination()}
      renderItem={(item) => <CommandTile key={item.id} value={item.id} title={item.title} subtitle={item.subtitle} image={item.image} video={item.video || item.videoUrl} actionHint={tileActionHint(item)} draggable={Boolean(dragPathForItem(item))} onDragStart={(event) => startItemDrag(event, item)} onSelect={() => runDefaultAction(item)} />}
    />
  }

  if (view.type === 'list') {
    const items = filterItems(view.items)
    if (view.presentation === 'root') return <RootCommandList items={items} iconForItem={renderRootIcon} onSelect={runDefaultAction} emptyTitle={view.emptyView?.title || EMPTY_ITEMS_TITLE} emptySubtitle={view.emptyView?.subtitle} />
    return <ListView
      items={items}
      sections={filterSections(view)}
      empty={renderEmpty(view)}
      isLoading={view.isLoading}
      pagination={pagination()}
      renderItem={(item) => <CommandRow key={item.id} value={item.id} className="result extensionListItem" icon={iconForItem(item)} title={item.title} subtitle={item.subtitle || item.text} accessories={item.accessories} shortcut={item.actionPanelVisibility === 'hidden' ? undefined : actionsFromPanel(item.actionPanel, item.actions || []).find((action) => action.shortcut)?.shortcut} onSelect={() => runDefaultAction(item)} />}
    />
  }

  if (view.type === 'chat') {
    const messages = (view.aiChat ? aiChat.messages : view.messages || []).map((message) => ({ ...message, content: renderMarkdown(message.content) }))
    const input = view.aiChat ? <form className="chatInputRow" onSubmit={(event) => { event.preventDefault(); sendAiPrompt(aiChat.input) }}><textarea ref={aiChat.inputRef} rows={1} value={aiChat.input} onChange={(event) => aiChat.setInput(event.target.value)} onInput={(event) => aiChat.resizeInput(event.currentTarget)} onKeyDown={(event) => { if (event.key !== 'Enter') return; event.stopPropagation(); if (!event.shiftKey) { event.preventDefault(); sendAiPrompt(aiChat.input) } }} placeholder={aiChat.busy ? 'Thinking…' : 'Message AI'} />{aiChat.busy ? <button className="chatIconButton chatStopButton" type="button" aria-label="Stop" title="Stop" onClick={() => abortAiChat(view.chatId)}><Square size={14} fill="currentColor" /></button> : <button className="chatIconButton chatEnterButton" type="submit" aria-label="Enter" title="Enter" disabled={!aiChat.input.trim()}><CornerDownLeft size={16} /></button>}</form> : null
    return <ChatView messages={messages} isBusy={aiChat.busy} input={input} messagesRef={view.aiChat ? aiChat.messagesRef : undefined} />
  }

  if (view.type === 'form') return <FormView fields={view.fields || []} values={formValues} onChange={(id, value) => setFormValues((current) => ({ ...current, [id]: value }))} onSubmit={view.submitAction ? () => runAction({ ...view.submitAction!, formValues }) : undefined} submitTitle={view.submitAction?.title} />

  if (view.type === 'progress') return <ProgressView steps={view.steps || []} />

  if (view.type === 'webview') {
    const webviewActionRows = visibleActionPanelRows(view, actionPanelRows(view.actionPanel, view.actions || [], 'extension-webview', false))
    const webviewActions = webviewActionRows.length ? renderActionPanel(webviewActionRows) : null
    return <div className={`webviewSurface ${view.size === 'large' || view.presentation === 'preview' ? 'webviewLarge' : ''}`}><iframe className="extensionWebview" title={view.title} srcDoc={view.html || view.content || ''} sandbox="allow-scripts allow-forms allow-same-origin" allow="camera; microphone; display-capture; autoplay; clipboard-read; clipboard-write" />{webviewActions}</div>
  }

  if (view.type === 'camera') {
    const cameraActionRows = visibleActionPanelRows(view, actionPanelRows(view.actionPanel, view.actions || [], 'extension-camera', false))
    const cameraActions = cameraActionRows.length ? renderActionPanel(cameraActionRows) : null
    return <CameraView view={view} actions={cameraActions} />
  }

  const previewActionRows = visibleActionPanelRows(view, actionPanelRows(view.actionPanel, view.actions || [], 'extension-view', false))
  const previewActions = previewActionRows.length ? renderActionPanel(previewActionRows) : null
  return <div className={view.presentation === 'preview' ? 'previewMode' : undefined}><PreviewView content={view.content || view.subtitle || ''} image={view.image} video={view.video || view.videoUrl} actions={previewActions} /></div>
}

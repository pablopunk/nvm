import React, { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { CornerDownLeft, CreditCard, LogIn, Search, Square } from 'lucide-react'
import { actionsFromPanel, type CommandAction, type CommandItem, type CommandView } from './model'
import type { AiLimitState } from './use-ai-chat'
import { ChatView, CommandRow, CommandTile, EditorView, EmptyState, FormView, GridView, ListView, PreviewView, ProgressView, shortcutLabel, EMPTY_ITEMS_TITLE, type FormValue } from './ui'
import { RootCommandList } from './command-list'
import { iconForItem } from './command-icons'

type AiChatState = {
  messages: NonNullable<CommandView['messages']>
  input: string
  setInput: (value: string) => void
  busy: boolean
  limit: AiLimitState | null
  inputRef: React.RefObject<HTMLTextAreaElement | null>
  messagesRef: React.RefObject<HTMLDivElement | null>
  resizeInput: (textarea?: HTMLTextAreaElement | null) => void
}

export type ExtensionViewRendererProps = {
  view: CommandView
  aiChat: AiChatState
  nevermindAuthed: boolean | null
  onSignInToNevermind: () => void
  formValues: Record<string, FormValue>
  setFormValues: React.Dispatch<React.SetStateAction<Record<string, FormValue>>>
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
  selectedItemId?: string
}

function tileActionHint(item: CommandItem) {
  if (item.actionPanelVisibility === 'hidden') return null
  const actions = actionsFromPanel(item.actionPanel, item.actions || [])
  if (actions.length === 0) return null
  const shortcut = actions.find((action) => action.shortcut)?.shortcut
  return <span className="tileActionHint">{shortcut ? shortcutLabel(shortcut) : '⌘K'}</span>
}

function visibleActionPanelRows(view: CommandView, rows: unknown[]) {
  return view.actionPanelVisibility === 'hidden' || view.actionPanelVisibility === 'menu' ? [] : rows
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

function imageSource(image: CommandItem['image']) {
  if (!image) return ''
  if (typeof image === 'string') return image
  return image.dark || image.src || image.light || image.fallback || ''
}

function MetadataRows({ items = [] }: { items?: NonNullable<CommandItem['detail']>['metadata'] }) {
  if (!items?.length) return null
  return <dl className="extensionMetadata">{items.map((item, index) => {
    if (item.type === 'separator') return <div key={index} className="metadataSeparator" />
    if (item.type === 'tag') return <div key={index} className="metadataTagRow"><dt>{item.label || 'Tag'}</dt><dd><span className="accessory metadataTag" data-tone={item.tone || 'default'}>{item.value}</span></dd></div>
    if (item.type === 'link') return <div key={index}><dt>{item.label}</dt><dd><a href={item.url}>{item.value}</a></dd></div>
    return <div key={index}><dt>{item.label}</dt><dd>{item.value}</dd></div>
  })}</dl>
}

function ExtensionItemDetail({ item, renderMarkdown, renderActionPanel, actionPanelRows }: { item?: CommandItem; renderMarkdown: (content: string) => ReactNode; renderActionPanel: (rows: unknown[], emptyMessage?: string) => ReactNode; actionPanelRows: ExtensionViewRendererProps['actionPanelRows'] }) {
  const detail = item?.detail
  if (!item || !detail) return null
  const actions = detail.actions?.length ? renderActionPanel(actionPanelRows({ sections: [{ actions: detail.actions }] }, [], `detail-${item.id}`, false)) : null
  const image = imageSource(detail.image || item.image)
  return <aside className="extensionDetailPane"><div className="extensionDetailHeader">{image ? <img src={image} alt="" /> : null}<div><strong>{detail.title || item.title}</strong>{detail.subtitle || item.subtitle ? <small>{detail.subtitle || item.subtitle}</small> : null}</div></div>{detail.markdown ? <div className="previewText detailMarkdown">{renderMarkdown(detail.markdown)}</div> : null}<MetadataRows items={detail.metadata} />{actions}</aside>
}

function NevermindSignInGate({ onSignIn }: { onSignIn: () => void }) {
  const [busy, setBusy] = useState(false)
  async function handle() {
    if (busy) return
    setBusy(true)
    try { await onSignIn() } finally { setBusy(false) }
  }
  return <EmptyState icon={<LogIn size={24} />} title="Sign in to Nevermind" subtitle="Connect this device to your Nevermind account to use AI chats." action={{ value: 'sign-in', icon: <LogIn size={16} />, title: busy ? 'Opening browser…' : 'Sign in to Nevermind', onSelect: handle }} />
}

export function NevermindLimitGate({ limit, runAction }: { limit: AiLimitState; runAction: (action: CommandAction) => void }) {
  const action = limit.action ? {
    value: 'run-limit-action',
    icon: <CreditCard size={16} />,
    title: limit.actionTitle || limit.action.title,
    onSelect: () => runAction(limit.action!),
  } : limit.dashboardUrl ? {
    value: 'open-dashboard',
    icon: <CreditCard size={16} />,
    title: limit.actionTitle || 'Open Dashboard',
    onSelect: () => runAction({ type: 'openUrl', title: limit.actionTitle || 'Open Dashboard', url: limit.dashboardUrl }),
  } : undefined
  return <EmptyState icon={<CreditCard size={24} />} title={limit.title} subtitle={limit.message} action={action} />
}

function EditorSurface({ view, actions, renderMarkdown, runAction }: { view: CommandView; actions: ReactNode; renderMarkdown: (content: string) => ReactNode; runAction: (action: CommandAction) => void }) {
  const [value, setValue] = useState(view.content || '')
  const viewKey = `${view.id || ''}:${view.title}`
  useEffect(() => setValue(view.content || ''), [viewKey, view.content])
  const preview = view.format === 'markdown' ? renderMarkdown(value) : undefined
  return <EditorView
    value={value}
    format={view.format || 'text'}
    language={view.language}
    placeholder={view.placeholder}
    readOnly={view.readOnly}
    preview={preview}
    actions={actions}
    submitTitle={view.submitAction?.title}
    onChange={setValue}
    onSubmit={view.submitAction ? () => runAction({ ...view.submitAction!, editorContent: value }) : undefined}
  />
}

function CameraView({ view, actions }: { view: CommandView; actions: ReactNode }) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const storageKey = `camera-device:${view.id || view.title}`
  const [selectedDeviceId, setSelectedDeviceId] = useState(() => view.deviceId || localStorage.getItem(storageKey) || '')
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([])
  const [activeDeviceLabel, setActiveDeviceLabel] = useState('')
  const [muted, setMuted] = useState(view.muted !== false)
  const [controls, setControls] = useState(Boolean(view.controls))
  const [status, setStatus] = useState('Initializing…')
  const [error, setError] = useState('')

  useEffect(() => setSelectedDeviceId(view.deviceId || localStorage.getItem(storageKey) || ''), [storageKey, view.deviceId])
  useEffect(() => setMuted(view.muted !== false), [view.muted])
  useEffect(() => setControls(Boolean(view.controls)), [view.controls])

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

  function switchDevice(direction: 'next' | 'previous' = 'next') {
    if (devices.length < 2) return
    const activeIndex = activeDeviceLabel ? devices.findIndex((device) => device.label === activeDeviceLabel) : -1
    const selectedIndex = devices.findIndex((device) => device.deviceId === selectedDeviceId)
    const currentIndex = activeIndex >= 0 ? activeIndex : selectedIndex >= 0 ? selectedIndex : 0
    const offset = direction === 'previous' ? -1 : 1
    const next = devices[(currentIndex + offset + devices.length) % devices.length]
    localStorage.setItem(storageKey, next.deviceId)
    setSelectedDeviceId(next.deviceId)
  }

  useEffect(() => {
    function handleCameraAction(event: Event) {
      const detail = (event as CustomEvent<{ kind?: string; direction?: 'next' | 'previous' }>).detail
      if (detail?.kind === 'camera.switchDevice' || detail?.kind === 'camera.nextDevice') switchDevice('next')
      else if (detail?.kind === 'camera.previousDevice') switchDevice('previous')
      else if (detail?.kind === 'camera.toggleMuted') setMuted((value) => !value)
      else if (detail?.kind === 'camera.toggleControls') setControls((value) => !value)
    }
    window.addEventListener('nvm:camera-action', handleCameraAction)
    return () => window.removeEventListener('nvm:camera-action', handleCameraAction)
  })

  const currentDevice = devices.find((device) => device.deviceId === selectedDeviceId) || devices.find((device) => device.label === activeDeviceLabel) || devices[0]
  const switcher = view.showDeviceSwitcher === false || devices.length < 2 ? null : <button className="cameraSwitchButton" type="button" onClick={() => switchDevice()}>Switch Camera{currentDevice?.label ? ` · ${currentDevice.label}` : ''}</button>

  return <div className={`cameraSurface ${view.size === 'large' || view.presentation === 'preview' ? 'cameraLarge' : ''}`}><div className="cameraFrame"><video ref={videoRef} className="cameraVideo" autoPlay playsInline muted={muted} controls={controls} />{switcher}{error ? <div className="cameraError"><strong>Camera Issue</strong><span>{error}</span></div> : null}<div className="cameraStatus">{status}</div></div>{actions}</div>
}

type ExtensionViewSurfaceProps = ExtensionViewRendererProps & { renderEmpty: NonNullable<ExtensionViewRendererProps['renderEmpty']> }

export const EXTENSION_WEBVIEW_SANDBOX = 'allow-scripts allow-forms'
export const DEFAULT_EXTENSION_WEBVIEW_PERMISSIONS = ['autoplay'] as const
export const EXTENSION_WEBVIEW_PERMISSION_ALLOW_VALUES = {
  autoplay: 'autoplay',
  camera: 'camera',
  microphone: 'microphone',
  'display-capture': 'display-capture',
  'clipboard-read': 'clipboard-read',
  'clipboard-write': 'clipboard-write',
} as const
export const EXTENSION_WEBVIEW_ALLOW = DEFAULT_EXTENSION_WEBVIEW_PERMISSIONS.map((permission) => EXTENSION_WEBVIEW_PERMISSION_ALLOW_VALUES[permission]).join('; ')

export function extensionWebviewAllow(webviewPermissions?: readonly string[]) {
  const permissions = webviewPermissions?.length ? webviewPermissions : DEFAULT_EXTENSION_WEBVIEW_PERMISSIONS
  return Array.from(new Set(permissions))
    .map((permission) => EXTENSION_WEBVIEW_PERMISSION_ALLOW_VALUES[permission as keyof typeof EXTENSION_WEBVIEW_PERMISSION_ALLOW_VALUES])
    .filter(Boolean)
    .join('; ')
}

function ViewPagination({ view, runAction }: Pick<ExtensionViewSurfaceProps, 'view' | 'runAction'>) {
  if (!view.pagination?.hasMore || !view.pagination.onLoadMore) return null
  return <button className="loadMoreButton" type="button" onClick={() => runAction(view.pagination!.onLoadMore!)}>Load More</button>
}

function GridExtensionView({ view, filterItems, filterSections, renderEmpty, runDefaultAction, dragPathForItem, startItemDrag, runAction }: ExtensionViewSurfaceProps) {
  return <GridView
    items={filterItems(view.items)}
    sections={filterSections(view)}
    subtitle={view.subtitle}
    layout={view.layout || 'square'}
    style={gridStyle(view)}
    empty={renderEmpty(view)}
    isLoading={view.isLoading}
    pagination={<ViewPagination view={view} runAction={runAction} />}
    renderItem={(item) => <CommandTile key={item.id} value={item.id} title={item.title} subtitle={item.subtitle} image={item.image} video={item.video || item.videoUrl} actionHint={tileActionHint(item)} appearance={item.appearance} draggable={Boolean(dragPathForItem(item))} onDragStart={(event) => startItemDrag(event, item)} onSelect={() => runDefaultAction(item)} />}
  />
}

function ListExtensionView({ view, filterItems, filterSections, renderRootIcon, renderEmpty, renderMarkdown, renderActionPanel, actionPanelRows, runDefaultAction, runAction, selectedItemId }: ExtensionViewSurfaceProps) {
  const items = filterItems(view.items)
  if (view.presentation === 'root') return <RootCommandList items={items} iconForItem={renderRootIcon} onSelect={runDefaultAction} emptyTitle={view.emptyView?.title || EMPTY_ITEMS_TITLE} emptySubtitle={view.emptyView?.subtitle} />
  const selected = items.find((item) => item.id === selectedItemId) || items[0]
  const list = <ListView
    items={items}
    sections={filterSections(view)}
    empty={renderEmpty(view)}
    isLoading={view.isLoading}
    pagination={<ViewPagination view={view} runAction={runAction} />}
    renderItem={(item) => <CommandRow key={item.id} value={item.id} className="result extensionListItem" icon={iconForItem(item)} title={item.title} subtitle={item.subtitle || item.text} accessories={item.accessories} shortcut={item.actionPanelVisibility === 'hidden' ? undefined : actionsFromPanel(item.actionPanel, item.actions || []).find((action) => action.shortcut)?.shortcut} appearance={item.appearance} onSelect={() => runDefaultAction(item)} />}
  />
  if (view.detail?.visible && selected?.detail) return <div className={`extensionListWithDetail extensionListDetail-${view.detail.placement || 'side'}`}><div className="extensionListPane">{list}</div><ExtensionItemDetail item={selected} renderMarkdown={renderMarkdown} renderActionPanel={renderActionPanel} actionPanelRows={actionPanelRows} /></div>
  return list
}

function ChatInputForm({ value, onChange, onResize, onSubmit, busy, inputRef, placeholder, onAbort, chatId }: { value: string; onChange: (value: string) => void; onResize: (textarea?: HTMLTextAreaElement | null) => void; onSubmit: () => void; busy: boolean; inputRef: React.RefObject<HTMLTextAreaElement | null>; placeholder?: string; onAbort?: (chatId?: string) => void; chatId?: string }) {
  return <form className="chatInputRow" onSubmit={(event) => { event.preventDefault(); onSubmit() }}><textarea ref={inputRef} rows={1} value={value} onChange={(event) => onChange(event.target.value)} onInput={(event) => onResize(event.currentTarget)} onKeyDown={(event) => { if (event.key !== 'Enter') return; event.stopPropagation(); if (!event.shiftKey) { event.preventDefault(); onSubmit() } }} placeholder={busy ? 'Thinking…' : placeholder || 'Message AI'} />{busy ? <button className="chatIconButton chatStopButton" type="button" aria-label="Stop" title="Stop" onClick={() => onAbort?.(chatId)}><Square size={14} fill="currentColor" /></button> : <button className="chatIconButton chatEnterButton" type="submit" aria-label="Enter" title="Enter" disabled={!value.trim()}><CornerDownLeft size={16} /></button>}</form>
}

function ChatExtensionView({ view, aiChat, nevermindAuthed, onSignInToNevermind, renderMarkdown, runAction, sendAiPrompt, abortAiChat }: ExtensionViewSurfaceProps) {
  if (view.aiChat && nevermindAuthed === false) return <NevermindSignInGate onSignIn={onSignInToNevermind} />
  if (view.aiChat && aiChat.limit) return <NevermindLimitGate limit={aiChat.limit} runAction={runAction} />
  const messages = (view.aiChat ? aiChat.messages : view.messages || []).map((message) => ({ ...message, content: renderMarkdown(message.content) }))
  const chatInputRef = useRef<HTMLTextAreaElement>(null)
  const [chatValue, setChatValue] = useState('')
  function resizeChatInput(textarea?: HTMLTextAreaElement | null) {
    const el = textarea || chatInputRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 120)}px`
    el.style.overflowY = el.scrollHeight > 120 ? 'auto' : 'hidden'
  }
  useLayoutEffect(() => { resizeChatInput() }, [])
  useLayoutEffect(() => {
    if (view.aiChat || !view.submitAction) return
    const frame = requestAnimationFrame(() => chatInputRef.current?.focus())
    return () => cancelAnimationFrame(frame)
  }, [view.aiChat, view.id, view.title])
  function handleNonAiSubmit() {
    const trimmed = chatValue.trim()
    if (!trimmed) return
    setChatValue('')
    runAction({ ...view.submitAction!, formValues: { message: trimmed } })
  }
  const input = view.aiChat
    ? <ChatInputForm value={aiChat.input} onChange={aiChat.setInput} onResize={aiChat.resizeInput} onSubmit={() => sendAiPrompt(aiChat.input)} busy={aiChat.busy} inputRef={aiChat.inputRef} placeholder={aiChat.busy ? 'Thinking…' : 'Message AI'} onAbort={abortAiChat} chatId={view.chatId} />
    : view.submitAction
      ? <ChatInputForm value={chatValue} onChange={setChatValue} onResize={resizeChatInput} onSubmit={handleNonAiSubmit} busy={false} inputRef={chatInputRef} placeholder={view.placeholder || 'Type a message…'} />
      : null
  return <ChatView messages={messages} isBusy={view.aiChat ? aiChat.busy : false} input={input} messagesRef={view.aiChat ? aiChat.messagesRef : undefined} />
}

function FormExtensionView({ view, formValues, setFormValues, runAction }: ExtensionViewSurfaceProps) {
  return <FormView fields={view.fields || []} values={formValues} onChange={(id, value) => setFormValues((current) => ({ ...current, [id]: value }))} onSubmit={view.submitAction ? () => runAction({ ...view.submitAction!, formValues }) : undefined} submitTitle={view.submitAction?.title} />
}

function EditorExtensionView({ view, renderMarkdown, renderActionPanel, actionPanelRows, runAction }: ExtensionViewSurfaceProps) {
  const editorActionRows = visibleActionPanelRows(view, actionPanelRows(view.actionPanel, view.actions || [], 'extension-editor', false))
  const editorActions = editorActionRows.length ? renderActionPanel(editorActionRows) : null
  return <EditorSurface view={view} actions={editorActions} renderMarkdown={renderMarkdown} runAction={runAction} />
}

function WebExtensionView({ view, renderActionPanel, actionPanelRows }: ExtensionViewSurfaceProps) {
  const webviewActionRows = visibleActionPanelRows(view, actionPanelRows(view.actionPanel, view.actions || [], 'extension-webview', false))
  const webviewActions = webviewActionRows.length ? renderActionPanel(webviewActionRows) : null
  return <div className={`webviewSurface ${view.size === 'large' || view.presentation === 'preview' ? 'webviewLarge' : ''}`}><iframe className="extensionWebview" title={view.title} srcDoc={view.html || view.content || ''} sandbox={EXTENSION_WEBVIEW_SANDBOX} allow={extensionWebviewAllow(view.webviewPermissions)} />{webviewActions}</div>
}

function CameraExtensionView({ view, renderActionPanel, actionPanelRows }: ExtensionViewSurfaceProps) {
  const cameraActionRows = visibleActionPanelRows(view, actionPanelRows(view.actionPanel, view.actions || [], 'extension-camera', false))
  const cameraActions = cameraActionRows.length ? renderActionPanel(cameraActionRows) : null
  return <CameraView view={view} actions={cameraActions} />
}

function PreviewExtensionView({ view, renderMarkdown, renderActionPanel, actionPanelRows }: ExtensionViewSurfaceProps) {
  const previewActionRows = visibleActionPanelRows(view, actionPanelRows(view.actionPanel, view.actions || [], 'extension-view', false))
  const previewActions = previewActionRows.length ? renderActionPanel(previewActionRows) : null
  const previewContent = view.content ? renderMarkdown(view.content) : view.subtitle || ''
  return <div className={view.presentation === 'preview' || view.size === 'large' ? 'previewMode' : undefined}><PreviewView content={previewContent} image={view.image} video={view.video || view.videoUrl} actions={previewActions} /></div>
}

export function ExtensionViewRenderer(props: ExtensionViewRendererProps) {
  const surfaceProps: ExtensionViewSurfaceProps = { ...props, renderEmpty: props.renderEmpty || fallbackEmpty }
  if (props.view.type === 'grid') return <GridExtensionView {...surfaceProps} />
  if (props.view.type === 'list') return <ListExtensionView {...surfaceProps} />
  if (props.view.type === 'chat') return <ChatExtensionView {...surfaceProps} />
  if (props.view.type === 'form') return <FormExtensionView {...surfaceProps} />
  if (props.view.type === 'editor') return <EditorExtensionView {...surfaceProps} />
  if (props.view.type === 'progress') return <ProgressView steps={props.view.steps || []} value={props.view.value} total={props.view.total} status={props.view.status} />
  if (props.view.type === 'webview') return <WebExtensionView {...surfaceProps} />
  if (props.view.type === 'camera') return <CameraExtensionView {...surfaceProps} />
  return <PreviewExtensionView {...surfaceProps} />
}

import type { CSSProperties, ReactNode } from 'react'
import { CornerDownLeft, Search, Sparkles, Square } from 'lucide-react'
import { actionsFromPanel, type CommandAction, type CommandItem, type CommandView } from './model'
import { ChatView, CommandRow, CommandTile, EmptyState, FormView, GridView, ListView, PreviewView, ProgressView, shortcutLabel } from './ui'
import { RootCommandList } from './command-list'
import { iconFor, type CommandIconName } from './command-icons'

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

function tileActionHint(actions: CommandAction[] = []) {
  if (actions.length === 0) return null
  const shortcut = actions.find((action) => action.shortcut)?.shortcut
  return <span className="tileActionHint">{shortcut ? shortcutLabel(shortcut) : '⌘K'}</span>
}

function gridStyle(view: CommandView) {
  return {
    ...(view.columns ? { '--grid-columns': String(view.columns) } : {}),
    ...(view.aspectRatio ? { '--tile-aspect-ratio': String(view.aspectRatio) } : {}),
  } as CSSProperties
}

function fallbackEmpty(view: CommandView, fallback = 'No items found') {
  return <EmptyState icon={<Search size={24} />} title={view.emptyView?.title || fallback} subtitle={view.emptyView?.subtitle} />
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
      renderItem={(item) => <CommandTile key={item.id} value={item.id} title={item.title} subtitle={item.subtitle} image={item.image} video={item.video || item.videoUrl} actionHint={tileActionHint(actionsFromPanel(item.actionPanel, item.actions || []))} draggable={Boolean(dragPathForItem(item))} onDragStart={(event) => startItemDrag(event, item)} onSelect={() => runDefaultAction(item)} />}
    />
  }

  if (view.type === 'list') {
    const items = filterItems(view.items)
    if (view.presentation === 'root') return <RootCommandList items={items} iconForItem={renderRootIcon} onSelect={runDefaultAction} emptyTitle={view.emptyView?.title || 'No items found'} emptySubtitle={view.emptyView?.subtitle} />
    return <ListView
      items={items}
      sections={filterSections(view)}
      empty={renderEmpty(view)}
      isLoading={view.isLoading}
      pagination={pagination()}
      renderItem={(item) => {
        const Icon = iconFor[(item.icon as CommandIconName) || 'sparkles'] ?? Sparkles
        return <CommandRow key={item.id} value={item.id} className="result extensionListItem" icon={item.image ? <span className="thumbnailIcon"><img src={item.image} alt="" /></span> : <Icon size={18} />} title={item.title} subtitle={item.subtitle || item.text} accessories={item.accessories} shortcut={actionsFromPanel(item.actionPanel, item.actions || []).find((action) => action.shortcut)?.shortcut} onSelect={() => runDefaultAction(item)} />
      }}
    />
  }

  if (view.type === 'chat') {
    const messages = (view.aiChat ? aiChat.messages : view.messages || []).map((message) => ({ ...message, content: renderMarkdown(message.content) }))
    const input = view.aiChat ? <form className="chatInputRow" onSubmit={(event) => { event.preventDefault(); sendAiPrompt(aiChat.input) }}><textarea ref={aiChat.inputRef} rows={1} value={aiChat.input} onChange={(event) => aiChat.setInput(event.target.value)} onInput={(event) => aiChat.resizeInput(event.currentTarget)} onKeyDown={(event) => { if (event.key !== 'Enter') return; event.stopPropagation(); if (!event.shiftKey) { event.preventDefault(); sendAiPrompt(aiChat.input) } }} placeholder={aiChat.busy ? 'Thinking…' : 'Message AI'} />{aiChat.busy ? <button className="chatIconButton chatStopButton" type="button" aria-label="Stop" title="Stop" onClick={() => abortAiChat(view.chatId)}><Square size={14} fill="currentColor" /></button> : <button className="chatIconButton chatEnterButton" type="submit" aria-label="Enter" title="Enter" disabled={!aiChat.input.trim()}><CornerDownLeft size={16} /></button>}</form> : null
    return <ChatView messages={messages} isBusy={aiChat.busy} input={input} messagesRef={view.aiChat ? aiChat.messagesRef : undefined} />
  }

  if (view.type === 'form') return <FormView fields={view.fields || []} values={formValues} onChange={(id, value) => setFormValues((current) => ({ ...current, [id]: value }))} onSubmit={view.submitAction ? () => runAction({ ...view.submitAction!, formValues }) : undefined} submitTitle={view.submitAction?.title} />

  if (view.type === 'progress') return <ProgressView steps={view.steps || []} />

  const previewActionRows = actionPanelRows(view.actionPanel, view.actions || [], 'extension-view', false)
  const previewActions = previewActionRows.length ? renderActionPanel(previewActionRows) : null
  return <div className={view.presentation === 'preview' ? 'previewMode' : undefined}><PreviewView content={view.content || view.subtitle || ''} image={view.image} video={view.video || view.videoUrl} actions={previewActions} /></div>
}

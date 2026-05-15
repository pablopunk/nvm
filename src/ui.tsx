import { type ReactNode } from 'react'
import { Command } from 'cmdk'
import { Folder } from 'lucide-react'

export type KeyHintsProps = { shortcut?: string; extras?: string[]; showEnter?: boolean }
export type CommandRowProps = { value: string; icon: ReactNode; title: string; subtitle?: string; accessories?: { text?: string; icon?: ReactNode }[]; shortcut?: string; extras?: string[]; className?: string; selectedOnlyShortcut?: boolean; onSelect: () => void }
export type CommandTileProps = { value: string; title: string; subtitle?: string; image?: string; video?: string; actionHint?: ReactNode; draggable?: boolean; onDragStart?: (event: React.DragEvent) => void; onSelect: () => void }
export type EmptyStateProps = { icon: ReactNode; title: string; subtitle?: string }
export type ToastProps = { message: string; tone?: 'default' | 'error' }
export type DetailViewProps = { content?: ReactNode; image?: string; video?: string; poster?: string; actions?: ReactNode }
export type ProgressViewProps = { steps: { title: string; status?: string }[] }
export type FormField = { id: string; label: string; type?: string; value?: string; placeholder?: string; required?: boolean }
export type FormViewProps = { fields: FormField[]; values?: Record<string, string | boolean>; onChange?: (id: string, value: string | boolean) => void; onSubmit?: () => void; submitTitle?: string }
export type ItemSection<T> = { title?: string; subtitle?: string; items: T[] }
export type ListViewProps<T> = { items?: T[]; sections?: ItemSection<T>[]; renderItem: (item: T) => ReactNode; empty?: ReactNode; subtitle?: string; isLoading?: boolean; pagination?: ReactNode }
export type GridViewProps<T> = { items?: T[]; sections?: ItemSection<T>[]; renderItem: (item: T) => ReactNode; empty?: ReactNode; subtitle?: string; layout?: string; style?: React.CSSProperties; isLoading?: boolean; pagination?: ReactNode }
export type ChatViewProps = { messages: { role: string; content: ReactNode }[]; isBusy?: boolean; input?: ReactNode; messagesRef?: React.RefObject<HTMLDivElement | null> }
export type ActionPanelRow = { value: string; icon?: ReactNode; title: string; subtitle?: string; shortcut?: string; className?: string; sectionHeader?: boolean; onSelect: () => void }
export type ActionPanelViewProps = { rows: ActionPanelRow[]; renderEmpty: () => ReactNode }

export function shortcutLabel(shortcut?: string) {
  return String(shortcut || '').split('+').map((part) => ({ Command: '⌘', Cmd: '⌘', Control: '⌃', Ctrl: '⌃', Alt: '⌥', Option: '⌥', Shift: '⇧', Enter: '↵', Return: '↵', Space: '␣', Escape: 'Esc', Tab: 'Tab' }[part] || part)).join('')
}

export function KeyHints({ shortcut, extras = [], showEnter = true }: KeyHintsProps) {
  return <span className="keyHints">{extras.map((extra) => <span key={extra} className="shortcutHint selectedOnlyEnter">{extra}</span>)}{shortcut ? <span className="shortcutHint">{shortcutLabel(shortcut)}</span> : null}{showEnter ? <span className="enterHint selectedOnlyEnter">↵</span> : null}</span>
}

export function CommandRow({ value, icon, title, subtitle, accessories = [], shortcut, extras, className, selectedOnlyShortcut = false, onSelect }: CommandRowProps) {
  return <Command.Item value={value} className={className || 'result'} onSelect={onSelect}><span className="resultIcon">{icon}</span><span className="resultText"><strong>{title}</strong><small>{subtitle}</small></span>{accessories.length ? <span className="accessories">{accessories.map((accessory, index) => <span key={index} className="accessory">{accessory.icon}{accessory.text}</span>)}</span> : null}{selectedOnlyShortcut ? (shortcut ? <span className="keyHints selectedOnlyEnter"><span className="shortcutHint">{shortcutLabel(shortcut)}</span><span className="enterHint">↵</span></span> : null) : <KeyHints shortcut={shortcut} extras={extras} />}</Command.Item>
}

export function CommandTile({ value, title, subtitle, image, video, actionHint, draggable, onDragStart, onSelect }: CommandTileProps) {
  return <Command.Item value={value} className="extensionTile" data-extension-item-id={value} draggable={draggable} onDragStart={onDragStart} onSelect={onSelect}><span className="tileMedia">{video ? <video src={video} poster={image} draggable={false} muted loop playsInline preload="metadata" onMouseEnter={(event) => event.currentTarget.play().catch(() => {})} onMouseLeave={(event) => event.currentTarget.pause()} /> : image ? <img src={image} alt="" draggable={false} loading="lazy" decoding="async" /> : <span className="tileIcon"><Folder size={20} /></span>}{actionHint}</span><strong>{title}</strong>{subtitle ? <small>{subtitle}</small> : null}</Command.Item>
}

export function EmptyState({ icon, title, subtitle = 'Try a different filter.' }: EmptyStateProps) {
  return <div className="empty">{icon}<strong>{title}</strong><span>{subtitle}</span></div>
}

export function Toast({ message, tone }: ToastProps) {
  return <div className={`toast ${tone === 'error' ? 'toastError' : ''}`}>{message}</div>
}

export function DetailView({ content, image, video, poster, actions }: DetailViewProps) {
  return <div className="extensionView">{video ? <video className="detailMedia" src={video} poster={poster || image} controls autoPlay muted loop playsInline /> : null}{!video && image ? <img className="detailMedia" src={image} alt="" /> : null}<pre className="previewText">{content}</pre>{actions}</div>
}

export function ProgressView({ steps }: ProgressViewProps) {
  return <div className="extensionView progressView">{steps.map((step, index) => <div key={index} className="progressStep"><strong>{step.title}</strong><small>{step.status || 'Pending'}</small></div>)}</div>
}

export function FormView({ fields, values = {}, onChange, onSubmit, submitTitle = 'Submit' }: FormViewProps) {
  return <form className="extensionView formView" onSubmit={(event) => { event.preventDefault(); onSubmit?.() }}>
    {fields.map((field) => <label key={field.id}><span>{field.label}</span><input value={String(values[field.id] ?? field.value ?? '')} placeholder={field.placeholder} required={field.required} type={field.type || 'text'} onChange={(event) => onChange?.(field.id, field.type === 'checkbox' ? event.currentTarget.checked : event.currentTarget.value)} /></label>)}
    {onSubmit ? <button className="formSubmitButton" type="submit">{submitTitle}</button> : null}
  </form>
}

function normalizedSections<T>(items?: T[], sections?: ItemSection<T>[]) {
  return sections?.length ? sections : [{ items: items || [] }]
}

export function ListView<T>({ items, sections, renderItem, empty, subtitle, isLoading, pagination }: ListViewProps<T>) {
  const visibleSections = normalizedSections(items, sections).filter((section) => section.items.length > 0)
  return <>{isLoading ? <div className="loadingBar" /> : null}{subtitle ? <div className="extensionSubtitle">{subtitle}</div> : null}{visibleSections.length > 0 ? visibleSections.map((section, index) => <div key={index} className="itemSection">{section.title ? <div className="actionSectionHeader">{section.title}</div> : null}{section.subtitle ? <div className="extensionSubtitle">{section.subtitle}</div> : null}{section.items.map(renderItem)}</div>) : empty}{pagination}</>
}

export function GridView<T>({ items, sections, renderItem, empty, subtitle, layout = 'square', style, isLoading, pagination }: GridViewProps<T>) {
  const visibleSections = normalizedSections(items, sections).filter((section) => section.items.length > 0)
  return <div className="extensionView">{isLoading ? <div className="loadingBar" /> : null}{subtitle ? <div className="extensionSubtitle">{subtitle}</div> : null}{visibleSections.length > 0 ? visibleSections.map((section, index) => <div key={index} className="itemSection">{section.title ? <div className="actionSectionHeader">{section.title}</div> : null}{section.subtitle ? <div className="extensionSubtitle">{section.subtitle}</div> : null}<div className={`extensionGrid extensionGrid-${layout}`} style={style}>{section.items.map(renderItem)}</div></div>) : empty}{pagination}</div>
}

export function ChatView({ messages, isBusy, input, messagesRef }: ChatViewProps) {
  return <div className="extensionView chatView"><div className="chatMessages" ref={messagesRef}>{messages.map((message, index) => <div key={index} className={`chatBubble ${message.role}`}>{message.content}</div>)}{isBusy ? <div className="chatBubble system">Thinking…</div> : null}</div>{input}</div>
}

export function ActionPanelView({ rows, renderEmpty }: ActionPanelViewProps) {
  if (rows.length === 0) return <>{renderEmpty()}</>
  return <>{rows.map((row) => row.sectionHeader ? <div key={row.value} className="actionSectionHeader">{row.title}</div> : <CommandRow key={row.value} value={row.value} icon={row.icon} title={row.title} subtitle={row.subtitle} shortcut={row.shortcut} className={row.className} onSelect={row.onSelect} />)}</>
}

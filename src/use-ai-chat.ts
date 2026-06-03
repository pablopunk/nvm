import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { CommandView } from './model'

export type AiChatEvent = { type: string; text?: string; message?: string; name?: string; chatId?: string; label?: string; data?: unknown }

export function useAiChat(sendMessage: (message: string, chatId?: string) => Promise<void>, resetChat: (chatId?: string) => Promise<void>) {
  const messagesRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const [messages, setMessages] = useState<NonNullable<CommandView['messages']>>([])
  const [input, setInput] = useState('')
  const [busy, setBusy] = useState(false)
  const pendingDeltaRef = useRef('')
  const deltaFrameRef = useRef<number | null>(null)

  function appendDeltaToMessages(current: NonNullable<CommandView['messages']>, text: string) {
    const last = current[current.length - 1]
    if (last?.role === 'assistant') return [...current.slice(0, -1), { ...last, content: `${last.content}${text}` }]
    return [...current, { role: 'assistant' as const, content: text }]
  }

  function cancelDeltaFlush() {
    if (deltaFrameRef.current == null) return
    cancelAnimationFrame(deltaFrameRef.current)
    deltaFrameRef.current = null
  }

  function flushDelta() {
    deltaFrameRef.current = null
    const text = pendingDeltaRef.current
    pendingDeltaRef.current = ''
    if (text) setMessages((current) => appendDeltaToMessages(current, text))
  }

  function appendPendingDelta(current: NonNullable<CommandView['messages']>) {
    const text = pendingDeltaRef.current
    pendingDeltaRef.current = ''
    cancelDeltaFlush()
    return text ? appendDeltaToMessages(current, text) : current
  }

  function appendMessage(role: 'user' | 'assistant' | 'system', content: string) {
    setMessages((current) => [...appendPendingDelta(current), { role, content }])
  }

  function appendDelta(text: string) {
    pendingDeltaRef.current += text
    if (deltaFrameRef.current == null) deltaFrameRef.current = requestAnimationFrame(flushDelta)
  }

  function resizeInput(textarea = inputRef.current) {
    if (!textarea) return
    textarea.style.height = 'auto'
    textarea.style.height = `${Math.min(textarea.scrollHeight, 120)}px`
    textarea.style.overflowY = textarea.scrollHeight > 120 ? 'auto' : 'hidden'
  }

  function focusInput() {
    requestAnimationFrame(() => {
      messagesRef.current?.scrollTo({ top: messagesRef.current.scrollHeight })
      inputRef.current?.focus()
    })
  }

  async function sendPrompt(message: string, chatId?: string) {
    const trimmed = message.trim()
    if (!trimmed || busy) return
    appendMessage('user', trimmed)
    setInput('')
    await sendMessage(trimmed, chatId)
  }

  async function openChat(view: CommandView) {
    pendingDeltaRef.current = ''
    cancelDeltaFlush()
    setMessages(view.messages || [])
    setInput('')
    focusInput()
    if (view.initialPrompt) await sendPrompt(view.initialPrompt, view.chatId)
    focusInput()
  }

  function handleEvent(event: AiChatEvent, activeChatId?: string) {
    if (event.chatId && event.chatId !== activeChatId) return
    if (event.type === 'start') setBusy(true)
    if (event.type === 'done' || event.type === 'error' || event.type === 'aborted') setBusy(false)
    if (event.type === 'delta' && event.text) appendDelta(event.text)
    if (event.type === 'tool_start' && event.name) appendMessage('system', `Using ${event.name}…`)
    if (event.type === 'error' && event.message) appendMessage('system', event.message)
  }

  useLayoutEffect(() => {
    resizeInput()
  }, [input])

  useEffect(() => () => cancelDeltaFlush(), [])

  return { messages, setMessages, input, setInput, busy, setBusy, messagesRef, inputRef, appendMessage, appendDelta, resizeInput, focusInput, sendPrompt, openChat, handleEvent }
}

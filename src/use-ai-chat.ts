import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { CommandView } from './model'

export type AiLimitState = { kind?: string; title: string; message: string; actionTitle?: string; dashboardUrl?: string }
export type AiChatEvent = { type: string; text?: string; message?: string; name?: string; chatId?: string; label?: string; data?: unknown }

function limitStateFromEvent(event: AiChatEvent): AiLimitState | null {
  const data = event.data as AiLimitState | undefined
  if (!data || typeof data !== 'object' || !data.title || !data.message) return null
  return data
}

export function useAiChat(sendMessage: (message: string, chatId?: string) => Promise<void>, resetChat: (chatId?: string) => Promise<void>) {
  const messagesRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const [messages, setMessages] = useState<NonNullable<CommandView['messages']>>([])
  const [input, setInput] = useState('')
  const [busy, setBusy] = useState(false)
  const [limit, setLimit] = useState<AiLimitState | null>(null)
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
    setLimit(null)
    appendMessage('user', trimmed)
    setInput('')
    try {
      await sendMessage(trimmed, chatId)
    } catch (error) {
      appendMessage('system', error instanceof Error ? error.message : String(error))
      setBusy(false)
    }
  }

  async function openChat(view: CommandView) {
    pendingDeltaRef.current = ''
    cancelDeltaFlush()
    setMessages(view.messages || [])
    setLimit(null)
    setInput('')
    focusInput()
    if (view.initialPrompt) await sendPrompt(view.initialPrompt, view.chatId)
    focusInput()
  }

  function handleEvent(event: AiChatEvent, activeChatId?: string) {
    if (event.chatId && event.chatId !== activeChatId) return
    if (event.type === 'start') {
      setLimit(null)
      setBusy(true)
    }
    if (event.type === 'done' || event.type === 'error' || event.type === 'aborted') setBusy(false)
    if (event.type === 'delta' && event.text) appendDelta(event.text)
    if (event.type === 'tool_start' && event.name) appendMessage('system', `Using ${event.name}…`)
    if (event.type === 'error') {
      const nextLimit = limitStateFromEvent(event)
      if (nextLimit) setLimit(nextLimit)
      else if (event.message) appendMessage('system', event.message)
    }
  }

  useLayoutEffect(() => {
    resizeInput()
  }, [input])

  useEffect(() => () => cancelDeltaFlush(), [])

  return { messages, setMessages, input, setInput, busy, setBusy, limit, setLimit, messagesRef, inputRef, appendMessage, appendDelta, resizeInput, focusInput, sendPrompt, openChat, handleEvent }
}

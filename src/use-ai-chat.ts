import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import {
  markDebugPerformance,
  measureDebugPerformance,
  measureDebugPerformanceSync,
  recordPerformanceTrace,
} from './debug-performance';
import { createRendererPerformanceTrace } from './performance-trace';
import type { CommandAction, CommandView } from './model';

export type AiLimitState = {
  kind?: string;
  title: string;
  message: string;
  actionTitle?: string;
  dashboardUrl?: string;
  action?: CommandAction;
};
export type AiChatEvent = {
  type: string;
  text?: string;
  message?: string;
  name?: string;
  chatId?: string;
  label?: string;
  data?: unknown;
  traceId?: string;
};

function isSafeLimitAction(action: unknown): action is CommandAction {
  if (!action || typeof action !== 'object') return false;
  const candidate = action as { type?: unknown; title?: unknown };
  if (typeof candidate.type !== 'string' || typeof candidate.title !== 'string')
    return false;
  return Object.values(action).every((value) => typeof value !== 'function');
}

export function aiChatEventMatchesActiveChat(
  event: AiChatEvent,
  activeChatId?: string,
) {
  return !event.chatId || event.chatId === activeChatId;
}

function limitStateFromEvent(event: AiChatEvent): AiLimitState | null {
  const data = event.data as AiLimitState | undefined;
  if (
    !data ||
    typeof data !== 'object' ||
    typeof data.title !== 'string' ||
    typeof data.message !== 'string'
  )
    return null;
  if (data.action && !isSafeLimitAction(data.action))
    return { ...data, action: undefined };
  return data;
}

export function useAiChat(
  sendMessage: (
    message: string,
    chatId?: string,
    traceId?: string,
  ) => Promise<void>,
  resetChat: (chatId?: string) => Promise<void>,
) {
  const messagesRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const [messages, setMessages] = useState<
    NonNullable<CommandView['messages']>
  >([]);
  const [input, setInput] = useState('');
  const [busy, setBusy] = useState(false);
  const [limit, setLimit] = useState<AiLimitState | null>(null);
  const [creditNotice, setCreditNotice] = useState<string | null>(null);
  const pendingDeltaRef = useRef('');
  const deltaFrameRef = useRef<number | null>(null);
  const activeTraceIdRef = useRef<string | undefined>(undefined);
  const activeTraceStartedAtRef = useRef<number | undefined>(undefined);
  const firstPaintRecordedRef = useRef(false);

  function finishActiveAiTrace(
    operation: string,
    status: 'error' | 'cancelled' | 'ok',
    traceId?: string,
  ) {
    if (
      traceId &&
      activeTraceIdRef.current &&
      traceId !== activeTraceIdRef.current
    )
      return;
    const startedAt = activeTraceStartedAtRef.current;
    const activeTraceId = traceId || activeTraceIdRef.current;
    if (activeTraceId && startedAt !== undefined)
      recordPerformanceTrace(activeTraceId, operation, startedAt, status);
    activeTraceIdRef.current = undefined;
    activeTraceStartedAtRef.current = undefined;
    firstPaintRecordedRef.current = false;
  }

  function appendDeltaToMessages(
    current: NonNullable<CommandView['messages']>,
    text: string,
  ) {
    const last = current[current.length - 1];
    if (last?.role === 'assistant')
      return [
        ...current.slice(0, -1),
        { ...last, content: `${last.content}${text}` },
      ];
    return [...current, { role: 'assistant' as const, content: text }];
  }

  function cancelDeltaFlush() {
    if (deltaFrameRef.current == null) return;
    cancelAnimationFrame(deltaFrameRef.current);
    deltaFrameRef.current = null;
  }

  function flushDelta() {
    deltaFrameRef.current = null;
    const text = pendingDeltaRef.current;
    pendingDeltaRef.current = '';
    if (text) {
      if (
        activeTraceIdRef.current &&
        activeTraceStartedAtRef.current !== undefined &&
        !firstPaintRecordedRef.current
      ) {
        const traceId = activeTraceIdRef.current;
        const startedAt = activeTraceStartedAtRef.current;
        requestAnimationFrame(() => {
          recordPerformanceTrace(traceId, 'ai.first-painted-delta', startedAt, 'ok', {
            deltaLength: text.length,
          });
        });
        firstPaintRecordedRef.current = true;
      }
      markDebugPerformance('ai.delta.flush', { textLength: text.length });
      setMessages((current) =>
        measureDebugPerformanceSync(
          'ai.delta.apply',
          { textLength: text.length, messageCount: current.length },
          () => appendDeltaToMessages(current, text),
        ),
      );
    }
  }

  function appendPendingDelta(current: NonNullable<CommandView['messages']>) {
    const text = pendingDeltaRef.current;
    pendingDeltaRef.current = '';
    cancelDeltaFlush();
    return text ? appendDeltaToMessages(current, text) : current;
  }

  function appendMessage(
    role: 'user' | 'assistant' | 'system',
    content: string,
  ) {
    markDebugPerformance('ai.message.append', {
      role,
      contentLength: content.length,
    });
    setMessages((current) => [
      ...appendPendingDelta(current),
      { role, content },
    ]);
  }

  function appendDelta(text: string) {
    pendingDeltaRef.current += text;
    markDebugPerformance('ai.delta.queued', {
      textLength: text.length,
      pendingLength: pendingDeltaRef.current.length,
    });
    if (deltaFrameRef.current == null)
      deltaFrameRef.current = requestAnimationFrame(flushDelta);
  }

  function resizeInput(textarea = inputRef.current) {
    measureDebugPerformanceSync(
      'ai.input.resize',
      { inputLength: textarea?.value.length || 0 },
      () => {
        if (!textarea) return;
        textarea.style.height = 'auto';
        textarea.style.height = `${Math.min(textarea.scrollHeight, 120)}px`;
        textarea.style.overflowY =
          textarea.scrollHeight > 120 ? 'auto' : 'hidden';
      },
    );
  }

  function focusInput() {
    requestAnimationFrame(() => {
      messagesRef.current?.scrollTo({ top: messagesRef.current.scrollHeight });
      inputRef.current?.focus();
    });
  }

  async function sendPrompt(message: string, chatId?: string) {
    const trimmed = message.trim();
    if (!trimmed || busy) return;
    setLimit(null);
    setCreditNotice(null);
    appendMessage('user', trimmed);
    setInput('');
    const trace = createRendererPerformanceTrace();
    activeTraceIdRef.current = trace.traceId;
    activeTraceStartedAtRef.current = trace.startedAt;
    firstPaintRecordedRef.current = false;
    try {
      await measureDebugPerformance(
        'ai.send-message',
        {
          chatId,
          messageLength: trimmed.length,
          traceId: trace.traceId,
          alwaysLog: true,
        },
        () => sendMessage(trimmed, chatId, trace.traceId),
      );
    } catch (error) {
      finishActiveAiTrace('ai.send.error', 'error');
      appendMessage(
        'system',
        error instanceof Error ? error.message : String(error),
      );
      setBusy(false);
    }
  }

  async function openChat(view: CommandView) {
    await measureDebugPerformance(
      'ai.chat-state.open',
      {
        chatId: view.chatId,
        messageCount: view.messages?.length || 0,
        hasInitialPrompt: Boolean(view.initialPrompt),
      },
      async () => {
        pendingDeltaRef.current = '';
        cancelDeltaFlush();
        setMessages(view.messages || []);
        setLimit(null);
        setCreditNotice(null);
        setInput('');
        focusInput();
        if (view.initialPrompt)
          await sendPrompt(view.initialPrompt, view.chatId);
        focusInput();
      },
    );
  }

  function handleEvent(event: AiChatEvent, activeChatId?: string) {
    if (!aiChatEventMatchesActiveChat(event, activeChatId)) return;
    if (
      event.traceId &&
      activeTraceIdRef.current &&
      event.traceId !== activeTraceIdRef.current
    )
      return;
    if (event.type === 'start') {
      setLimit(null);
      setCreditNotice(null);
      setBusy(true);
    }
    if (
      event.type === 'done' ||
      event.type === 'error' ||
      event.type === 'aborted'
    )
      setBusy(false);
    if (event.type === 'done')
      finishActiveAiTrace('ai.done', 'ok', event.traceId);
    if (event.type === 'error')
      finishActiveAiTrace('ai.error', 'error', event.traceId);
    if (event.type === 'aborted')
      finishActiveAiTrace('ai.aborted', 'cancelled', event.traceId);
    if (event.type === 'delta' && event.text) appendDelta(event.text);
    if (event.type === 'tool_start' && event.name)
      appendMessage('system', `Using ${event.name}…`);
    if (event.type === 'credit_warning' && event.message)
      setCreditNotice(event.message);
    if (event.type === 'error') {
      const nextLimit = limitStateFromEvent(event);
      if (nextLimit) setLimit(nextLimit);
      else if (event.message) appendMessage('system', event.message);
    }
  }

  useLayoutEffect(() => {
    resizeInput();
  }, [input]);

  useEffect(
    () => () => {
      cancelDeltaFlush();
      activeTraceIdRef.current = undefined;
      activeTraceStartedAtRef.current = undefined;
      firstPaintRecordedRef.current = false;
    },
    [],
  );

  return {
    messages,
    setMessages,
    input,
    setInput,
    busy,
    setBusy,
    limit,
    setLimit,
    creditNotice,
    messagesRef,
    inputRef,
    appendMessage,
    appendDelta,
    resizeInput,
    focusInput,
    sendPrompt,
    openChat,
    handleEvent,
  };
}

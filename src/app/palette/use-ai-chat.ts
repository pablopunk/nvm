import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import {
  AI_CHAT_IMAGE_LIMIT,
  AI_CHAT_IMAGE_MAX_BYTES,
  AI_CHAT_IMAGE_MIME_TYPES,
  AI_CHAT_IMAGE_TOTAL_MAX_BYTES,
  type AiChatImageInput,
  type AiChatMessageImage,
  canSendAiChatMessage,
} from '../shared/ai-chat-images';
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

export type AiChatAttachment = AiChatImageInput & {
  id: string;
  previewUrl: string;
  byteLength: number;
};

function readImageFile(file: File) {
  const mimeType = file.type.toLowerCase();
  if (!AI_CHAT_IMAGE_MIME_TYPES.includes(mimeType as never))
    return Promise.reject(new Error('Paste a PNG, JPEG, WebP, or GIF image'));
  if (!file.size || file.size > AI_CHAT_IMAGE_MAX_BYTES)
    return Promise.reject(new Error('Each image must be 8 MB or smaller'));
  return new Promise<AiChatAttachment>((resolve, reject) => {
    const reader = new FileReader();
    const previewUrl = URL.createObjectURL(file);
    function rejectRead() {
      URL.revokeObjectURL(previewUrl);
      reject(new Error('Could not read the pasted image'));
    }
    reader.onerror = rejectRead;
    reader.onabort = rejectRead;
    reader.onload = () => {
      const dataUrl = String(reader.result || '');
      const separator = dataUrl.indexOf(',');
      if (separator < 0) {
        URL.revokeObjectURL(previewUrl);
        return reject(new Error('Could not read the pasted image'));
      }
      resolve({
        id: crypto.randomUUID(),
        previewUrl,
        data: dataUrl.slice(separator + 1),
        mimeType,
        byteLength: file.size,
        ...(file.name ? { name: file.name } : {}),
      });
    };
    reader.readAsDataURL(file);
  });
}

function isSafeLimitAction(action: unknown): action is CommandAction {
  if (!action || typeof action !== 'object') return false;
  const candidate = action as {
    type?: unknown;
    title?: unknown;
    executionId?: unknown;
  };
  if (
    typeof candidate.type !== 'string' ||
    typeof candidate.title !== 'string' ||
    typeof candidate.executionId !== 'string'
  )
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

export function userMessageFromEvent(event: AiChatEvent) {
  const message = (event.data as { message?: unknown } | undefined)?.message;
  if (!message || typeof message !== 'object') return null;
  const candidate = message as {
    role?: unknown;
    content?: unknown;
    images?: unknown;
  };
  if (candidate.role !== 'user' || typeof candidate.content !== 'string')
    return null;
  const images = Array.isArray(candidate.images)
    ? candidate.images.flatMap((image) => {
        if (!image || typeof image !== 'object') return [];
        const typedImage = image as { url?: unknown; alt?: unknown };
        if (typeof typedImage.url !== 'string') return [];
        return [
          {
            url: typedImage.url,
            ...(typeof typedImage.alt === 'string'
              ? { alt: typedImage.alt }
              : {}),
          },
        ];
      })
    : [];
  return {
    role: 'user' as const,
    content: candidate.content,
    ...(images.length ? { images } : {}),
  };
}

function latestUserMessageIndex(
  messages: NonNullable<CommandView['messages']>,
) {
  for (let index = messages.length - 1; index >= 0; index -= 1)
    if (messages[index].role === 'user') return index;
  return -1;
}

export function useAiChat(
  sendMessage: (
    message: string,
    chatId?: string,
    traceId?: string,
    images?: AiChatImageInput[],
  ) => Promise<void>,
  resetChat: (chatId?: string) => Promise<void>,
) {
  const messagesRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const [messages, setMessages] = useState<
    NonNullable<CommandView['messages']>
  >([]);
  const [input, setInput] = useState('');
  const [attachments, setAttachments] = useState<AiChatAttachment[]>([]);
  const attachmentsRef = useRef<AiChatAttachment[]>([]);
  const pendingAttachmentCountRef = useRef(0);
  const attachmentGenerationRef = useRef(0);
  const attachmentPreviewUrlsRef = useRef(new Set<string>());
  const [attaching, setAttaching] = useState(false);
  const [attachmentError, setAttachmentError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);
  const busyByChatIdRef = useRef(new Map<string, boolean>());
  const [limit, setLimit] = useState<AiLimitState | null>(null);
  const [creditNotice, setCreditNotice] = useState<string | null>(null);
  const pendingDeltaRef = useRef('');
  const deltaFrameRef = useRef<number | null>(null);
  const activeTraceIdRef = useRef<string | undefined>(undefined);
  const optimisticTraceIdRef = useRef<string | undefined>(undefined);
  const activeTraceStartedAtRef = useRef<number | undefined>(undefined);
  const firstPaintRecordedRef = useRef(false);
  const openChatIdRef = useRef<string | undefined>(undefined);

  function updateBusy(nextBusy: boolean) {
    busyRef.current = nextBusy;
    setBusy(nextBusy);
  }

  function chatStateKey(chatId?: string) {
    return chatId || 'default';
  }

  function updateChatBusy(chatId: string | undefined, nextBusy: boolean) {
    const key = chatStateKey(chatId);
    busyByChatIdRef.current.set(key, nextBusy);
    if (key === chatStateKey(openChatIdRef.current)) updateBusy(nextBusy);
  }

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
    return boundedMessages([
      ...current,
      { role: 'assistant' as const, content: text },
    ]);
  }

  function releaseMessagePreviews(
    released: NonNullable<CommandView['messages']>,
  ) {
    for (const message of released)
      for (const image of message.images || [])
        if (attachmentPreviewUrlsRef.current.delete(image.url))
          URL.revokeObjectURL(image.url);
  }

  function boundedMessages(next: NonNullable<CommandView['messages']>) {
    const released = next.slice(0, -100);
    if (released.length) releaseMessagePreviews(released);
    return next.slice(-100);
  }

  function replaceMessages(next: NonNullable<CommandView['messages']>) {
    if (!next.length) {
      attachmentGenerationRef.current += 1;
      pendingAttachmentCountRef.current = 0;
      setAttaching(false);
      updateAttachments([]);
      for (const url of attachmentPreviewUrlsRef.current)
        URL.revokeObjectURL(url);
      attachmentPreviewUrlsRef.current.clear();
      setAttachmentError(null);
    }
    setMessages((current) => {
      releaseMessagePreviews(current);
      return boundedMessages(next);
    });
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
          recordPerformanceTrace(
            traceId,
            'ai.first-painted-delta',
            startedAt,
            'ok',
            {
              deltaLength: text.length,
            },
          );
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
    images?: AiChatMessageImage[],
  ) {
    markDebugPerformance('ai.message.append', {
      role,
      contentLength: content.length,
    });
    setMessages((current) =>
      boundedMessages([
        ...appendPendingDelta(current),
        { role, content, ...(images?.length ? { images } : {}) },
      ]),
    );
  }

  function applyUserMessageEvent(event: AiChatEvent) {
    const message = userMessageFromEvent(event);
    if (!message) return;
    const replacesOptimisticMessage =
      Boolean(event.traceId) && event.traceId === optimisticTraceIdRef.current;
    if (replacesOptimisticMessage) optimisticTraceIdRef.current = undefined;
    setMessages((current) => {
      if (!replacesOptimisticMessage)
        return boundedMessages([...appendPendingDelta(current), message]);
      const index = latestUserMessageIndex(current);
      if (index < 0) return boundedMessages([...current, message]);
      releaseMessagePreviews([current[index]]);
      return [...current.slice(0, index), message, ...current.slice(index + 1)];
    });
  }

  function releaseLatestOptimisticMessage() {
    setMessages((current) => {
      const index = latestUserMessageIndex(current);
      if (index < 0) return current;
      const optimistic = current[index];
      releaseMessagePreviews([optimistic]);
      return [
        ...current.slice(0, index),
        {
          role: optimistic.role,
          content: optimistic.content || '[Image attachment failed]',
        },
        ...current.slice(index + 1),
      ];
    });
  }

  function updateAttachments(next: AiChatAttachment[]) {
    attachmentsRef.current = next;
    setAttachments(next);
  }

  async function attachImageFiles(files: File[]) {
    const imageFiles = files.filter((file) => file.type.startsWith('image/'));
    if (!imageFiles.length) return false;
    const available = Math.max(
      0,
      AI_CHAT_IMAGE_LIMIT -
        attachmentsRef.current.length -
        pendingAttachmentCountRef.current,
    );
    const acceptedFiles = imageFiles.slice(0, available);
    setAttachmentError(
      imageFiles.length > available
        ? `Attach no more than ${AI_CHAT_IMAGE_LIMIT} images`
        : null,
    );
    if (!acceptedFiles.length) return true;
    const generation = attachmentGenerationRef.current;
    pendingAttachmentCountRef.current += acceptedFiles.length;
    setAttaching(true);
    const results = await Promise.allSettled(acceptedFiles.map(readImageFile));
    if (generation !== attachmentGenerationRef.current) {
      for (const result of results)
        if (result.status === 'fulfilled')
          URL.revokeObjectURL(result.value.previewUrl);
      return true;
    }
    const readable = results.flatMap((result) =>
      result.status === 'fulfilled' ? [result.value] : [],
    );
    const next: AiChatAttachment[] = [];
    let totalBytes = attachmentsRef.current.reduce(
      (total, attachment) => total + attachment.byteLength,
      0,
    );
    for (const attachment of readable) {
      if (totalBytes + attachment.byteLength > AI_CHAT_IMAGE_TOTAL_MAX_BYTES) {
        URL.revokeObjectURL(attachment.previewUrl);
        setAttachmentError('AI chat images must total 24 MB or less');
        continue;
      }
      totalBytes += attachment.byteLength;
      next.push(attachment);
    }
    const failure = results.find(
      (result): result is PromiseRejectedResult => result.status === 'rejected',
    );
    if (failure)
      setAttachmentError(
        failure.reason instanceof Error
          ? failure.reason.message
          : String(failure.reason),
      );
    if (next.length) {
      for (const attachment of next)
        attachmentPreviewUrlsRef.current.add(attachment.previewUrl);
      updateAttachments([...attachmentsRef.current, ...next]);
    }
    pendingAttachmentCountRef.current -= acceptedFiles.length;
    setAttaching(pendingAttachmentCountRef.current > 0);
    return true;
  }

  function removeAttachment(id: string) {
    const removed = attachmentsRef.current.find(
      (attachment) => attachment.id === id,
    );
    if (removed) {
      URL.revokeObjectURL(removed.previewUrl);
      attachmentPreviewUrlsRef.current.delete(removed.previewUrl);
    }
    updateAttachments(
      attachmentsRef.current.filter((attachment) => attachment.id !== id),
    );
    setAttachmentError(null);
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
    const outgoingAttachments = attachmentsRef.current;
    const targetChatId = chatId || openChatIdRef.current;
    if (
      !canSendAiChatMessage(trimmed, outgoingAttachments.length) ||
      pendingAttachmentCountRef.current > 0 ||
      busyByChatIdRef.current.get(chatStateKey(targetChatId)) ||
      (!targetChatId && busyRef.current)
    )
      return;
    updateChatBusy(targetChatId, true);
    setLimit(null);
    setCreditNotice(null);
    appendMessage(
      'user',
      trimmed,
      outgoingAttachments.map((attachment) => ({
        url: attachment.previewUrl,
        alt: attachment.name || 'Pasted image',
      })),
    );
    setInput('');
    updateAttachments([]);
    setAttachmentError(null);
    const trace = createRendererPerformanceTrace();
    activeTraceIdRef.current = trace.traceId;
    optimisticTraceIdRef.current = trace.traceId;
    activeTraceStartedAtRef.current = trace.startedAt;
    firstPaintRecordedRef.current = false;
    try {
      await measureDebugPerformance(
        'ai.send-message',
        {
          chatId,
          messageLength: trimmed.length,
          imageCount: outgoingAttachments.length,
          traceId: trace.traceId,
          alwaysLog: true,
        },
        () =>
          sendMessage(
            trimmed,
            chatId,
            trace.traceId,
            outgoingAttachments.map(({ data, mimeType, name }) => ({
              data,
              mimeType,
              ...(name ? { name } : {}),
            })),
          ),
      );
    } catch (error) {
      if (optimisticTraceIdRef.current === trace.traceId) {
        releaseLatestOptimisticMessage();
        optimisticTraceIdRef.current = undefined;
      }
      finishActiveAiTrace('ai.send.error', 'error', trace.traceId);
      if (chatStateKey(targetChatId) === chatStateKey(openChatIdRef.current))
        appendMessage(
          'system',
          error instanceof Error ? error.message : String(error),
        );
      updateChatBusy(targetChatId, false);
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
        openChatIdRef.current = view.chatId;
        updateBusy(
          busyByChatIdRef.current.get(chatStateKey(view.chatId)) || false,
        );
        pendingDeltaRef.current = '';
        cancelDeltaFlush();
        setMessages(view.messages || []);
        setLimit(null);
        setCreditNotice(null);
        setInput('');
        optimisticTraceIdRef.current = undefined;
        attachmentGenerationRef.current += 1;
        pendingAttachmentCountRef.current = 0;
        setAttaching(false);
        for (const url of attachmentPreviewUrlsRef.current)
          URL.revokeObjectURL(url);
        attachmentPreviewUrlsRef.current.clear();
        updateAttachments([]);
        setAttachmentError(null);
        focusInput();
        if (view.initialPrompt)
          await sendPrompt(view.initialPrompt, view.chatId);
        focusInput();
      },
    );
  }

  function handleEvent(event: AiChatEvent, activeChatId?: string) {
    const eventChatId = event.chatId || activeChatId;
    if (event.type === 'start') updateChatBusy(eventChatId, true);
    if (
      event.type === 'done' ||
      event.type === 'error' ||
      event.type === 'aborted'
    )
      updateChatBusy(eventChatId, false);
    if (!aiChatEventMatchesActiveChat(event, activeChatId)) return;
    if (event.type === 'user_message') {
      applyUserMessageEvent(event);
      return;
    }
    if (
      event.traceId &&
      activeTraceIdRef.current &&
      event.traceId !== activeTraceIdRef.current
    )
      return;
    if (event.type === 'start') {
      setLimit(null);
      setCreditNotice(null);
    }
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
      attachmentGenerationRef.current += 1;
      for (const url of attachmentPreviewUrlsRef.current)
        URL.revokeObjectURL(url);
      attachmentPreviewUrlsRef.current.clear();
      cancelDeltaFlush();
      activeTraceIdRef.current = undefined;
      optimisticTraceIdRef.current = undefined;
      activeTraceStartedAtRef.current = undefined;
      firstPaintRecordedRef.current = false;
      busyByChatIdRef.current.clear();
    },
    [],
  );

  return {
    messages,
    setMessages: replaceMessages,
    input,
    setInput,
    attachments,
    attaching,
    attachmentError,
    attachImageFiles,
    removeAttachment,
    busy,
    setBusy: updateBusy,
    limit,
    setLimit,
    creditNotice,
    setCreditNotice,
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

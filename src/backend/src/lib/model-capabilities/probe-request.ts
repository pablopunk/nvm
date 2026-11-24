import { rewriteChatCompletionsBody, type RouteReasoning } from '../chat-completions-request';
import { getUpstreamConfig } from '../upstream';

const PROBE_TIMEOUT_MS = 60_000;
const PROBE_DETAIL_LIMIT = 200;

export type ProbeTarget = { provider: string; modelId: string };

export type ProbeRequest = {
  messages: unknown[];
  tools?: unknown[];
  thinkingLevel: string;
  reasoning: RouteReasoning;
  maxOutputTokens: number;
};

export type ProbeReply =
  | { ok: false; detail: string; temporary: boolean }
  | {
      ok: true;
      text: string;
      reasoningCharacters: number;
      reasoningTokens: number | null;
      promptTokens: number;
      completionTokens: number;
      toolCalls: Array<{ name: string; arguments: string }>;
      firstTokenMs: number | null;
    };

type StreamedReply = Extract<ProbeReply, { ok: true }>;

function emptyStreamedReply(): StreamedReply {
  return {
    ok: true,
    text: '',
    reasoningCharacters: 0,
    reasoningTokens: null,
    promptTokens: 0,
    completionTokens: 0,
    toolCalls: [],
    firstTokenMs: null,
  };
}

function readStreamEvent(reply: StreamedReply, event: any) {
  const usage = event?.usage;
  if (usage) {
    reply.promptTokens = Number(usage.prompt_tokens) || 0;
    reply.completionTokens = Number(usage.completion_tokens) || 0;
    const reasoningTokens = usage.completion_tokens_details?.reasoning_tokens;
    reply.reasoningTokens = typeof reasoningTokens === 'number' ? reasoningTokens : reply.reasoningTokens;
  }
  for (const choice of event?.choices ?? []) {
    const delta = choice?.delta ?? {};
    if (typeof delta.content === 'string') reply.text += delta.content;
    for (const key of ['reasoning', 'reasoning_content'] as const)
      if (typeof delta[key] === 'string') reply.reasoningCharacters += delta[key].length;
    for (const toolCall of delta.tool_calls ?? []) {
      const index = typeof toolCall?.index === 'number' ? toolCall.index : reply.toolCalls.length;
      const current = reply.toolCalls[index] ?? { name: '', arguments: '' };
      current.name += toolCall?.function?.name ?? '';
      current.arguments += toolCall?.function?.arguments ?? '';
      reply.toolCalls[index] = current;
    }
  }
}

function hasOutput(reply: StreamedReply): boolean {
  return reply.text.length > 0 || reply.reasoningCharacters > 0 || reply.toolCalls.length > 0;
}

function readServerSentEventLine(reply: StreamedReply, line: string) {
  const trimmed = line.trim();
  if (!trimmed.startsWith('data:')) return;
  const payload = trimmed.slice(5).trim();
  if (!payload || payload === '[DONE]') return;
  try {
    readStreamEvent(reply, JSON.parse(payload));
  } catch {
    // A malformed event cannot prove a capability; the reply is judged on what parsed.
  }
}

async function readServerSentEvents(body: ReadableStream<Uint8Array>, startedAt: number): Promise<StreamedReply> {
  const reply = emptyStreamedReply();
  const decoder = new TextDecoder();
  let pending = '';
  const reader = body.getReader();
  for (let chunk = await reader.read(); !chunk.done; chunk = await reader.read()) {
    pending += decoder.decode(chunk.value, { stream: true });
    const lines = pending.split('\n');
    pending = lines.pop() ?? '';
    for (const line of lines) readServerSentEventLine(reply, line);
    if (reply.firstTokenMs === null && hasOutput(reply)) reply.firstTokenMs = Date.now() - startedAt;
  }
  readServerSentEventLine(reply, pending + decoder.decode());
  return reply;
}

function errorDetail(status: number, body: string): string {
  let message = body;
  try {
    const parsed = JSON.parse(body)?.error?.message;
    if (typeof parsed === 'string') message = parsed;
  } catch {
    // Not a JSON error envelope; keep the raw body.
  }
  return `HTTP ${status}: ${message}`.slice(0, PROBE_DETAIL_LIMIT);
}

function isTemporaryStatus(status: number): boolean {
  return status === 408 || status === 429 || status >= 500;
}

export async function sendProbe(target: ProbeTarget, request: ProbeRequest): Promise<ProbeReply> {
  const { baseUrl, apiKey } = getUpstreamConfig(target.provider);
  const clientBody = JSON.stringify({
    model: target.modelId,
    messages: request.messages,
    ...(request.tools ? { tools: request.tools } : {}),
    stream: true,
  });
  const body = rewriteChatCompletionsBody(
    clientBody,
    { activeModelId: target.modelId, provider: target.provider, thinkingLevel: request.thinkingLevel },
    request.maxOutputTokens,
    request.reasoning,
  );
  const startedAt = Date.now();
  try {
    const response = await fetch(`${baseUrl}/chat/completions`, {
      method: 'POST',
      headers: { authorization: `Bearer ${apiKey}`, 'content-type': 'application/json' },
      body,
      signal: AbortSignal.timeout(PROBE_TIMEOUT_MS),
    });
    if (!response.ok)
      return {
        ok: false,
        detail: errorDetail(response.status, await response.text()),
        temporary: isTemporaryStatus(response.status),
      };
    if (!response.body) return { ok: false, detail: 'empty response body', temporary: true };
    return await readServerSentEvents(response.body, startedAt);
  } catch (error) {
    return {
      ok: false,
      detail: (error instanceof Error ? error.message : String(error)).slice(0, PROBE_DETAIL_LIMIT),
      temporary: true,
    };
  }
}

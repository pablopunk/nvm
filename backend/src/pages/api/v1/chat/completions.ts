import type { APIRoute } from 'astro';
import { randomUUID } from 'node:crypto';
import { streamText, generateText, type ModelMessage } from 'ai';
import { modelFor } from '../../../../lib/provider';
import { getUserFromBearer } from '../../../../lib/tokens';
import { getBalances } from '../../../../lib/users';
import { getActiveModelId, getFreeModelId, getActiveProvider } from '../../../../lib/settings';
import {
  lookupModelCost,
  computeUsdCost,
  usdToCredits,
  usdToMicrocents,
} from '../../../../lib/cost';
import { db } from '../../../../db/client';
import { creditLedger, usage } from '../../../../db/schema';

export const config = { maxDuration: 300 };
const SOFT_CAP_MS = 240_000;

type OpenAIMessage = { role: 'system' | 'user' | 'assistant' | 'tool'; content: string };
type OpenAIRequest = {
  model?: string;
  messages: OpenAIMessage[];
  stream?: boolean;
  temperature?: number;
  max_tokens?: number;
};

function toModelMessages(messages: OpenAIMessage[]): ModelMessage[] {
  return messages
    .filter((m) => m.role !== 'tool')
    .map((m) => ({ role: m.role, content: m.content }) as ModelMessage);
}

async function recordUsage(args: {
  userId: string;
  provider: string;
  model: string;
  kind: 'free' | 'paid';
  inputTokens: number;
  outputTokens: number;
  costCredits: number;
  upstreamCostMicrocents: number;
  requestId: string;
}) {
  await db.transaction(async (tx) => {
    await tx.insert(creditLedger).values({
      userId: args.userId,
      delta: -args.costCredits,
      kind: args.kind,
      reason: 'ai_usage',
      refId: args.requestId,
    });
    await tx.insert(usage).values({
      userId: args.userId,
      model: args.model,
      provider: args.provider,
      inputTokens: args.inputTokens,
      outputTokens: args.outputTokens,
      costCredits: args.costCredits,
      upstreamCostMicrocents: args.upstreamCostMicrocents,
      requestId: args.requestId,
    });
  });
}

export const POST: APIRoute = async ({ request }) => {
  const user = await getUserFromBearer(request.headers.get('authorization'));
  if (!user) return new Response('Unauthorized', { status: 401 });

  const balances = await getBalances(user.id);
  if (balances.total <= 0) {
    return Response.json(
      { error: { type: 'insufficient_credits', message: 'No credits remaining' } },
      { status: 402 },
    );
  }

  const body = (await request.json().catch(() => null)) as OpenAIRequest | null;
  if (!body?.messages?.length) return new Response('Bad request', { status: 400 });

  const usingFreeTier = balances.free > 0;
  const kind: 'free' | 'paid' = usingFreeTier ? 'free' : 'paid';
  const provider = await getActiveProvider();
  const activeModelId = usingFreeTier ? await getFreeModelId() : await getActiveModelId();

  const costRow = await lookupModelCost(provider, activeModelId);
  if (!costRow) {
    console.error('[chat/completions] pricing_unavailable', { provider, model: activeModelId });
    return Response.json(
      { error: { type: 'pricing_unavailable', message: 'No pricing configured for active model' } },
      { status: 503 },
    );
  }

  const model = modelFor(activeModelId, provider);
  const requestId = randomUUID();
  const created = Math.floor(Date.now() / 1000);
  const messages = toModelMessages(body.messages);

  const finalize = async (inputTokens: number, outputTokens: number) => {
    const costUsd = computeUsdCost(costRow, inputTokens, outputTokens);
    const creditsCharged = usdToCredits(costUsd);
    const upstreamCostMicrocents = usdToMicrocents(costUsd);
    await recordUsage({
      userId: user.id,
      provider,
      model: activeModelId,
      kind,
      inputTokens,
      outputTokens,
      costCredits: creditsCharged,
      upstreamCostMicrocents,
      requestId,
    });
  };

  if (body.stream === false) {
    const result = await generateText({ model, messages, temperature: body.temperature });
    await finalize(result.usage.inputTokens ?? 0, result.usage.outputTokens ?? 0);
    return Response.json({
      id: `chatcmpl-${requestId}`,
      object: 'chat.completion',
      created,
      model: activeModelId,
      choices: [{ index: 0, message: { role: 'assistant', content: result.text }, finish_reason: 'stop' }],
      usage: {
        prompt_tokens: result.usage.inputTokens ?? 0,
        completion_tokens: result.usage.outputTokens ?? 0,
        total_tokens: (result.usage.inputTokens ?? 0) + (result.usage.outputTokens ?? 0),
      },
    });
  }

  const abort = new AbortController();
  const softCapTimer = setTimeout(() => abort.abort(), SOFT_CAP_MS);

  const result = streamText({
    model,
    messages,
    temperature: body.temperature,
    abortSignal: abort.signal,
    onFinish: async ({ usage: u }) => {
      clearTimeout(softCapTimer);
      await finalize(u.inputTokens ?? 0, u.outputTokens ?? 0);
    },
  });

  const encoder = new TextEncoder();
  const stream = new ReadableStream({
    async start(controller) {
      const baseChunk = {
        id: `chatcmpl-${requestId}`,
        object: 'chat.completion.chunk',
        created,
        model: activeModelId,
      };
      const send = (payload: object) => controller.enqueue(encoder.encode(`data: ${JSON.stringify(payload)}\n\n`));
      try {
        send({ ...baseChunk, choices: [{ index: 0, delta: { role: 'assistant' }, finish_reason: null }] });
        for await (const delta of result.textStream) {
          send({ ...baseChunk, choices: [{ index: 0, delta: { content: delta }, finish_reason: null }] });
        }
        const finalUsage = await result.usage;
        const finishReason = abort.signal.aborted ? 'length' : 'stop';
        send({ ...baseChunk, choices: [{ index: 0, delta: {}, finish_reason: finishReason }] });
        send({
          ...baseChunk,
          choices: [],
          usage: {
            prompt_tokens: finalUsage.inputTokens ?? 0,
            completion_tokens: finalUsage.outputTokens ?? 0,
            total_tokens: (finalUsage.inputTokens ?? 0) + (finalUsage.outputTokens ?? 0),
          },
        });
        controller.enqueue(encoder.encode('data: [DONE]\n\n'));
      } catch (err) {
        send({ ...baseChunk, choices: [{ index: 0, delta: {}, finish_reason: 'error' }] });
        console.error('[chat/completions] stream error', err);
      } finally {
        clearTimeout(softCapTimer);
        controller.close();
      }
    },
  });

  return new Response(stream, {
    headers: {
      'Content-Type': 'text/event-stream; charset=utf-8',
      'Cache-Control': 'no-cache, no-transform',
      Connection: 'keep-alive',
    },
  });
};

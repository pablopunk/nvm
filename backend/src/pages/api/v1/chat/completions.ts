import type { APIRoute } from 'astro';
import { randomUUID } from 'node:crypto';
import { streamText, generateText, type ModelMessage } from 'ai';
import { modelFor } from '../../../../lib/provider';
import { getUserFromBearer } from '../../../../lib/tokens';
import { getBalance } from '../../../../lib/users';
import { resolveModel, computeCostCredits } from '../../../../lib/pricing';
import { db } from '../../../../db/client';
import { creditLedger, usage } from '../../../../db/schema';

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
  model: string;
  inputTokens: number;
  outputTokens: number;
  costCredits: number;
  requestId: string;
}) {
  await db.transaction(async (tx) => {
    await tx.insert(creditLedger).values({
      userId: args.userId,
      delta: -args.costCredits,
      reason: 'ai_usage',
      refId: args.requestId,
    });
    await tx.insert(usage).values({
      userId: args.userId,
      model: args.model,
      inputTokens: args.inputTokens,
      outputTokens: args.outputTokens,
      costCredits: args.costCredits,
      requestId: args.requestId,
    });
  });
}

export const POST: APIRoute = async ({ request }) => {
  const user = await getUserFromBearer(request.headers.get('authorization'));
  if (!user) return new Response('Unauthorized', { status: 401 });

  const balance = await getBalance(user.id);
  if (balance <= 0) {
    return Response.json(
      { error: { type: 'insufficient_credits', message: 'No credits remaining' } },
      { status: 402 },
    );
  }

  const body = (await request.json().catch(() => null)) as OpenAIRequest | null;
  if (!body?.messages?.length) return new Response('Bad request', { status: 400 });

  const pricing = resolveModel(body.model);
  const model = modelFor(pricing.realModel);
  const requestId = randomUUID();
  const created = Math.floor(Date.now() / 1000);
  const modelLabel = body.model ?? pricing.realModel;
  const messages = toModelMessages(body.messages);

  if (body.stream === false) {
    const result = await generateText({ model, messages, temperature: body.temperature });
    const inputTokens = result.usage.inputTokens ?? 0;
    const outputTokens = result.usage.outputTokens ?? 0;
    const cost = computeCostCredits(pricing, inputTokens, outputTokens);
    await recordUsage({ userId: user.id, model: pricing.realModel, inputTokens, outputTokens, costCredits: cost, requestId });
    return Response.json({
      id: `chatcmpl-${requestId}`,
      object: 'chat.completion',
      created,
      model: modelLabel,
      choices: [{ index: 0, message: { role: 'assistant', content: result.text }, finish_reason: 'stop' }],
      usage: { prompt_tokens: inputTokens, completion_tokens: outputTokens, total_tokens: inputTokens + outputTokens },
    });
  }

  const result = streamText({
    model,
    messages,
    temperature: body.temperature,
    onFinish: async ({ usage: u }) => {
      const inputTokens = u.inputTokens ?? 0;
      const outputTokens = u.outputTokens ?? 0;
      const cost = computeCostCredits(pricing, inputTokens, outputTokens);
      await recordUsage({ userId: user.id, model: pricing.realModel, inputTokens, outputTokens, costCredits: cost, requestId });
    },
  });

  const encoder = new TextEncoder();
  const stream = new ReadableStream({
    async start(controller) {
      const baseChunk = {
        id: `chatcmpl-${requestId}`,
        object: 'chat.completion.chunk',
        created,
        model: modelLabel,
      };
      const send = (payload: object) => controller.enqueue(encoder.encode(`data: ${JSON.stringify(payload)}\n\n`));
      try {
        send({ ...baseChunk, choices: [{ index: 0, delta: { role: 'assistant' }, finish_reason: null }] });
        for await (const delta of result.textStream) {
          send({ ...baseChunk, choices: [{ index: 0, delta: { content: delta }, finish_reason: null }] });
        }
        const finalUsage = await result.usage;
        send({ ...baseChunk, choices: [{ index: 0, delta: {}, finish_reason: 'stop' }] });
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

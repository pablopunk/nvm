import { randomUUID } from 'node:crypto';
import { db } from '../db/client';
import { creditLedger, usage } from '../db/schema';
import {
  getActiveModelId,
  getFreeModelId,
  getActiveProvider,
  ModelNotConfiguredError,
} from './settings';
import { getBalances } from './users';
import {
  lookupModelCost,
  computeUsdCost,
  usdToCredits,
  usdToMicrocents,
  type ModelCost,
} from './cost';
import { getUpstreamConfig, UpstreamConfigError } from './upstream';
import { extractPatFromHeaders, getUserFromHeaders, type PatHeaderName } from './tokens';

export type UsageTokens = { inputTokens: number; outputTokens: number };

export type ProxyConfig = {
  request: Request;
  authHeaderName: PatHeaderName;
  upstreamAuthHeaderName: 'authorization' | 'x-api-key' | 'x-goog-api-key';
  formatUpstreamAuthValue: (apiKey: string) => string;
  buildUpstreamUrl: (cfg: { upstreamBaseUrl: string; activeModelId: string }) => string;
  rewriteRequestBody?: (bodyText: string, activeModelId: string) => string;
  parseUsageFromJson: (json: any) => UsageTokens | null;
  parseUsageFromStreamChunk: (chunk: string, acc: StreamUsageAccumulator) => void;
};

export type StreamUsageAccumulator = {
  inputTokens: number;
  outputTokens: number;
  finalized: boolean;
};

type BillContext = {
  user: { id: string };
  provider: string;
  activeModelId: string;
  costRow: ModelCost;
  kind: 'free' | 'paid';
  requestId: string;
};

async function recordUsage(ctx: BillContext, tokens: UsageTokens) {
  if (tokens.outputTokens <= 0) return;
  const costUsd = computeUsdCost(ctx.costRow, tokens.inputTokens, tokens.outputTokens);
  const credits = usdToCredits(costUsd);
  const microcents = usdToMicrocents(costUsd);
  await db.transaction(async (tx) => {
    await tx.insert(creditLedger).values({
      userId: ctx.user.id,
      delta: -credits,
      kind: ctx.kind,
      reason: 'ai_usage',
      refId: ctx.requestId,
    });
    await tx.insert(usage).values({
      userId: ctx.user.id,
      model: ctx.activeModelId,
      provider: ctx.provider,
      inputTokens: tokens.inputTokens,
      outputTokens: tokens.outputTokens,
      costCredits: credits,
      upstreamCostMicrocents: microcents,
      requestId: ctx.requestId,
    });
  });
}

type ResolvedRouting = {
  user: { id: string };
  provider: string;
  activeModelId: string;
  costRow: ModelCost;
  kind: 'free' | 'paid';
  upstreamBaseUrl: string;
  upstreamApiKey: string;
};

async function resolveRouting(request: Request, headerName: PatHeaderName): Promise<Response | ResolvedRouting> {
  if (!extractPatFromHeaders(request, headerName)) {
    return new Response('Unauthorized', { status: 401 });
  }
  const user = await getUserFromHeaders(request, headerName);
  if (!user) return new Response('Unauthorized', { status: 401 });

  const balances = await getBalances(user.id);
  if (balances.total <= 0) {
    return Response.json(
      { error: { type: 'insufficient_credits', message: 'No credits remaining' } },
      { status: 402 },
    );
  }

  const kind: 'free' | 'paid' = balances.free > 0 ? 'free' : 'paid';
  const provider = await getActiveProvider();
  let activeModelId: string;
  try {
    activeModelId = kind === 'free' ? await getFreeModelId() : await getActiveModelId();
  } catch (err) {
    if (err instanceof ModelNotConfiguredError) {
      return Response.json(
        { error: { type: 'model_not_configured', message: 'No active model configured. Admin must set one.' } },
        { status: 503 },
      );
    }
    throw err;
  }

  const costRow = await lookupModelCost(provider, activeModelId);
  if (!costRow) {
    console.error('[proxy] pricing_unavailable', { provider, model: activeModelId });
    return Response.json(
      { error: { type: 'pricing_unavailable', message: 'No pricing configured for active model' } },
      { status: 503 },
    );
  }

  let upstream;
  try {
    upstream = getUpstreamConfig(provider);
  } catch (err) {
    if (err instanceof UpstreamConfigError) {
      return Response.json(
        { error: { type: 'model_not_configured', message: err.message } },
        { status: 503 },
      );
    }
    throw err;
  }

  return {
    user,
    provider,
    activeModelId,
    costRow,
    kind,
    upstreamBaseUrl: upstream.baseUrl,
    upstreamApiKey: upstream.apiKey,
  };
}

const HOP_BY_HOP = new Set([
  'connection',
  'keep-alive',
  'proxy-authenticate',
  'proxy-authorization',
  'te',
  'trailer',
  'transfer-encoding',
  'upgrade',
  'host',
  'content-length',
]);

function buildForwardHeaders(
  request: Request,
  desktopAuthHeader: PatHeaderName,
  upstreamAuthHeader: ProxyConfig['upstreamAuthHeaderName'],
  upstreamAuthValue: string,
): Headers {
  const out = new Headers();
  for (const [name, value] of request.headers) {
    const lower = name.toLowerCase();
    if (HOP_BY_HOP.has(lower)) continue;
    if (lower === desktopAuthHeader) continue;
    if (lower === upstreamAuthHeader) continue;
    out.set(name, value);
  }
  out.set(upstreamAuthHeader, upstreamAuthValue);
  return out;
}

function isStreamingContentType(contentType: string | null): boolean {
  if (!contentType) return false;
  return contentType.includes('text/event-stream') || contentType.includes('stream');
}

export async function proxyAndBill(cfg: ProxyConfig): Promise<Response> {
  const routing = await resolveRouting(cfg.request, cfg.authHeaderName);
  if (routing instanceof Response) return routing;

  const requestId = randomUUID();
  const upstreamUrl = cfg.buildUpstreamUrl({
    upstreamBaseUrl: routing.upstreamBaseUrl,
    activeModelId: routing.activeModelId,
  });

  let forwardBody: BodyInit | undefined;
  if (cfg.request.method !== 'GET' && cfg.request.method !== 'HEAD') {
    const text = await cfg.request.text();
    forwardBody = cfg.rewriteRequestBody ? cfg.rewriteRequestBody(text, routing.activeModelId) : text;
  }

  const headers = buildForwardHeaders(
    cfg.request,
    cfg.authHeaderName,
    cfg.upstreamAuthHeaderName,
    cfg.formatUpstreamAuthValue(routing.upstreamApiKey),
  );

  const upstreamResponse = await fetch(upstreamUrl, {
    method: cfg.request.method,
    headers,
    body: forwardBody,
  });

  const billCtx: BillContext = {
    user: routing.user,
    provider: routing.provider,
    activeModelId: routing.activeModelId,
    costRow: routing.costRow,
    kind: routing.kind,
    requestId,
  };

  const responseHeaders = stripHopByHop(upstreamResponse.headers);

  if (!upstreamResponse.ok) {
    return new Response(upstreamResponse.body, {
      status: upstreamResponse.status,
      statusText: upstreamResponse.statusText,
      headers: responseHeaders,
    });
  }

  if (isStreamingContentType(upstreamResponse.headers.get('content-type'))) {
    const transformed = teeStreamAndBill(upstreamResponse, cfg, billCtx);
    return new Response(transformed, {
      status: upstreamResponse.status,
      statusText: upstreamResponse.statusText,
      headers: responseHeaders,
    });
  }

  const buffered = await upstreamResponse.arrayBuffer();
  try {
    const json = JSON.parse(new TextDecoder().decode(buffered));
    const tokens = cfg.parseUsageFromJson(json);
    if (tokens) await recordUsage(billCtx, tokens);
  } catch (err) {
    console.warn('[proxy] failed to parse usage from non-stream body', err);
  }
  return new Response(buffered, {
    status: upstreamResponse.status,
    statusText: upstreamResponse.statusText,
    headers: responseHeaders,
  });
}

function stripHopByHop(input: Headers): Headers {
  const out = new Headers();
  for (const [name, value] of input) {
    if (HOP_BY_HOP.has(name.toLowerCase())) continue;
    out.set(name, value);
  }
  return out;
}

function teeStreamAndBill(
  upstreamResponse: Response,
  cfg: ProxyConfig,
  billCtx: BillContext,
): ReadableStream<Uint8Array> {
  const acc: StreamUsageAccumulator = { inputTokens: 0, outputTokens: 0, finalized: false };
  const decoder = new TextDecoder('utf-8', { fatal: false });
  const transform = new TransformStream<Uint8Array, Uint8Array>({
    transform(chunk, controller) {
      controller.enqueue(chunk);
      try {
        const text = decoder.decode(chunk, { stream: true });
        cfg.parseUsageFromStreamChunk(text, acc);
      } catch (err) {
        console.warn('[proxy] usage sniffer failed', err);
      }
    },
    async flush() {
      try {
        const tail = decoder.decode();
        if (tail) cfg.parseUsageFromStreamChunk(tail, acc);
      } catch {}
      if (acc.finalized || acc.outputTokens > 0) {
        await recordUsage(billCtx, { inputTokens: acc.inputTokens, outputTokens: acc.outputTokens });
      }
    },
  });
  return upstreamResponse.body!.pipeThrough(transform);
}

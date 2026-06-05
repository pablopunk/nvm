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
import { rateLimitChat, tooManyRequests } from './ratelimit';
import { estimateInputTokensFromBody, estimatePromptCredits, MAX_INPUT_TOKENS } from './limits';
import { backendVersion, desktopClientFromRequest, type DesktopClient } from './compatibility';
import { log } from './log';
import * as Sentry from '@sentry/astro';

const DASHBOARD_URL = 'https://nvm.fyi/dashboard';

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
  client: DesktopClient;
};

async function recordUsage(ctx: BillContext, tokens: UsageTokens, status: number, latencyMs: number) {
  const costUsd = computeUsdCost(ctx.costRow, tokens.inputTokens, tokens.outputTokens);
  const credits = usdToCredits(costUsd);
  const microcents = usdToMicrocents(costUsd);
  const billable = tokens.outputTokens > 0 && status >= 200 && status < 300;
  await db.transaction(async (tx) => {
    if (billable && credits > 0) {
      await tx.insert(creditLedger).values({
        userId: ctx.user.id,
        delta: -credits,
        kind: ctx.kind,
        reason: 'ai_usage',
        refId: ctx.requestId,
      });
    }
    await tx.insert(usage).values({
      userId: ctx.user.id,
      model: ctx.activeModelId,
      provider: ctx.provider,
      inputTokens: tokens.inputTokens,
      outputTokens: tokens.outputTokens,
      costCredits: billable ? credits : 0,
      upstreamCostMicrocents: billable ? microcents : 0,
      requestId: ctx.requestId,
      status,
      latencyMs,
    });
  });
  log.info('chat_completion', {
    request_id: ctx.requestId,
    user_id: ctx.user.id,
    route: 'proxy',
    model: ctx.activeModelId,
    provider: ctx.provider,
    kind: ctx.kind,
    status,
    latency_ms: latencyMs,
    input_tokens: tokens.inputTokens,
    output_tokens: tokens.outputTokens,
    cost_credits: billable ? credits : 0,
    client_name: ctx.client.name,
    client_version: ctx.client.version,
    client_api_version: ctx.client.apiVersion,
    client_platform: ctx.client.platform,
    client_arch: ctx.client.arch,
  });
}

type ResolvedRouting = {
  user: { id: string };
  provider: string;
  activeModelId: string;
  costRow: ModelCost;
  kind: 'free' | 'paid';
  balanceTotal: number;
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
      { error: { type: 'insufficient_credits', message: 'No credits remaining', dashboard_url: DASHBOARD_URL } },
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
    log.error('pricing_unavailable', { provider, model: activeModelId });
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
    balanceTotal: balances.total,
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
    if (lower === 'x-request-id' || lower.startsWith('x-nevermind-')) continue;
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

function withRequestId(res: Response, requestId: string): Response {
  res.headers.set('x-request-id', requestId);
  res.headers.set('x-nevermind-backend-version', backendVersion());
  return res;
}

export async function proxyAndBill(cfg: ProxyConfig): Promise<Response> {
  const requestId = randomUUID();
  const startedAt = Date.now();
  const client = desktopClientFromRequest(cfg.request);
  Sentry.getCurrentScope().setTag('request_id', requestId);
  if (client.version) Sentry.getCurrentScope().setTag('client_version', client.version);
  if (client.apiVersion) Sentry.getCurrentScope().setTag('client_api_version', String(client.apiVersion));
  const routing = await resolveRouting(cfg.request, cfg.authHeaderName);
  if (routing instanceof Response) return withRequestId(routing, requestId);

  Sentry.getCurrentScope().setUser({ id: routing.user.id });
  const rateDecision = await rateLimitChat(routing.user.id, routing.kind);
  if (!rateDecision.ok) {
    log.warn('rate_limited', { request_id: requestId, user_id: routing.user.id, scope: rateDecision.scope, client_version: client.version, client_api_version: client.apiVersion });
    return withRequestId(tooManyRequests(rateDecision), requestId);
  }

  const upstreamUrl = cfg.buildUpstreamUrl({
    upstreamBaseUrl: routing.upstreamBaseUrl,
    activeModelId: routing.activeModelId,
  });

  let forwardBody: BodyInit | undefined;
  if (cfg.request.method !== 'GET' && cfg.request.method !== 'HEAD') {
    const text = await cfg.request.text();
    const inputTokens = estimateInputTokensFromBody(text);
    if (inputTokens > MAX_INPUT_TOKENS) {
      return withRequestId(Response.json(
        { error: { type: 'prompt_too_large', message: `Prompt exceeds ${MAX_INPUT_TOKENS} input tokens` } },
        { status: 413 },
      ), requestId);
    }
    const estimatedCredits = estimatePromptCredits(inputTokens, routing.costRow);
    if (estimatedCredits > routing.balanceTotal) {
      return withRequestId(Response.json(
        { error: { type: 'insufficient_credits', message: 'Prompt cost would exceed remaining balance', estimated_credits: estimatedCredits, balance: routing.balanceTotal, dashboard_url: DASHBOARD_URL } },
        { status: 402 },
      ), requestId);
    }
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
    client,
  };

  const responseHeaders = stripHopByHop(upstreamResponse.headers);
  responseHeaders.set('x-request-id', requestId);
  responseHeaders.set('x-nevermind-backend-version', backendVersion());

  if (!upstreamResponse.ok) {
    const latencyMs = Date.now() - startedAt;
    await recordUsage(billCtx, { inputTokens: 0, outputTokens: 0 }, upstreamResponse.status, latencyMs).catch((err) => {
      log.error('record_usage_failed', { request_id: requestId, error: err });
    });
    return new Response(upstreamResponse.body, {
      status: upstreamResponse.status,
      statusText: upstreamResponse.statusText,
      headers: responseHeaders,
    });
  }

  if (isStreamingContentType(upstreamResponse.headers.get('content-type'))) {
    const transformed = teeStreamAndBill(upstreamResponse, cfg, billCtx, upstreamResponse.status, startedAt);
    return new Response(transformed, {
      status: upstreamResponse.status,
      statusText: upstreamResponse.statusText,
      headers: responseHeaders,
    });
  }

  const buffered = await upstreamResponse.arrayBuffer();
  const latencyMs = Date.now() - startedAt;
  let tokens: UsageTokens = { inputTokens: 0, outputTokens: 0 };
  try {
    const json = JSON.parse(new TextDecoder().decode(buffered));
    tokens = cfg.parseUsageFromJson(json) ?? tokens;
  } catch (err) {
    log.warn('parse_usage_failed', { request_id: requestId, error: err });
  }
  await recordUsage(billCtx, tokens, upstreamResponse.status, latencyMs);
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
  status: number,
  startedAt: number,
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
        log.warn('usage_sniffer_failed', { request_id: billCtx.requestId, error: err });
      }
    },
    async flush() {
      try {
        const tail = decoder.decode();
        if (tail) cfg.parseUsageFromStreamChunk(tail, acc);
      } catch {}
      const latencyMs = Date.now() - startedAt;
      await recordUsage(
        billCtx,
        { inputTokens: acc.inputTokens, outputTokens: acc.outputTokens },
        status,
        latencyMs,
      );
    },
  });
  return upstreamResponse.body!.pipeThrough(transform);
}

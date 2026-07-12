import { randomUUID, createHash } from 'node:crypto';
import { and, eq } from 'drizzle-orm';
import { db } from '../db/client';
import { creditLedger, usage, requestDedup } from '../db/schema';
import { getModelRoute, getModelProviderChain, ModelNotConfiguredError, parseExtensionAiModelRole, type ModelRouteSlot } from './settings';
import { ensureMonthlyFreeCredits, getBalances } from './users';
import {
  lookupModelCost,
  computeUsdCost,
  usdToCredits,
  usdToMicrocents,
  type ModelCost,
} from './cost';
import { getUpstreamConfig, selectApiForModel, providerSupportsFormat, UpstreamConfigError } from './upstream';
import { extractPatFromHeaders, getUserFromHeaders, type PatHeaderName } from './tokens';
import { rateLimitChat, tooManyRequests } from './ratelimit';
import { estimateInputTokensFromBody, estimatePromptCredits, MAX_INPUT_TOKENS } from './limits';
import { backendKillSwitchEnabled, backendVersion, desktopClientFromRequest, killSwitchResponse, type DesktopClient } from './compatibility';
import { log } from './log';
import * as Sentry from '@sentry/astro';

const DASHBOARD_URL = 'https://nvm.fyi/dashboard';

const CREDIT_GRACE_THRESHOLD = Math.max(0, Number(process.env.CREDIT_GRACE_THRESHOLD ?? 100));

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
  idempotencyKey?: string;
};

export type StreamUsageAccumulator = {
  inputTokens: number;
  outputTokens: number;
  finalized: boolean;
  pendingText?: string;
};

type BillContext = {
  user: { id: string };
  provider: string;
  activeModelId: string;
  costRow: ModelCost;
  kind: 'free' | 'paid';
  requestId: string;
  client: DesktopClient;
  estimatedInputTokens: number;
  dedupIdempotencyKey?: string;
};

export function resolveBillableTokens(ctx: BillContext, tokens: UsageTokens, status: number): UsageTokens {
  if (status >= 200 && status < 300 && tokens.outputTokens === 0 && ctx.estimatedInputTokens > 0) {
    log.error('usage_missing_on_success', {
      request_id: ctx.requestId,
      provider: ctx.provider,
      model: ctx.activeModelId,
      estimated_input_tokens: ctx.estimatedInputTokens,
    });
    return {
      inputTokens: tokens.inputTokens > 0 ? tokens.inputTokens : ctx.estimatedInputTokens,
      outputTokens: 1,
    };
  }
  return tokens;
}

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
      }).onConflictDoNothing();
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
  balanceAvailable: number;
  freeBalanceAvailable: number;
  upstreamBaseUrl: string;
  upstreamApiKey: string;
};

type ModelRouting = Pick<ResolvedRouting, 'provider' | 'activeModelId' | 'costRow' | 'upstreamBaseUrl' | 'upstreamApiKey'>;

async function resolveModelRouting(slot: ModelRouteSlot): Promise<Response | ModelRouting> {
  let provider: string;
  let activeModelId: string;
  try {
    const route = await getModelRoute(slot);
    provider = route.provider;
    activeModelId = route.modelId;
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

  try {
    const upstream = getUpstreamConfig(provider);
    return { provider, activeModelId, costRow, upstreamBaseUrl: upstream.baseUrl, upstreamApiKey: upstream.apiKey };
  } catch (err) {
    if (err instanceof UpstreamConfigError) {
      return Response.json(
        { error: { type: 'model_not_configured', message: err.message } },
        { status: 503 },
      );
    }
    throw err;
  }
}

const DEDUP_STALE_MS = 5 * 60 * 1000;

export async function handleDedup(
  idempotencyKey: string,
  userId: string,
  request: Request,
  requestId: string,
): Promise<Response | undefined> {
  const [inserted] = await db.insert(requestDedup).values({
    userId,
    idempotencyKey,
    status: 'in_flight',
    requestId,
  }).onConflictDoNothing().returning();

  if (inserted) return undefined;

  const [existing] = await db.select().from(requestDedup).where(
    and(eq(requestDedup.userId, userId), eq(requestDedup.idempotencyKey, idempotencyKey)),
  ).limit(1);

  if (!existing) return undefined;

  if (existing.status === 'completed') {
    if (existing.responseJson) {
      return replayDedupResponse(existing, requestId);
    }
    return withRequestId(Response.json(
      { error: { type: 'idempotency_conflict', message: 'Request already processed' } },
      { status: 409 },
    ), requestId);
  }

  if (existing.status === 'in_flight') {
    const createdAt = new Date(existing.createdAt).getTime();
    if (Date.now() - createdAt > DEDUP_STALE_MS) {
      await db.update(requestDedup).set({
        status: 'in_flight',
        requestId,
        createdAt: new Date(),
        requestHash: null,
        responseJson: null,
        responseHeaders: null,
        upstreamStatus: null,
        completedAt: null,
      }).where(eq(requestDedup.id, existing.id));
      return undefined;
    }
    return withRequestId(Response.json(
      { error: { type: 'idempotency_conflict', message: 'Request already in progress' } },
      { status: 409 },
    ), requestId);
  }

  await db.update(requestDedup).set({
    status: 'in_flight',
    requestId,
    createdAt: new Date(),
    requestHash: null,
    responseJson: null,
    responseHeaders: null,
    upstreamStatus: null,
    completedAt: null,
  }).where(eq(requestDedup.id, existing.id));
  return undefined;
}

function replayDedupResponse(existing: typeof requestDedup.$inferSelect, requestId: string): Response {
  const headers = new Headers();
  if (existing.responseHeaders && typeof existing.responseHeaders === 'object') {
    for (const [key, value] of Object.entries(existing.responseHeaders as Record<string, unknown>)) {
      headers.set(key, String(value));
    }
  }
  headers.set('x-request-id', requestId);
  headers.set('x-nevermind-backend-version', backendVersion());
  return new Response(JSON.stringify(existing.responseJson), {
    status: existing.upstreamStatus ?? 200,
    headers,
  });
}

async function resolveRouting(request: Request, headerName: PatHeaderName): Promise<Response | ResolvedRouting> {
  if (!extractPatFromHeaders(request, headerName)) {
    return new Response('Unauthorized', { status: 401 });
  }
  const user = await getUserFromHeaders(request, headerName);
  if (!user) return new Response('Unauthorized', { status: 401 });
  await ensureMonthlyFreeCredits(user.id);

  const balances = await getBalances(user.id);
  if (balances.total <= 0) {
    return Response.json(
      { error: { type: 'insufficient_credits', message: 'No credits remaining', dashboard_url: DASHBOARD_URL } },
      { status: 402 },
    );
  }

  const kind: 'free' | 'paid' = balances.paid > 0 ? 'paid' : 'free';
  const requestedModel = parseExtensionAiModelRole(request.headers.get('x-nevermind-ai-model'));
  const modelRouting = await resolveModelRouting(requestedModel ?? kind);
  if (modelRouting instanceof Response) return modelRouting;

  return {
    user,
    ...modelRouting,
    kind,
    balanceAvailable: kind === 'paid' ? balances.paid : balances.free,
    freeBalanceAvailable: balances.free,
  };
}

const FORWARD_ALLOWLIST = new Set([
  'content-type',
  'accept',
  'accept-encoding',
  'user-agent',
  'anthropic-beta',
  'anthropic-version',
  'openai-beta',
  'x-goog-api-client',
]);

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
    if (FORWARD_ALLOWLIST.has(name.toLowerCase())) {
      out.set(name, value);
    }
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

function chainExhaustedResponse(requestId: string): Response {
  return withRequestId(Response.json(
    { error: { type: 'upstream_unavailable', message: 'All configured upstream providers are unavailable' } },
    { status: 503 },
  ), requestId);
}

async function tryUpstreamProviders(
  cfg: ProxyConfig,
  routing: ResolvedRouting,
  forwardBody: BodyInit | undefined,
  requestId: string,
): Promise<{ response: Response; provider: string; costRow: ModelCost } | Response> {
  const apiFormat = selectApiForModel(routing.provider, routing.activeModelId);
  const failoverEnabled = !backendKillSwitchEnabled('ai_failover');

  let chainProviders: string[] = [];
  if (failoverEnabled) {
    try {
      chainProviders = await getModelProviderChain(routing.kind, routing.activeModelId);
    } catch (err) {
      log.warn('provider_chain_fetch_failed', { request_id: requestId, error: err instanceof Error ? err.message : String(err) });
    }
  }

  const providerChain = [routing.provider, ...chainProviders.filter((p) => p !== routing.provider)];
  let lastErrorStatus = 503;

  for (const providerId of providerChain) {
    if (!providerSupportsFormat(providerId, apiFormat)) {
      log.info('upstream_format_skip', { request_id: requestId, provider: providerId, format: apiFormat });
      continue;
    }

    let upstreamCfg;
    try {
      upstreamCfg = getUpstreamConfig(providerId);
    } catch (err) {
      log.warn('upstream_config_skip', { request_id: requestId, provider: providerId, error: err instanceof Error ? err.message : String(err) });
      continue;
    }

    const costRow = providerId === routing.provider
      ? routing.costRow
      : await lookupModelCost(providerId, routing.activeModelId);
    if (!costRow) {
      log.warn('upstream_cost_skip', { request_id: requestId, provider: providerId, model: routing.activeModelId });
      continue;
    }

    const upstreamUrl = cfg.buildUpstreamUrl({
      upstreamBaseUrl: upstreamCfg.baseUrl,
      activeModelId: routing.activeModelId,
    });

    const headers = buildForwardHeaders(
      cfg.request,
      cfg.authHeaderName,
      cfg.upstreamAuthHeaderName,
      cfg.formatUpstreamAuthValue(upstreamCfg.apiKey),
    );

    let resp;
    try {
      resp = await fetch(upstreamUrl, {
        method: cfg.request.method,
        headers,
        body: forwardBody,
      });
    } catch (err) {
      log.warn('upstream_fetch_error', {
        request_id: requestId,
        provider: providerId,
        error: err instanceof Error ? err.message : String(err),
      });
      continue;
    }

    const isLast = providerId === providerChain[providerChain.length - 1];

    if (resp.status >= 500) {
      lastErrorStatus = resp.status;
      log.warn('upstream_5xx', { request_id: requestId, provider: providerId, status: resp.status });
      if (!isLast) continue;
    }

    log.info('upstream_selected', { request_id: requestId, provider: providerId, status: resp.status });
    return { response: resp, provider: providerId, costRow };
  }

  return chainExhaustedResponse(requestId);
}

export async function proxyAndBill(cfg: ProxyConfig): Promise<Response> {
  const requestId = randomUUID();
  if (backendKillSwitchEnabled('ai_proxy')) return killSwitchResponse('ai_proxy', 'AI proxy is temporarily disabled.', requestId);
  const startedAt = Date.now();
  const client = desktopClientFromRequest(cfg.request);
  Sentry.getCurrentScope().setTag('request_id', requestId);
  if (client.version) Sentry.getCurrentScope().setTag('client_version', client.version);
  if (client.apiVersion) Sentry.getCurrentScope().setTag('client_api_version', String(client.apiVersion));
  let routing = await resolveRouting(cfg.request, cfg.authHeaderName);
  if (routing instanceof Response) return withRequestId(routing, requestId);

  Sentry.getCurrentScope().setUser({ id: routing.user.id });

  const idempotencyKey = cfg.idempotencyKey;
  const dedupEnabled = idempotencyKey && !backendKillSwitchEnabled('idempotency_dedup');
  if (dedupEnabled) {
    requestId = createHash('sha256').update(`${routing.user.id}:${idempotencyKey}`).digest('hex');
    const dedupResult = await handleDedup(idempotencyKey, routing.user.id, cfg.request, requestId);
    if (dedupResult instanceof Response) return dedupResult;
  }

  const rateDecision = await rateLimitChat(routing.user.id, routing.kind);
  if (!rateDecision.ok) {
    log.warn('rate_limited', { request_id: requestId, user_id: routing.user.id, scope: rateDecision.scope, client_version: client.version, client_api_version: client.apiVersion });
    return withRequestId(tooManyRequests(rateDecision), requestId);
  }

  let forwardBody: BodyInit | undefined;
  let estimatedInputTokens = 0;
  if (cfg.request.method !== 'GET' && cfg.request.method !== 'HEAD') {
    const text = await cfg.request.text();
    estimatedInputTokens = estimateInputTokensFromBody(text);
    if (estimatedInputTokens > MAX_INPUT_TOKENS) {
      return withRequestId(Response.json(
        { error: { type: 'prompt_too_large', message: `Prompt exceeds ${MAX_INPUT_TOKENS} input tokens` } },
        { status: 413 },
      ), requestId);
    }
    let estimatedCredits = estimatePromptCredits(estimatedInputTokens, routing.costRow);
    if (estimatedCredits > routing.balanceAvailable && routing.kind === 'paid' && routing.freeBalanceAvailable > 0) {
      const freeRouting = await resolveModelRouting(parseExtensionAiModelRole(cfg.request.headers.get('x-nevermind-ai-model')) ?? 'free');
      if (freeRouting instanceof Response) return withRequestId(freeRouting, requestId);
      const freeEstimatedCredits = estimatePromptCredits(estimatedInputTokens, freeRouting.costRow);
      if (freeEstimatedCredits <= routing.freeBalanceAvailable) {
        routing = {
          user: routing.user,
          ...freeRouting,
          kind: 'free',
          balanceAvailable: routing.freeBalanceAvailable,
          freeBalanceAvailable: routing.freeBalanceAvailable,
        };
        estimatedCredits = freeEstimatedCredits;
      }
    }
    if (estimatedCredits > routing.balanceAvailable + CREDIT_GRACE_THRESHOLD) {
      return withRequestId(Response.json(
        { error: { type: 'insufficient_credits', message: 'Prompt cost would exceed remaining balance', estimated_credits: estimatedCredits, balance: routing.balanceAvailable, dashboard_url: DASHBOARD_URL } },
        { status: 402 },
      ), requestId);
    } else if (estimatedCredits > routing.balanceAvailable) {
      log.warn('credit_grace_used', {
        request_id: requestId,
        user_id: routing.user.id,
        estimated_credits: estimatedCredits,
        balance: routing.balanceAvailable,
        grace_threshold: CREDIT_GRACE_THRESHOLD,
        kind: routing.kind,
      });
    }
    forwardBody = cfg.rewriteRequestBody ? cfg.rewriteRequestBody(text, routing.activeModelId) : text;
    if (dedupEnabled) {
      const bodyHash = createHash('sha256').update(`${text}|${routing.activeModelId}`).digest('hex');
      db.update(requestDedup).set({ requestHash: bodyHash }).where(
        and(eq(requestDedup.userId, routing.user.id), eq(requestDedup.idempotencyKey, idempotencyKey)),
      ).catch((err) => {
        log.warn('dedup_hash_update_failed', { request_id: requestId, error: err });
      });
    }
  }

  const result = await tryUpstreamProviders(cfg, routing, forwardBody, requestId);
  if (result instanceof Response) return result;

  const upstreamResponse = result.response;
  const winningProvider = result.provider;
  const winningCostRow = result.costRow;

  const billCtx: BillContext = {
    user: routing.user,
    provider: winningProvider,
    activeModelId: routing.activeModelId,
    costRow: winningCostRow,
    kind: routing.kind,
    requestId,
    client,
    estimatedInputTokens,
    dedupIdempotencyKey: dedupEnabled ? idempotencyKey : undefined,
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
    if (backendKillSwitchEnabled('ai_streaming')) return killSwitchResponse('ai_streaming', 'AI streaming is temporarily disabled.', requestId);
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
  let responseJsonForDedup: unknown = null;
  try {
    const json = JSON.parse(new TextDecoder().decode(buffered));
    responseJsonForDedup = json;
    tokens = cfg.parseUsageFromJson(json) ?? tokens;
  } catch (err) {
    log.warn('parse_usage_failed', { request_id: requestId, error: err });
  }
  tokens = resolveBillableTokens(billCtx, tokens, upstreamResponse.status);
  await recordUsage(billCtx, tokens, upstreamResponse.status, latencyMs);
  if (dedupEnabled) {
    const headersObj: Record<string, string> = {};
    responseHeaders.forEach((value, key) => { headersObj[key] = value; });
    db.update(requestDedup).set({
      status: 'completed',
      responseJson: responseJsonForDedup,
      responseHeaders: headersObj,
      upstreamStatus: upstreamResponse.status,
      completedAt: new Date(),
    }).where(
      and(eq(requestDedup.userId, routing.user.id), eq(requestDedup.idempotencyKey, idempotencyKey)),
    ).catch((err) => {
      log.error('dedup_complete_update_failed', { request_id: requestId, error: err });
    });
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
        if (acc.pendingText) cfg.parseUsageFromStreamChunk('\n', acc);
      } catch {}
      const latencyMs = Date.now() - startedAt;
      const streamTokens = resolveBillableTokens(
        billCtx,
        { inputTokens: acc.inputTokens, outputTokens: acc.outputTokens },
        status,
      );
      await recordUsage(billCtx, streamTokens, status, latencyMs);
      if (billCtx.dedupIdempotencyKey) {
        db.update(requestDedup).set({
          status: 'completed',
          upstreamStatus: status,
          completedAt: new Date(),
        }).where(
          and(eq(requestDedup.userId, billCtx.user.id), eq(requestDedup.idempotencyKey, billCtx.dedupIdempotencyKey)),
        ).catch((err) => {
          log.error('dedup_stream_complete_update_failed', { request_id: billCtx.requestId, error: err });
        });
      }
    },
  });
  return upstreamResponse.body!.pipeThrough(transform);
}

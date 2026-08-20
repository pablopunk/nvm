import { createHash, randomUUID } from 'node:crypto';
import { and, eq, sql } from 'drizzle-orm';
import { db } from '../db/client';
import { creditReservations, requestDedup } from '../db/schema';
import {
  getModelRoute,
  getModelProviderChain,
  modelRouteSlotForAccount,
  ModelNotConfiguredError,
  parseExtensionAiModelRole,
  tieredModelRouteSlot,
  type ExtensionAiModelRole,
  type ModelRouteSlot,
  type ThinkingLevel,
} from './settings';
import { ensureMonthlyFreeCredits, getBalances } from './users';
import {
  lookupModelCost,
  computeUsdCost,
  usdToCredits,
  type ModelCost,
} from './cost';
import { lookupModelDescriptor } from './pricing';
import { getUpstreamConfig, selectApiForModel, providerSupportsFormat, UpstreamConfigError } from './upstream';
import { extractPatFromHeaders, getUserFromHeaders, type PatHeaderName } from './tokens';
import { rateLimitChat, tooManyRequests } from './ratelimit';
import { estimateInputTokensFromBody, estimateRequestCredits, MAX_INPUT_TOKENS, requestedMaxOutputTokens } from './limits';
import { finalizeReservation, reserveCredits } from './credit-reservations';
import { backendKillSwitchEnabled, backendVersion, desktopClientFromRequest, killSwitchResponse, type DesktopClient } from './compatibility';
import { log } from './log';
import * as Sentry from '@sentry/astro';
import { PRODUCTION_WEB_ORIGIN } from '../../../app/shared/public-origin';

const DASHBOARD_URL = `${PRODUCTION_WEB_ORIGIN}/dashboard`;

export type UsageTokens = { inputTokens: number; outputTokens: number };

export type ProxyConfig = {
  request: Request;
  authHeaderName: PatHeaderName;
  upstreamAuthHeaderName: 'authorization' | 'x-api-key' | 'x-goog-api-key';
  formatUpstreamAuthValue: (apiKey: string) => string;
  buildUpstreamUrl: (cfg: { upstreamBaseUrl: string; activeModelId: string }) => string;
  rewriteRequestBody?: (
    bodyText: string,
    routing: Pick<ModelRouting, 'activeModelId' | 'provider' | 'thinkingLevel'>,
  ) => string;
  parseUsageFromJson: (json: any) => UsageTokens | null;
  parseUsageFromStreamChunk: (chunk: string, acc: StreamUsageAccumulator, finalize?: boolean) => void;
  idempotencyKey?: string;
};

export type StreamUsageAccumulator = {
  inputTokens: number;
  outputTokens: number;
  finalized: boolean;
  pendingText?: string;
  reportMalformedFrame?: (error: unknown) => void;
};

export function completeStreamLines(chunkText: string, acc: StreamUsageAccumulator, finalize = false): string[] {
  const text = `${acc.pendingText ?? ''}${chunkText}`;
  if (finalize) {
    acc.pendingText = '';
    return text ? text.split(/\r?\n/) : [];
  }
  const lines = text.split(/\r?\n/);
  acc.pendingText = lines.pop() ?? '';
  return lines;
}

export function parseStreamUsageJson(payload: string, acc: StreamUsageAccumulator): any | null {
  try {
    return JSON.parse(payload);
  } catch (error) {
    acc.reportMalformedFrame?.(error);
    return null;
  }
}

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
  dedupRequestHash?: string;
};

export type DedupClaim = {
  userId: string;
  idempotencyKey: string;
  requestId: string;
  requestHash: string;
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

type DedupTerminal = {
  status: 'completed' | 'failed';
  responseJson?: unknown;
  responseHeaders?: Record<string, string>;
  upstreamStatus?: number;
};

function reservationDedupFinalization(
  ctx: BillContext,
  terminal?: DedupTerminal,
) {
  if (!(terminal && ctx.dedupIdempotencyKey && ctx.dedupRequestHash)) {
    return undefined;
  }
  return {
    userId: ctx.user.id,
    idempotencyKey: ctx.dedupIdempotencyKey,
    requestHash: ctx.dedupRequestHash,
    ...terminal,
  };
}

function claimDedupFinalization(
  claim: DedupClaim | undefined,
  terminal: DedupTerminal,
) {
  if (!claim) return undefined;
  return {
    userId: claim.userId,
    idempotencyKey: claim.idempotencyKey,
    requestHash: claim.requestHash,
    ...terminal,
  };
}

function billContextDedupClaim(ctx: BillContext): DedupClaim | undefined {
  if (!(ctx.dedupIdempotencyKey && ctx.dedupRequestHash)) return undefined;
  return {
    userId: ctx.user.id,
    idempotencyKey: ctx.dedupIdempotencyKey,
    requestId: ctx.requestId,
    requestHash: ctx.dedupRequestHash,
  };
}

async function recordUsage(
  ctx: BillContext,
  tokens: UsageTokens,
  status: number,
  latencyMs: number,
  dedupTerminal?: DedupTerminal,
) {
  const billable = tokens.outputTokens > 0 && status >= 200 && status < 300;
  const credits = billable ? usdToCredits(computeUsdCost(ctx.costRow, tokens.inputTokens, tokens.outputTokens)) : 0;
  await finalizeReservation(billable
    ? { requestId: ctx.requestId, outcome: 'settle', model: ctx.activeModelId, provider: ctx.provider, tokens, costRow: ctx.costRow, status, latencyMs, dedup: reservationDedupFinalization(ctx, dedupTerminal) }
    : { requestId: ctx.requestId, outcome: 'release', dedup: reservationDedupFinalization(ctx, dedupTerminal) });
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

async function releaseReservationAfterError(
  requestId: string,
  dedupClaim: DedupClaim | undefined,
  error: unknown,
) {
  try {
    await finalizeReservation({
      requestId,
      outcome: 'release',
      dedup: claimDedupFinalization(dedupClaim, { status: 'failed' }),
    });
  } catch (releaseError) {
    log.error('reservation_error_release_failed', { request_id: requestId, error, release_error: releaseError });
  }
}

export async function markDedupFailed(claim?: DedupClaim) {
  if (!claim) return;
  const [updated] = await db
    .update(requestDedup)
    .set({ status: 'failed', completedAt: new Date() })
    .where(
      and(
        eq(requestDedup.userId, claim.userId),
        eq(requestDedup.idempotencyKey, claim.idempotencyKey),
        eq(requestDedup.requestId, claim.requestId),
        eq(requestDedup.requestHash, claim.requestHash),
        eq(requestDedup.status, 'in_flight'),
      ),
    )
    .returning({ id: requestDedup.id });
  if (!updated) {
    throw new Error(`Missing matching idempotency claim ${claim.requestId}`);
  }
}

type ResolvedRouting = {
  user: { id: string };
  routeSlot: ModelRouteSlot;
  provider: string;
  activeModelId: string;
  thinkingLevel: ThinkingLevel;
  costRow: ModelCost;
  kind: 'free' | 'paid';
  balanceAvailable: number;
  freeBalanceAvailable: number;
  upstreamBaseUrl: string;
  upstreamApiKey: string;
};

type ModelRouting = Pick<
  ResolvedRouting,
  | 'provider'
  | 'activeModelId'
  | 'thinkingLevel'
  | 'costRow'
  | 'upstreamBaseUrl'
  | 'upstreamApiKey'
>;

async function resolveModelRouting(slot: ModelRouteSlot): Promise<Response | ModelRouting> {
  let provider: string;
  let activeModelId: string;
  let thinkingLevel: ThinkingLevel;
  try {
    const route = await getModelRoute(slot);
    provider = route.provider;
    activeModelId = route.modelId;
    thinkingLevel = route.thinkingLevel;
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
    return {
      provider,
      activeModelId,
      thinkingLevel,
      costRow,
      upstreamBaseUrl: upstream.baseUrl,
      upstreamApiKey: upstream.apiKey,
    };
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

const DEDUP_STALE_MS = 7 * 60 * 1000;

const REQUEST_IDENTITY_HEADERS = [
  'accept',
  'anthropic-beta',
  'anthropic-version',
  'content-type',
  'openai-beta',
  'x-goog-api-client',
] as const;

function normalizedRequestRoute(request: Request) {
  const url = new URL(request.url);
  url.searchParams.sort();
  const pathname = url.pathname.replace(/\/+$/, '') || '/';
  return `${pathname}${url.search}`;
}

function semanticHeaderValue(request: Request, name: string) {
  return request.headers.get(name)?.trim() ?? '';
}

export function aiRequestHash(request: Request, body: Uint8Array) {
  const requestedModel =
    parseExtensionAiModelRole(request.headers.get('x-nevermind-ai-model')) ??
    'smart';
  const modelTier = semanticHeaderValue(request, 'x-nevermind-ai-model-tier');
  const creditKind = semanticHeaderValue(request, 'x-nevermind-ai-credit-kind');
  const identity = {
    method: request.method.toUpperCase(),
    route: normalizedRequestRoute(request),
    requestedModel,
    modelTier: modelTier === 'free' || modelTier === 'pro' ? modelTier : '',
    creditKind:
      creditKind === 'free' || creditKind === 'paid' ? creditKind : '',
    headers: REQUEST_IDENTITY_HEADERS.map((name) => [
      name,
      semanticHeaderValue(request, name),
    ]),
  };
  return createHash('sha256')
    .update(JSON.stringify(identity))
    .update('\0')
    .update(body)
    .digest('hex');
}

export async function handleDedup(
  idempotencyKey: string,
  userId: string,
  requestHash: string,
  requestId: string,
): Promise<Response | undefined> {
  const [inserted] = await db.insert(requestDedup).values({
    userId,
    idempotencyKey,
    requestHash,
    status: 'in_flight',
    requestId,
  }).onConflictDoNothing().returning();

  if (inserted) return undefined;

  const [existing] = await db.select().from(requestDedup).where(
    and(eq(requestDedup.userId, userId), eq(requestDedup.idempotencyKey, idempotencyKey)),
  ).limit(1);

  if (!existing) {
    return withRequestId(Response.json(
      { error: { type: 'idempotency_conflict', message: 'Idempotency claim is unavailable' } },
      { status: 409 },
    ), requestId);
  }

  if (existing.requestHash !== requestHash) {
    return withRequestId(Response.json(
      { error: { type: 'idempotency_conflict', message: 'Idempotency key was used for a different request' } },
      { status: 409 },
    ), requestId);
  }

  if (existing.status === 'completed') {
    if (existing.responseJson !== null) {
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
      const [reservation] = existing.requestId
        ? await db.select({
            status: creditReservations.status,
            expiresAt: creditReservations.expiresAt,
          }).from(creditReservations).where(
            eq(creditReservations.requestId, existing.requestId),
          ).limit(1)
        : [];
      if (reservation?.status === 'pending') {
        if (new Date(reservation.expiresAt).getTime() > Date.now()) {
          return withRequestId(Response.json(
            { error: { type: 'idempotency_conflict', message: 'Request reservation is still active' } },
            { status: 409 },
          ), requestId);
        }
        await finalizeReservation({
          requestId: existing.requestId!,
          outcome: 'release',
          dedup: {
            userId,
            idempotencyKey,
            requestHash,
            status: 'failed',
          },
        });
        return handleDedup(idempotencyKey, userId, requestHash, requestId);
      }
      const [reclaimed] = await db.update(requestDedup).set({
        status: 'in_flight',
        requestId,
        createdAt: new Date(),
        responseJson: null,
        responseHeaders: null,
        upstreamStatus: null,
        completedAt: null,
      }).where(and(
        eq(requestDedup.id, existing.id),
        eq(requestDedup.requestId, existing.requestId!),
        eq(requestDedup.requestHash, requestHash),
        eq(requestDedup.status, 'in_flight'),
        sql`not exists (
          select 1
          from ${creditReservations}
          where ${creditReservations.requestId} = ${existing.requestId}
            and ${creditReservations.status} = 'pending'
        )`,
      )).returning();
      if (!reclaimed) {
        return withRequestId(Response.json(
          { error: { type: 'idempotency_conflict', message: 'Request already reclaimed' } },
          { status: 409 },
        ), requestId);
      }
      return undefined;
    }
    return withRequestId(Response.json(
      { error: { type: 'idempotency_conflict', message: 'Request already in progress' } },
      { status: 409 },
    ), requestId);
  }

  const [reclaimed] = await db.update(requestDedup).set({
    status: 'in_flight',
    requestId,
    createdAt: new Date(),
    responseJson: null,
    responseHeaders: null,
    upstreamStatus: null,
    completedAt: null,
  }).where(and(
    eq(requestDedup.id, existing.id),
    eq(requestDedup.requestId, existing.requestId!),
    eq(requestDedup.requestHash, requestHash),
    eq(requestDedup.status, 'failed'),
  )).returning({ id: requestDedup.id });
  if (reclaimed) return undefined;
  return withRequestId(Response.json(
    { error: { type: 'idempotency_conflict', message: 'Request already reclaimed' } },
    { status: 409 },
  ), requestId);
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

  const advertisedCreditKind = request.headers.get('x-nevermind-ai-credit-kind');
  const kind: 'free' | 'paid' = advertisedCreditKind === 'free' || advertisedCreditKind !== 'paid' && balances.paid <= 0 ? 'free' : 'paid';
  const requestedModel = parseExtensionAiModelRole(request.headers.get('x-nevermind-ai-model')) ?? 'smart';
  const advertisedModelTier = request.headers.get('x-nevermind-ai-model-tier');
  const routeSlot = advertisedModelTier === 'pro' && user.plan === 'pro'
    ? tieredModelRouteSlot('pro', requestedModel)
    : advertisedModelTier === 'free'
      ? tieredModelRouteSlot('free', requestedModel)
      : modelRouteSlotForAccount(user.plan, balances.paid, requestedModel, kind === 'free');
  const modelRouting = await resolveModelRouting(routeSlot);
  if (modelRouting instanceof Response) return modelRouting;

  return {
    user,
    routeSlot,
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
      chainProviders = await getModelProviderChain(routing.routeSlot, routing.activeModelId);
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
  let requestId: string = randomUUID();
  if (backendKillSwitchEnabled('ai_proxy')) return killSwitchResponse('ai_proxy', 'AI proxy is temporarily disabled.', requestId);
  const startedAt = Date.now();
  const client = desktopClientFromRequest(cfg.request);
  Sentry.getCurrentScope().setTag('request_id', requestId);
  if (client.version) Sentry.getCurrentScope().setTag('client_version', client.version);
  if (client.apiVersion) Sentry.getCurrentScope().setTag('client_api_version', String(client.apiVersion));
  let routing = await resolveRouting(cfg.request, cfg.authHeaderName);
  if (routing instanceof Response) return withRequestId(routing, requestId);

  Sentry.getCurrentScope().setUser({ id: routing.user.id });

  const requestBodyBuffer = await cfg.request.clone().arrayBuffer();
  const requestBodyBytes = new Uint8Array(requestBodyBuffer);
  const requestBodyText = new TextDecoder().decode(requestBodyBytes);
  const requestHash = aiRequestHash(cfg.request, requestBodyBytes);

  const idempotencyKey = cfg.idempotencyKey;
  const dedupEnabled = idempotencyKey && !backendKillSwitchEnabled('idempotency_dedup');
  const dedupClaim: DedupClaim | undefined = dedupEnabled
    ? {
        userId: routing.user.id,
        idempotencyKey,
        requestId,
        requestHash,
      }
    : undefined;
  if (dedupEnabled) {
    const dedupResult = await handleDedup(
      idempotencyKey,
      routing.user.id,
      requestHash,
      requestId,
    );
    if (dedupResult instanceof Response) return dedupResult;
  }

  let reservationCommitted = false;
  try {
    const rateDecision = await rateLimitChat(routing.user.id, routing.kind);
    if (!rateDecision.ok) {
      log.warn('rate_limited', { request_id: requestId, user_id: routing.user.id, scope: rateDecision.scope, client_version: client.version, client_api_version: client.apiVersion });
      await markDedupFailed(dedupClaim);
      return withRequestId(tooManyRequests(rateDecision), requestId);
    }

    let forwardBody: BodyInit | undefined;
    let estimatedInputTokens = 0;
    if (cfg.request.method !== 'GET' && cfg.request.method !== 'HEAD') {
    const text = requestBodyText;
    estimatedInputTokens = estimateInputTokensFromBody(text);
    if (estimatedInputTokens > MAX_INPUT_TOKENS) {
      await markDedupFailed(dedupClaim);
      return withRequestId(Response.json(
        { error: { type: 'prompt_too_large', message: `Prompt exceeds ${MAX_INPUT_TOKENS} input tokens` } },
        { status: 413 },
      ), requestId);
    }
    let parsedBody: unknown = null;
    try { parsedBody = JSON.parse(text); } catch { /* upstream retains its existing invalid-JSON behavior */ }
    const requestedOutput = requestedMaxOutputTokens(parsedBody);
    const maxOutputFor = async (candidate: ModelRouting) => {
      const descriptor = await lookupModelDescriptor(candidate.provider, candidate.activeModelId);
      const serverMaximum = descriptor?.maxTokens ?? 32_000;
      return Math.min(requestedOutput ?? serverMaximum, serverMaximum);
    };
    let maxOutputTokens = await maxOutputFor(routing);
    let estimatedCredits = estimateRequestCredits(estimatedInputTokens, maxOutputTokens, routing.costRow);
    let reservation = await reserveCredits({
      requestId,
      userId: routing.user.id,
      kind: routing.kind,
      credits: estimatedCredits,
    });
    if (!reservation.ok && reservation.reason === 'insufficient_credits' && routing.kind === 'paid') {
      const requestedModel: ExtensionAiModelRole = parseExtensionAiModelRole(cfg.request.headers.get('x-nevermind-ai-model')) ?? 'smart';
      const freeRouting = await resolveModelRouting(tieredModelRouteSlot('free', requestedModel));
      if (freeRouting instanceof Response) {
        await markDedupFailed(dedupClaim);
        return withRequestId(freeRouting, requestId);
      }
      if (selectApiForModel(freeRouting.provider, freeRouting.activeModelId) === selectApiForModel(routing.provider, routing.activeModelId)) {
        maxOutputTokens = await maxOutputFor(freeRouting);
        const freeEstimatedCredits = estimateRequestCredits(estimatedInputTokens, maxOutputTokens, freeRouting.costRow);
        const freeReservation = await reserveCredits({ requestId, userId: routing.user.id, kind: 'free', credits: freeEstimatedCredits });
        if (freeReservation.ok) {
          routing = { user: routing.user, routeSlot: tieredModelRouteSlot('free', requestedModel), ...freeRouting, kind: 'free', balanceAvailable: freeReservation.balance - freeReservation.reserved, freeBalanceAvailable: freeReservation.balance - freeReservation.reserved };
          estimatedCredits = freeEstimatedCredits;
          reservation = freeReservation;
        }
      }
    }
    if (!reservation.ok && reservation.reason === 'request_already_reserved') {
      await markDedupFailed(dedupClaim);
      return withRequestId(Response.json(
        { error: { type: 'idempotency_conflict', message: 'Request execution was already finalized or is still in progress' } },
        { status: 409 },
      ), requestId);
    }
    if (!reservation.ok) {
      const available = reservation.balance - reservation.reserved;
      await markDedupFailed(dedupClaim);
      return withRequestId(Response.json(
        { error: { type: 'insufficient_credits', message: 'Request cost would exceed remaining balance', estimated_credits: estimatedCredits, balance: available, dashboard_url: DASHBOARD_URL } },
        { status: 402 },
      ), requestId);
    }
    reservationCommitted = true;
    if (estimatedCredits + reservation.reserved > reservation.balance) {
      log.warn('credit_grace_used', {
        request_id: requestId,
        user_id: routing.user.id,
        estimated_credits: estimatedCredits,
        balance: reservation.balance - reservation.reserved,
        kind: routing.kind,
      });
    }
    forwardBody = cfg.rewriteRequestBody
      ? cfg.rewriteRequestBody(text, routing)
      : requestBodyBuffer;
  }

  const result = await tryUpstreamProviders(cfg, routing, forwardBody, requestId);
  if (result instanceof Response) {
    await finalizeReservation({
      requestId,
      outcome: 'release',
      dedup: claimDedupFinalization(dedupClaim, { status: 'failed' }),
    });
    return result;
  }

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
    dedupRequestHash: dedupEnabled ? requestHash : undefined,
  };

  const responseHeaders = stripHopByHop(upstreamResponse.headers);
  responseHeaders.set('x-request-id', requestId);
  responseHeaders.set('x-nevermind-backend-version', backendVersion());

  if (!upstreamResponse.ok) {
    const latencyMs = Date.now() - startedAt;
    await recordUsage(
      billCtx,
      { inputTokens: 0, outputTokens: 0 },
      upstreamResponse.status,
      latencyMs,
      { status: 'failed', upstreamStatus: upstreamResponse.status },
    );
    return new Response(upstreamResponse.body, {
      status: upstreamResponse.status,
      statusText: upstreamResponse.statusText,
      headers: responseHeaders,
    });
  }

  if (isStreamingContentType(upstreamResponse.headers.get('content-type'))) {
    if (backendKillSwitchEnabled('ai_streaming')) {
      await finalizeReservation({
        requestId,
        outcome: 'release',
        dedup: claimDedupFinalization(dedupClaim, { status: 'failed' }),
      });
      return killSwitchResponse('ai_streaming', 'AI streaming is temporarily disabled.', requestId);
    }
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
  const headersObj: Record<string, string> = {};
  responseHeaders.forEach((value, key) => { headersObj[key] = value; });
  await recordUsage(billCtx, tokens, upstreamResponse.status, latencyMs, {
    status: 'completed',
    responseJson: responseJsonForDedup,
    responseHeaders: headersObj,
    upstreamStatus: upstreamResponse.status,
  });
  return new Response(buffered, {
    status: upstreamResponse.status,
    statusText: upstreamResponse.statusText,
    headers: responseHeaders,
  });
  } catch (error) {
    if (reservationCommitted) {
      await releaseReservationAfterError(requestId, dedupClaim, error);
    } else if (dedupEnabled) {
      await markDedupFailed(dedupClaim)
        .catch((dedupError) => log.error('dedup_failed_update_failed', { request_id: requestId, error: dedupError }));
    }
    throw error;
  }
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
  function reportMalformedStreamUsageFrame(error: unknown) {
    log.warn('stream_usage_frame_parse_failed', { request_id: billCtx.requestId, error });
  }

  function sniffStreamUsage(text: string, finalize = false) {
    try {
      cfg.parseUsageFromStreamChunk(text, acc, finalize);
    } catch (error) {
      log.warn('usage_sniffer_failed', { request_id: billCtx.requestId, error });
    }
  }

  const acc: StreamUsageAccumulator = {
    inputTokens: 0,
    outputTokens: 0,
    finalized: false,
    reportMalformedFrame: reportMalformedStreamUsageFrame,
  };
  const decoder = new TextDecoder('utf-8', { fatal: false });
  let terminal = false;
  let deliveredOutput = false;
  let loggedFirstChunk = false;
  async function finish(naturalCompletion: boolean) {
    if (terminal) return;
    terminal = true;
    if (naturalCompletion) {
      sniffStreamUsage(decoder.decode(), true);
    }
    const observedOutput = acc.outputTokens > 0 || deliveredOutput;
    const streamTokens = naturalCompletion || deliveredOutput
      ? resolveBillableTokens(billCtx, { inputTokens: acc.inputTokens, outputTokens: acc.outputTokens }, status)
      : { inputTokens: acc.inputTokens, outputTokens: acc.outputTokens };
    try {
      await recordUsage(
        billCtx,
        streamTokens,
        observedOutput || naturalCompletion ? status : 499,
        Date.now() - startedAt,
        naturalCompletion
          ? { status: 'completed', upstreamStatus: status }
          : { status: 'failed', upstreamStatus: status },
      );
    } catch (error) {
      await releaseReservationAfterError(
        billCtx.requestId,
        billContextDedupClaim(billCtx),
        error,
      );
      throw error;
    }
  }

  const reader = upstreamResponse.body!.getReader();
  return new ReadableStream<Uint8Array>({
    async pull(controller) {
      try {
        const { value, done } = await reader.read();
        if (done) {
          await finish(true);
          controller.close();
          return;
        }
        if (value) {
          if (value.byteLength > 0 && !loggedFirstChunk) {
            loggedFirstChunk = true;
            log.info('upstream_first_chunk', {
              request_id: billCtx.requestId,
              provider: billCtx.provider,
              model: billCtx.activeModelId,
              latency_ms: Date.now() - startedAt,
            });
          }
          sniffStreamUsage(decoder.decode(value, { stream: true }));
          if (value.byteLength > 0) deliveredOutput = true;
          controller.enqueue(value);
        }
      } catch (error) {
        await finish(false).catch((finalizeError) => log.error('stream_finalize_failed', { request_id: billCtx.requestId, error: finalizeError }));
        controller.error(error);
      }
    },
    async cancel(reason) {
      // Claim the terminal transition before cancelling the upstream reader:
      // reader.cancel() resolves a pending read as done, which must not race
      // cancellation into the natural-completion billing policy.
      const finalization = finish(false)
        .catch((error) => log.error('stream_cancel_finalize_failed', { request_id: billCtx.requestId, error }));
      await reader.cancel(reason).catch(() => undefined);
      await finalization;
    },
  });
}

import { createHash, randomUUID } from 'node:crypto';
import * as Sentry from '@sentry/astro';
import { waitUntil } from '@vercel/functions';
import { and, eq, sql } from 'drizzle-orm';
import { PRODUCTION_WEB_ORIGIN } from '../../../app/shared/public-origin';
import { db } from '../db/client';
import { creditReservations, requestDedup } from '../db/schema';
import { analyzeAiImageInputs } from './ai-image-input';
import {
  backendKillSwitchEnabled,
  backendVersion,
  type DesktopClient,
  desktopClientFromRequest,
  killSwitchResponse,
} from './compatibility';
import {
  computeMaximumTokenPricedCost,
  computeUsdCost,
  lookupModelCost,
  type ModelCost,
  usdToCredits,
} from './cost';
import {
  finalizeReservation,
  reserveCredits,
  resizeReservation,
} from './credit-reservations';
import { captureStreamFailure } from './diagnostics';
import { freeSpendCapErrorBody } from './free-spend-cap';
import {
  type ReviewedImageTokenLimits,
  reviewedImageTokenLimits,
} from './image-cost-bound';
import {
  conservativeImageRequestInputTokenCeiling,
  containsUnpricedProviderTool,
  containsUnsupportedMultimodalPayload,
  estimateInputTokensFromBody,
  estimateInputTokensFromValue,
  MAX_CHAT_COMPLETION_CHOICES,
  MAX_INPUT_TOKENS,
  MAX_PROXY_REQUEST_BODY_BYTES,
  OUTPUT_CONTEXT_SAFETY_TOKENS,
  requestedCompletionCount,
  requestedMaxOutputTokens,
  supportsBoundedImageInput,
} from './limits';
import { log } from './log';
import { lookupModelDescriptor, type ModelDescriptor } from './pricing';
import { rateLimitChat, tooManyRequests } from './ratelimit';
import { healthyRouteCandidates } from './route-breakers';
import {
  decideAccountModel,
  type ExtensionAiModelRole,
  getModelProviderRoutes,
  getModelRoute,
  ModelNotConfiguredError,
  type ModelRouteSlot,
  parseExtensionAiModelRole,
  type ThinkingLevel,
} from './settings';
import {
  extractPatFromHeaders,
  getUserFromHeaders,
  type PatHeaderName,
} from './tokens';
import {
  getUpstreamConfig,
  providerSupportsFormat,
  selectApiForModel,
  UpstreamConfigError,
} from './upstream';
import { ensureMonthlyFreeCredits, getBalances } from './users';

const DASHBOARD_URL = `${PRODUCTION_WEB_ORIGIN}/dashboard`;

export type UsageTokens = {
  inputTokens: number;
  outputTokens: number;
  cachedInputTokens?: number;
  cacheWriteInputTokens?: number;
  reasoningTokens?: number;
  providerCostUsd?: number;
};

export type ProxyConfig = {
  request: Request;
  authHeaderName: PatHeaderName;
  upstreamAuthHeaderName: 'authorization' | 'x-api-key' | 'x-goog-api-key';
  formatUpstreamAuthValue: (apiKey: string) => string;
  buildUpstreamUrl: (cfg: {
    upstreamBaseUrl: string;
    activeModelId: string;
  }) => string;
  rewriteRequestBody?: (
    bodyText: string,
    routing: Pick<ModelRouting, 'activeModelId' | 'provider' | 'thinkingLevel'>,
    maxOutputTokens: number,
  ) => string;
  parseUsageFromJson: (json: any) => UsageTokens | null;
  parseUsageFromStreamChunk: (
    chunk: string,
    acc: StreamUsageAccumulator,
    finalize?: boolean,
  ) => void;
  idempotencyKey?: string;
};

export type StreamUsageAccumulator = {
  inputTokens: number;
  outputTokens: number;
  cachedInputTokens?: number;
  cacheWriteInputTokens?: number;
  reasoningTokens?: number;
  providerCostUsd?: number;
  observedOutputCharacters?: number;
  finalized: boolean;
  pendingText?: string;
  reportMalformedFrame?: (error: unknown) => void;
};

export function completeStreamLines(
  chunkText: string,
  acc: StreamUsageAccumulator,
  finalize = false,
): string[] {
  const text = `${acc.pendingText ?? ''}${chunkText}`;
  if (finalize) {
    acc.pendingText = '';
    return text ? text.split(/\r?\n/) : [];
  }
  const lines = text.split(/\r?\n/);
  acc.pendingText = lines.pop() ?? '';
  return lines;
}

export function parseStreamUsageJson(
  payload: string,
  acc: StreamUsageAccumulator,
): any | null {
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
  routeSlot: ModelRouteSlot;
  activeModelId: string;
  costRow: ModelCost;
  kind: 'free' | 'paid';
  requestId: string;
  client: DesktopClient;
  estimatedInputTokens: number;
  reservationCredits?: number;
  dedupIdempotencyKey?: string;
  dedupRequestHash?: string;
};

function requestReservationCostUsd(
  costRow: ModelCost,
  inputTokens: number,
  outputTokens: number,
  imageCount: number,
  completionCount: number,
  reviewedLimits?: ReviewedImageTokenLimits,
) {
  const totalOutputTokens = outputTokens * completionCount;
  return imageCount > 0
    ? computeMaximumTokenPricedCost(
        costRow,
        inputTokens,
        totalOutputTokens,
        reviewedLimits?.maximumCost,
      )
    : computeUsdCost(costRow, inputTokens, totalOutputTokens);
}

function requestReservationCredits(
  costRow: ModelCost,
  inputTokens: number,
  outputTokens: number,
  imageCount: number,
  completionCount: number,
  reviewedLimits?: ReviewedImageTokenLimits,
) {
  return usdToCredits(
    requestReservationCostUsd(
      costRow,
      inputTokens,
      outputTokens,
      imageCount,
      completionCount,
      reviewedLimits,
    ),
  );
}

export type DedupClaim = {
  userId: string;
  idempotencyKey: string;
  requestId: string;
  requestHash: string;
};

export function resolveBillableTokens(
  ctx: BillContext,
  tokens: UsageTokens,
  status: number,
): UsageTokens {
  if (
    status >= 200 &&
    status < 300 &&
    ctx.estimatedInputTokens > 0 &&
    (tokens.inputTokens === 0 || tokens.outputTokens === 0)
  ) {
    log.error('usage_missing_on_success', {
      request_id: ctx.requestId,
      provider: ctx.provider,
      model: ctx.activeModelId,
      estimated_input_tokens: ctx.estimatedInputTokens,
    });
    return {
      ...tokens,
      inputTokens:
        tokens.inputTokens > 0 ? tokens.inputTokens : ctx.estimatedInputTokens,
      outputTokens: tokens.outputTokens > 0 ? tokens.outputTokens : 1,
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
  const providerCostUsd =
    ctx.provider === 'openrouter' &&
    tokens.providerCostUsd != null &&
    Number.isFinite(tokens.providerCostUsd) &&
    tokens.providerCostUsd >= 0
      ? tokens.providerCostUsd
      : undefined;
  const costUsd =
    providerCostUsd ??
    computeUsdCost(
      ctx.costRow,
      tokens.inputTokens,
      tokens.outputTokens,
      tokens,
    );
  const calculatedCredits = billable ? usdToCredits(costUsd) : 0;
  const credits =
    ctx.reservationCredits == null
      ? calculatedCredits
      : Math.min(calculatedCredits, ctx.reservationCredits);
  await finalizeReservation(
    billable
      ? {
          requestId: ctx.requestId,
          outcome: 'settle',
          model: ctx.activeModelId,
          provider: ctx.provider,
          routeSlot: ctx.routeSlot,
          tokens,
          costRow: ctx.costRow,
          providerCostUsd,
          costSource:
            providerCostUsd == null ? 'catalog_estimate' : 'provider_reported',
          status,
          latencyMs,
          dedup: reservationDedupFinalization(ctx, dedupTerminal),
        }
      : {
          requestId: ctx.requestId,
          outcome: 'release',
          model: ctx.activeModelId,
          provider: ctx.provider,
          tokens,
          costRow: ctx.costRow,
          status,
          latencyMs,
          dedup: reservationDedupFinalization(ctx, dedupTerminal),
        },
  );
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
    log.error('reservation_error_release_failed', {
      request_id: requestId,
      error,
      release_error: releaseError,
    });
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
  user: { id: string; plan: string };
  tier: 'free' | 'pro';
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

export function decideProxyModel(input: {
  plan: string;
  paidCredits: number;
  freeCredits: number;
  role: ExtensionAiModelRole;
  headers: Headers;
}) {
  return decideAccountModel({
    plan: input.plan,
    paidCredits: input.paidCredits,
    freeCredits: input.freeCredits,
    role: input.role,
    forceFree: input.headers.get('x-nevermind-ai-model-tier') === 'free',
  });
}

async function resolveModelRouting(
  slot: ModelRouteSlot,
): Promise<Response | ModelRouting> {
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
        {
          error: {
            type: 'model_not_configured',
            message: 'No active model configured. Admin must set one.',
          },
        },
        { status: 503 },
      );
    }
    throw err;
  }

  const costRow = await lookupModelCost(provider, activeModelId);
  if (!costRow) {
    log.error('pricing_unavailable', { provider, model: activeModelId });
    return Response.json(
      {
        error: {
          type: 'pricing_unavailable',
          message: 'No pricing configured for active model',
        },
      },
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

function serializedByteLength(value: unknown) {
  return value == null ? 0 : Buffer.byteLength(JSON.stringify(value));
}

function aiRequestShape(value: unknown) {
  const body =
    value && typeof value === 'object'
      ? (value as Record<string, unknown>)
      : {};
  const tools = Array.isArray(body.tools) ? body.tools : [];
  return {
    message_bytes: serializedByteLength(body.messages ?? body.contents),
    system_bytes: serializedByteLength(body.system ?? body.system_instruction),
    tool_bytes: serializedByteLength(tools),
    tool_count: tools.length,
  };
}

function logPromptTooLargeEvent(
  request: Request,
  requestId: string,
  client: DesktopClient,
  details: {
    bodyBytes: number;
    estimatedInputTokens: number;
    requestShape: ReturnType<typeof aiRequestShape>;
    reason: 'input_limit' | 'no_output_context';
    contextWindowTokens?: number;
    requestedOutputTokens?: number;
    maxOutputTokens?: number;
  },
) {
  log.warn('prompt_too_large', {
    request_id: requestId,
    route: new URL(request.url).pathname,
    status: 413,
    client_version: client.version,
    client_api_version: client.apiVersion,
    body_bytes: details.bodyBytes,
    estimated_input_tokens: details.estimatedInputTokens,
    max_input_tokens: MAX_INPUT_TOKENS,
    reason: details.reason,
    ...details.requestShape,
    ...(details.contextWindowTokens
      ? {
          context_window_tokens: details.contextWindowTokens,
          output_context_safety_tokens: OUTPUT_CONTEXT_SAFETY_TOKENS,
        }
      : {}),
    ...(details.requestedOutputTokens !== undefined
      ? { requested_output_tokens: details.requestedOutputTokens }
      : {}),
    ...(details.maxOutputTokens !== undefined
      ? { max_output_tokens: details.maxOutputTokens }
      : {}),
  });
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
  const [inserted] = await db
    .insert(requestDedup)
    .values({
      userId,
      idempotencyKey,
      requestHash,
      status: 'in_flight',
      requestId,
    })
    .onConflictDoNothing()
    .returning();

  if (inserted) return undefined;

  const [existing] = await db
    .select()
    .from(requestDedup)
    .where(
      and(
        eq(requestDedup.userId, userId),
        eq(requestDedup.idempotencyKey, idempotencyKey),
      ),
    )
    .limit(1);

  if (!existing) {
    return withRequestId(
      Response.json(
        {
          error: {
            type: 'idempotency_conflict',
            message: 'Idempotency claim is unavailable',
          },
        },
        { status: 409 },
      ),
      requestId,
    );
  }

  if (existing.requestHash !== requestHash) {
    return withRequestId(
      Response.json(
        {
          error: {
            type: 'idempotency_conflict',
            message: 'Idempotency key was used for a different request',
          },
        },
        { status: 409 },
      ),
      requestId,
    );
  }

  if (existing.status === 'completed') {
    if (existing.responseJson !== null) {
      return replayDedupResponse(existing, requestId);
    }
    return withRequestId(
      Response.json(
        {
          error: {
            type: 'idempotency_conflict',
            message: 'Request already processed',
          },
        },
        { status: 409 },
      ),
      requestId,
    );
  }

  if (existing.status === 'in_flight') {
    const createdAt = new Date(existing.createdAt).getTime();
    if (Date.now() - createdAt > DEDUP_STALE_MS) {
      const [reservation] = existing.requestId
        ? await db
            .select({
              status: creditReservations.status,
              expiresAt: creditReservations.expiresAt,
            })
            .from(creditReservations)
            .where(eq(creditReservations.requestId, existing.requestId))
            .limit(1)
        : [];
      if (reservation?.status === 'pending') {
        if (new Date(reservation.expiresAt).getTime() > Date.now()) {
          return withRequestId(
            Response.json(
              {
                error: {
                  type: 'idempotency_conflict',
                  message: 'Request reservation is still active',
                },
              },
              { status: 409 },
            ),
            requestId,
          );
        }
        const release = await finalizeReservation({
          requestId: existing.requestId!,
          outcome: 'release',
          expectedExpiredAtBefore: new Date(),
          dedup: {
            userId,
            idempotencyKey,
            requestHash,
            status: 'failed',
          },
        });
        if (release === 'not_expired') {
          return withRequestId(
            Response.json(
              {
                error: {
                  type: 'idempotency_conflict',
                  message: 'Request reservation is still active',
                },
              },
              { status: 409 },
            ),
            requestId,
          );
        }
        return handleDedup(idempotencyKey, userId, requestHash, requestId);
      }
      const [reclaimed] = await db
        .update(requestDedup)
        .set({
          status: 'in_flight',
          requestId,
          createdAt: new Date(),
          responseJson: null,
          responseHeaders: null,
          upstreamStatus: null,
          completedAt: null,
        })
        .where(
          and(
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
          ),
        )
        .returning();
      if (!reclaimed) {
        return withRequestId(
          Response.json(
            {
              error: {
                type: 'idempotency_conflict',
                message: 'Request already reclaimed',
              },
            },
            { status: 409 },
          ),
          requestId,
        );
      }
      return undefined;
    }
    return withRequestId(
      Response.json(
        {
          error: {
            type: 'idempotency_conflict',
            message: 'Request already in progress',
          },
        },
        { status: 409 },
      ),
      requestId,
    );
  }

  const [reclaimed] = await db
    .update(requestDedup)
    .set({
      status: 'in_flight',
      requestId,
      createdAt: new Date(),
      responseJson: null,
      responseHeaders: null,
      upstreamStatus: null,
      completedAt: null,
    })
    .where(
      and(
        eq(requestDedup.id, existing.id),
        eq(requestDedup.requestId, existing.requestId!),
        eq(requestDedup.requestHash, requestHash),
        eq(requestDedup.status, 'failed'),
      ),
    )
    .returning({ id: requestDedup.id });
  if (reclaimed) return undefined;
  return withRequestId(
    Response.json(
      {
        error: {
          type: 'idempotency_conflict',
          message: 'Request already reclaimed',
        },
      },
      { status: 409 },
    ),
    requestId,
  );
}

function replayDedupResponse(
  existing: typeof requestDedup.$inferSelect,
  requestId: string,
): Response {
  const headers = new Headers();
  if (
    existing.responseHeaders &&
    typeof existing.responseHeaders === 'object'
  ) {
    for (const [key, value] of Object.entries(
      existing.responseHeaders as Record<string, unknown>,
    )) {
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

async function resolveRouting(
  request: Request,
  headerName: PatHeaderName,
): Promise<Response | ResolvedRouting> {
  if (!extractPatFromHeaders(request, headerName)) {
    return new Response('Unauthorized', { status: 401 });
  }
  const user = await getUserFromHeaders(request, headerName);
  if (!user) return new Response('Unauthorized', { status: 401 });
  await ensureMonthlyFreeCredits(user.id);

  const balances = await getBalances(user.id);
  if (balances.total <= 0) {
    return Response.json(
      {
        error: {
          type: 'insufficient_credits',
          message: 'No credits remaining',
          dashboard_url: DASHBOARD_URL,
        },
      },
      { status: 402 },
    );
  }

  const requestedModel =
    parseExtensionAiModelRole(request.headers.get('x-nevermind-ai-model')) ??
    'smart';
  const decision = decideProxyModel({
    plan: user.plan,
    paidCredits: balances.paid,
    freeCredits: balances.free,
    role: requestedModel,
    headers: request.headers,
  });
  const { routeSlot, creditKind: kind, tier } = decision;
  const modelRouting = await resolveModelRouting(routeSlot);
  if (modelRouting instanceof Response) return modelRouting;

  return {
    user,
    tier,
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
  _desktopAuthHeader: PatHeaderName,
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
  return (
    contentType.includes('text/event-stream') || contentType.includes('stream')
  );
}

function withRequestId(res: Response, requestId: string): Response {
  res.headers.set('x-request-id', requestId);
  res.headers.set('x-nevermind-backend-version', backendVersion());
  return res;
}

async function readRequestBodyWithinLimit(
  request: Request,
): Promise<Uint8Array | null> {
  const body = request.body;
  if (!body) return new Uint8Array();

  const reader = body.getReader();
  const chunks: Uint8Array[] = [];
  let totalBytes = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      totalBytes += value.byteLength;
      if (totalBytes > MAX_PROXY_REQUEST_BODY_BYTES) {
        await reader.cancel();
        return null;
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }

  const bytes = new Uint8Array(totalBytes);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return bytes;
}

function requestTooLargeResponse(requestId: string) {
  return withRequestId(
    Response.json(
      {
        error: {
          type: 'request_too_large',
          message: 'AI request bodies must be 4 MB or smaller.',
        },
      },
      { status: 413 },
    ),
    requestId,
  );
}

function chainExhaustedResponse(requestId: string): Response {
  return withRequestId(
    Response.json(
      {
        error: {
          type: 'upstream_unavailable',
          message: 'All configured upstream providers are unavailable',
        },
      },
      { status: 503 },
    ),
    requestId,
  );
}

async function hasConfiguredImageRoute(
  routing: ResolvedRouting,
  primaryDescriptor: ModelDescriptor | null,
) {
  const apiFormat = selectApiForModel(routing.provider, routing.activeModelId);
  let configuredFallbacks: Array<{ providerId: string; modelId: string }> = [];
  if (!backendKillSwitchEnabled('ai_failover')) {
    try {
      configuredFallbacks = await getModelProviderRoutes(
        routing.routeSlot,
        routing.activeModelId,
      );
    } catch (error) {
      log.warn('provider_chain_fetch_failed', {
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  const candidates = [
    { providerId: routing.provider, modelId: routing.activeModelId },
    ...configuredFallbacks,
  ];
  for (const candidate of candidates) {
    if (
      !providerSupportsFormat(candidate.providerId, apiFormat) ||
      selectApiForModel(candidate.providerId, candidate.modelId) !== apiFormat
    )
      continue;
    const descriptor =
      candidate.providerId === routing.provider &&
      candidate.modelId === routing.activeModelId
        ? primaryDescriptor
        : await lookupModelDescriptor(candidate.providerId, candidate.modelId);
    const reviewedLimits = reviewedImageTokenLimits(
      candidate.providerId,
      candidate.modelId,
    );
    if (
      reviewedLimits &&
      supportsBoundedImageInput(
        candidate.providerId,
        candidate.modelId,
        descriptor?.input,
      )
    )
      return true;
  }
  return false;
}

async function tryUpstreamProviders(
  cfg: ProxyConfig,
  routing: ResolvedRouting,
  requestBody: BodyInit | undefined,
  requestBodyText: string,
  estimatedInputTokens: number,
  reservedCredits: number | undefined,
  maxOutputTokens: number | undefined,
  requestedOutputTokens: number | undefined,
  imageCount: number,
  completionCount: number,
  primaryDescriptor: ModelDescriptor | null,
  requestId: string,
): Promise<
  | {
      response: Response;
      provider: string;
      modelId: string;
      costRow: ModelCost;
      reservedCredits: number | undefined;
      estimatedInputTokens: number;
    }
  | {
      failure: Response;
      provider: string;
      modelId: string;
      costRow: ModelCost;
      status: number;
    }
> {
  const apiFormat = selectApiForModel(routing.provider, routing.activeModelId);
  const failoverEnabled = !backendKillSwitchEnabled('ai_failover');

  let chainProviders: Array<{ providerId: string; modelId: string }> = [];
  if (failoverEnabled) {
    try {
      chainProviders = await getModelProviderRoutes(
        routing.routeSlot,
        routing.activeModelId,
      );
    } catch (err) {
      log.warn('provider_chain_fetch_failed', {
        request_id: requestId,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }

  const configuredProviderChain = [
    { providerId: routing.provider, modelId: routing.activeModelId },
    ...chainProviders.filter(
      (route) =>
        route.providerId !== routing.provider ||
        route.modelId !== routing.activeModelId,
    ),
  ];
  const providerChain = await healthyRouteCandidates(
    routing.routeSlot,
    configuredProviderChain,
  );
  let lastFailure: { provider: string; modelId: string; costRow: ModelCost } = {
    provider: routing.provider,
    modelId: routing.activeModelId,
    costRow: routing.costRow,
  };
  let creditFailure: {
    response: Response;
    provider: string;
    modelId: string;
    costRow: ModelCost;
  } | null = null;
  let currentReservedCredits = reservedCredits;
  let imageBoundRouteFound = false;
  let imageBoundRouteRejected = false;

  for (const [attemptIndex, candidate] of providerChain.entries()) {
    const { providerId, modelId } = candidate;
    const candidateIsPrimary =
      providerId === routing.provider && modelId === routing.activeModelId;
    if (
      !providerSupportsFormat(providerId, apiFormat) ||
      selectApiForModel(providerId, modelId) !== apiFormat
    ) {
      log.info('upstream_format_skip', {
        request_id: requestId,
        provider: providerId,
        model: modelId,
        format: apiFormat,
      });
      continue;
    }

    let upstreamCfg;
    try {
      upstreamCfg = getUpstreamConfig(providerId);
    } catch (err) {
      log.warn('upstream_config_skip', {
        request_id: requestId,
        provider: providerId,
        error: err instanceof Error ? err.message : String(err),
      });
      continue;
    }

    const costRow =
      providerId === routing.provider && modelId === routing.activeModelId
        ? routing.costRow
        : await lookupModelCost(providerId, modelId);
    if (!costRow) {
      log.warn('upstream_cost_skip', {
        request_id: requestId,
        provider: providerId,
        model: modelId,
      });
      continue;
    }
    lastFailure = { provider: providerId, modelId, costRow };

    const descriptor = candidateIsPrimary
      ? primaryDescriptor
      : await lookupModelDescriptor(providerId, modelId);
    const reviewedLimits =
      imageCount > 0
        ? reviewedImageTokenLimits(providerId, modelId)
        : undefined;
    if (
      imageCount > 0 &&
      (!reviewedLimits ||
        !supportsBoundedImageInput(providerId, modelId, descriptor?.input))
    ) {
      imageBoundRouteRejected = true;
      log.info('upstream_image_bound_skip', {
        request_id: requestId,
        provider: providerId,
        model: modelId,
      });
      continue;
    }
    const candidateInputTokenCeiling =
      imageCount === 0
        ? estimatedInputTokens
        : descriptor && reviewedLimits
          ? conservativeImageRequestInputTokenCeiling(
              reviewedLimits,
              requestedOutputTokens,
              estimatedInputTokens,
            )
          : undefined;
    if (candidateInputTokenCeiling === undefined) {
      imageBoundRouteRejected = true;
      log.info('upstream_image_context_skip', {
        request_id: requestId,
        provider: providerId,
        model: modelId,
      });
      continue;
    }
    if (imageCount > 0) imageBoundRouteFound = true;
    const candidateMaxOutputTokens =
      !candidateIsPrimary && descriptor
        ? Math.min(
            requestedOutputTokens ??
              reviewedLimits?.maxTokens ??
              descriptor.maxTokens,
            reviewedLimits?.maxTokens ?? descriptor.maxTokens,
            Math.max(
              0,
              (reviewedLimits?.contextWindow ?? descriptor.contextWindow) -
                candidateInputTokenCeiling -
                OUTPUT_CONTEXT_SAFETY_TOKENS,
            ),
          )
        : maxOutputTokens;
    if (candidateMaxOutputTokens != null && candidateMaxOutputTokens < 1) {
      log.warn('upstream_context_skip', {
        request_id: requestId,
        provider: providerId,
        model: modelId,
      });
      continue;
    }
    const candidateCostUsd =
      candidateMaxOutputTokens == null
        ? undefined
        : requestReservationCostUsd(
            costRow,
            candidateInputTokenCeiling,
            candidateMaxOutputTokens,
            imageCount,
            completionCount,
            reviewedLimits,
          );
    const candidateCredits =
      candidateCostUsd == null ? undefined : usdToCredits(candidateCostUsd);
    if (
      candidateCredits != null &&
      candidateCostUsd != null &&
      currentReservedCredits != null &&
      !candidateIsPrimary
    ) {
      const resized = await resizeReservation(
        requestId,
        candidateCredits,
        candidateCostUsd,
        'catalog_estimate',
      );
      if (!resized.ok && resized.reason === 'free_spend_cap_reached') {
        creditFailure = {
          response: withRequestId(
            Response.json(freeSpendCapErrorBody(), { status: 503 }),
            requestId,
          ),
          ...lastFailure,
        };
        if (attemptIndex === providerChain.length - 1) {
          return {
            failure: creditFailure.response,
            provider: creditFailure.provider,
            modelId: creditFailure.modelId,
            costRow: creditFailure.costRow,
            status: 503,
          };
        }
        continue;
      }
      if (!resized.ok && resized.reason === 'insufficient_credits') {
        log.warn('upstream_credit_skip', {
          request_id: requestId,
          provider: providerId,
          model: modelId,
        });
        creditFailure = {
          response: withRequestId(
            Response.json(
              {
                error: {
                  type: 'insufficient_credits',
                  message: 'Failover cost would exceed remaining balance',
                  balance: resized.balance - resized.reserved,
                  dashboard_url: DASHBOARD_URL,
                },
              },
              { status: 402 },
            ),
            requestId,
          ),
          ...lastFailure,
        };
        if (attemptIndex === providerChain.length - 1) {
          return {
            failure: creditFailure.response,
            provider: creditFailure.provider,
            modelId: creditFailure.modelId,
            costRow: creditFailure.costRow,
            status: 402,
          };
        }
        continue;
      }
      if (resized.ok)
        currentReservedCredits = resized.reservation.reservedCredits;
    }

    const candidateRouting: ModelRouting = {
      provider: providerId,
      activeModelId: modelId,
      thinkingLevel: routing.thinkingLevel,
      costRow,
      upstreamBaseUrl: upstreamCfg.baseUrl,
      upstreamApiKey: upstreamCfg.apiKey,
    };
    const forwardBody =
      cfg.rewriteRequestBody && candidateMaxOutputTokens != null
        ? cfg.rewriteRequestBody(
            requestBodyText,
            candidateRouting,
            candidateMaxOutputTokens,
          )
        : requestBody;

    const upstreamUrl = cfg.buildUpstreamUrl({
      upstreamBaseUrl: upstreamCfg.baseUrl,
      activeModelId: modelId,
    });

    const headers = buildForwardHeaders(
      cfg.request,
      cfg.authHeaderName,
      cfg.upstreamAuthHeaderName,
      cfg.formatUpstreamAuthValue(upstreamCfg.apiKey),
    );

    let resp;
    creditFailure = null;
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

    const isLast = attemptIndex === providerChain.length - 1;

    if (resp.status >= 500) {
      log.warn('upstream_5xx', {
        request_id: requestId,
        provider: providerId,
        status: resp.status,
      });
      if (!isLast) continue;
    }

    log.info('upstream_selected', {
      request_id: requestId,
      provider: providerId,
      status: resp.status,
    });
    return {
      response: resp,
      provider: providerId,
      modelId,
      costRow,
      reservedCredits: currentReservedCredits,
      estimatedInputTokens,
    };
  }

  if (imageCount > 0 && !imageBoundRouteFound && imageBoundRouteRejected) {
    return {
      failure: withRequestId(
        Response.json(
          {
            error: {
              type: 'unsupported_request_shape',
              message:
                'Images are not available for this model route because no configured provider has a reviewed image billing and request-format contract.',
            },
          },
          { status: 400 },
        ),
        requestId,
      ),
      ...lastFailure,
      status: 400,
    };
  }

  if (creditFailure) {
    return {
      failure: creditFailure.response,
      provider: creditFailure.provider,
      modelId: creditFailure.modelId,
      costRow: creditFailure.costRow,
      status: 402,
    };
  }
  return {
    failure: chainExhaustedResponse(requestId),
    ...lastFailure,
    status: 503,
  };
}

export async function proxyAndBill(cfg: ProxyConfig): Promise<Response> {
  let requestId: string = randomUUID();
  if (backendKillSwitchEnabled('ai_proxy'))
    return killSwitchResponse(
      'ai_proxy',
      'AI proxy is temporarily disabled.',
      requestId,
    );
  const declaredBodyBytes = Number(cfg.request.headers.get('content-length'));
  if (
    Number.isFinite(declaredBodyBytes) &&
    declaredBodyBytes > MAX_PROXY_REQUEST_BODY_BYTES
  )
    return requestTooLargeResponse(requestId);
  const startedAt = Date.now();
  const client = desktopClientFromRequest(cfg.request);
  Sentry.getCurrentScope().setTag('request_id', requestId);
  if (client.version)
    Sentry.getCurrentScope().setTag('client_version', client.version);
  if (client.apiVersion)
    Sentry.getCurrentScope().setTag(
      'client_api_version',
      String(client.apiVersion),
    );
  let routing = await resolveRouting(cfg.request, cfg.authHeaderName);
  if (routing instanceof Response) return withRequestId(routing, requestId);
  const routingFinishedAt = Date.now();

  const requestBodyBytes = await readRequestBodyWithinLimit(cfg.request);
  if (!requestBodyBytes) return requestTooLargeResponse(requestId);
  const requestBodyText = new TextDecoder().decode(requestBodyBytes);
  const requestHash = aiRequestHash(cfg.request, requestBodyBytes);
  const requestPreparedAt = Date.now();

  const idempotencyKey = cfg.idempotencyKey;
  const dedupEnabled =
    idempotencyKey && !backendKillSwitchEnabled('idempotency_dedup');
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
  const dedupFinishedAt = Date.now();

  let reservationCommitted = false;
  try {
    const rateDecision = await rateLimitChat(routing.user.id, routing.kind);
    const rateLimitFinishedAt = Date.now();
    if (!rateDecision.ok) {
      log.warn('rate_limited', {
        request_id: requestId,
        user_id: routing.user.id,
        scope: rateDecision.scope,
        client_version: client.version,
        client_api_version: client.apiVersion,
      });
      await markDedupFailed(dedupClaim);
      return withRequestId(tooManyRequests(rateDecision), requestId);
    }

    let forwardBody: BodyInit | undefined;
    let maxOutputTokens: number | undefined;
    let requestedOutput: number | undefined;
    let completionCount = 1;
    let estimatedInputTokens = 0;
    let reservationInputTokenCeiling = 0;
    let estimatedCredits: number | undefined;
    let requestShape = aiRequestShape(null);
    let imageCount = 0;
    let routingDescriptor: ModelDescriptor | null = null;
    if (cfg.request.method !== 'GET' && cfg.request.method !== 'HEAD') {
      let parsedBody: unknown = null;
      let bodyIsJson = false;
      try {
        parsedBody = JSON.parse(requestBodyText);
        bodyIsJson = true;
      } catch {
        bodyIsJson = false;
      }
      const parsedImageInputs = analyzeAiImageInputs(
        bodyIsJson ? parsedBody : requestBodyText,
      );
      imageCount = parsedImageInputs.imageCount;
      const unsupportedMultimodal =
        parsedImageInputs.invalid ||
        (bodyIsJson &&
          containsUnsupportedMultimodalPayload(
            parsedBody,
            parsedImageInputs,
          )) ||
        (!bodyIsJson && containsUnsupportedMultimodalPayload(requestBodyText));
      if (unsupportedMultimodal) {
        await markDedupFailed(dedupClaim);
        return withRequestId(
          Response.json(
            {
              error: {
                type: 'unsupported_request_shape',
                message: parsedImageInputs.invalid
                  ? 'Only inline, non-animated PNG, JPEG, WebP, or GIF images up to 2048 × 2048 pixels and 2 MB each (2 MB total) are supported.'
                  : 'Multimodal requests are unavailable because this route has no conservative modality-cost bound.',
              },
            },
            { status: 400 },
          ),
          requestId,
        );
      }
      if (bodyIsJson && containsUnpricedProviderTool(parsedBody)) {
        await markDedupFailed(dedupClaim);
        return withRequestId(
          Response.json(
            {
              error: {
                type: 'unsupported_request_shape',
                message:
                  'OpenRouter web search is unavailable because its per-search charge is outside Nevermind’s token reservation.',
              },
            },
            { status: 400 },
          ),
          requestId,
        );
      }
      const requestedChoices = requestedCompletionCount(parsedBody);
      if (requestedChoices === undefined) {
        await markDedupFailed(dedupClaim);
        return withRequestId(
          Response.json(
            {
              error: {
                type: 'unsupported_request_shape',
                message: `The n field must be an integer from 1 to ${MAX_CHAT_COMPLETION_CHOICES}.`,
              },
            },
            { status: 400 },
          ),
          requestId,
        );
      }
      completionCount = requestedChoices;
      estimatedInputTokens = bodyIsJson
        ? estimateInputTokensFromValue(parsedBody)
        : estimateInputTokensFromBody(requestBodyText);
      requestShape = aiRequestShape(parsedBody);
      if (estimatedInputTokens > MAX_INPUT_TOKENS) {
        logPromptTooLargeEvent(cfg.request, requestId, client, {
          bodyBytes: requestBodyBytes.byteLength,
          estimatedInputTokens,
          requestShape,
          reason: 'input_limit',
        });
        await markDedupFailed(dedupClaim);
        return withRequestId(
          Response.json(
            {
              error: {
                type: 'prompt_too_large',
                message: `Prompt exceeds ${MAX_INPUT_TOKENS} input tokens`,
              },
            },
            { status: 413 },
          ),
          requestId,
        );
      }
      requestedOutput = requestedMaxOutputTokens(parsedBody);
      let contextWindowTokens: number | undefined;
      const maxOutputFor = async (candidate: ModelRouting) => {
        const descriptor = await lookupModelDescriptor(
          candidate.provider,
          candidate.activeModelId,
        );
        if (
          candidate.provider === routing.provider &&
          candidate.activeModelId === routing.activeModelId
        )
          routingDescriptor = descriptor;
        const reviewedLimits =
          imageCount > 0
            ? reviewedImageTokenLimits(
                candidate.provider,
                candidate.activeModelId,
              )
            : undefined;
        contextWindowTokens =
          reviewedLimits?.contextWindow ?? descriptor?.contextWindow;
        const serverMaximum =
          reviewedLimits?.maxTokens ?? descriptor?.maxTokens ?? 32_000;
        const desiredOutputMaximum = Math.min(
          requestedOutput ?? serverMaximum,
          serverMaximum,
        );
        const imageInputTokenCeiling =
          imageCount > 0 &&
          descriptor &&
          reviewedLimits &&
          supportsBoundedImageInput(
            candidate.provider,
            candidate.activeModelId,
            descriptor.input,
          )
            ? conservativeImageRequestInputTokenCeiling(
                reviewedLimits,
                desiredOutputMaximum,
                estimatedInputTokens,
              )
            : undefined;
        const inputTokensForReservation =
          imageInputTokenCeiling ?? estimatedInputTokens;
        const contextMaximum = descriptor
          ? (reviewedLimits?.contextWindow ?? descriptor.contextWindow) -
            inputTokensForReservation -
            OUTPUT_CONTEXT_SAFETY_TOKENS
          : serverMaximum;
        return {
          descriptor,
          inputTokensForReservation,
          maxOutputTokens: Math.min(
            desiredOutputMaximum,
            Math.max(0, contextMaximum),
          ),
        };
      };
      const initialAdmission = await maxOutputFor(routing);
      routingDescriptor = initialAdmission.descriptor;
      maxOutputTokens = initialAdmission.maxOutputTokens;
      reservationInputTokenCeiling = initialAdmission.inputTokensForReservation;
      const initialReviewedLimits =
        imageCount > 0
          ? reviewedImageTokenLimits(routing.provider, routing.activeModelId)
          : undefined;
      if (
        imageCount > 0 &&
        !(await hasConfiguredImageRoute(routing, routingDescriptor))
      ) {
        await markDedupFailed(dedupClaim);
        return withRequestId(
          Response.json(
            {
              error: {
                type: 'unsupported_request_shape',
                message:
                  'Images are not available for this model route because no configured provider has a reviewed image billing and request-format contract.',
              },
            },
            { status: 400 },
          ),
          requestId,
        );
      }
      if (maxOutputTokens < 1) {
        logPromptTooLargeEvent(cfg.request, requestId, client, {
          bodyBytes: requestBodyBytes.byteLength,
          estimatedInputTokens,
          requestShape,
          reason: 'no_output_context',
          contextWindowTokens,
          requestedOutputTokens: requestedOutput,
          maxOutputTokens,
        });
        await markDedupFailed(dedupClaim);
        return withRequestId(
          Response.json(
            {
              error: {
                type: 'prompt_too_large',
                message: 'Prompt leaves no room for model output',
              },
            },
            { status: 413 },
          ),
          requestId,
        );
      }
      estimatedCredits = requestReservationCredits(
        routing.costRow,
        reservationInputTokenCeiling,
        maxOutputTokens,
        imageCount,
        completionCount,
        initialReviewedLimits,
      );
      const estimatedProviderCostUsd = requestReservationCostUsd(
        routing.costRow,
        reservationInputTokenCeiling,
        maxOutputTokens,
        imageCount,
        completionCount,
        initialReviewedLimits,
      );
      let reservation = await reserveCredits({
        requestId,
        userId: routing.user.id,
        kind: routing.kind,
        credits: estimatedCredits,
        estimatedProviderCostUsd,
        estimateCostSource: 'catalog_estimate',
      });
      if (
        !reservation.ok &&
        reservation.reason === 'insufficient_credits' &&
        routing.tier === 'pro'
      ) {
        const requestedModel: ExtensionAiModelRole =
          parseExtensionAiModelRole(
            cfg.request.headers.get('x-nevermind-ai-model'),
          ) ?? 'smart';
        const freeDecision = decideAccountModel({
          plan: routing.user.plan,
          paidCredits: 0,
          freeCredits: routing.freeBalanceAvailable,
          role: requestedModel,
          forceFree: true,
        });
        const freeRouting = await resolveModelRouting(freeDecision.routeSlot);
        if (freeRouting instanceof Response) {
          await markDedupFailed(dedupClaim);
          return withRequestId(freeRouting, requestId);
        }
        if (
          selectApiForModel(freeRouting.provider, freeRouting.activeModelId) ===
          selectApiForModel(routing.provider, routing.activeModelId)
        ) {
          const freeDescriptor = await lookupModelDescriptor(
            freeRouting.provider,
            freeRouting.activeModelId,
          );
          const freeImageRouteSupported =
            imageCount === 0 ||
            (await hasConfiguredImageRoute(freeRouting, freeDescriptor));
          const freeAdmission = freeImageRouteSupported
            ? await maxOutputFor(freeRouting)
            : null;
          const freeMaxOutputTokens = freeAdmission?.maxOutputTokens ?? 0;
          if (freeMaxOutputTokens > 0) {
            const freeEstimatedCredits = requestReservationCredits(
              freeRouting.costRow,
              freeAdmission?.inputTokensForReservation ?? estimatedInputTokens,
              freeMaxOutputTokens,
              imageCount,
              completionCount,
              freeImageRouteSupported && imageCount > 0
                ? reviewedImageTokenLimits(
                    freeRouting.provider,
                    freeRouting.activeModelId,
                  )
                : undefined,
            );
            const freeReservation = await reserveCredits({
              requestId,
              userId: routing.user.id,
              kind: 'free',
              credits: freeEstimatedCredits,
              estimatedProviderCostUsd: requestReservationCostUsd(
                freeRouting.costRow,
                freeAdmission?.inputTokensForReservation ??
                  estimatedInputTokens,
                freeMaxOutputTokens,
                imageCount,
                completionCount,
                freeImageRouteSupported && imageCount > 0
                  ? reviewedImageTokenLimits(
                      freeRouting.provider,
                      freeRouting.activeModelId,
                    )
                  : undefined,
              ),
              estimateCostSource: 'catalog_estimate',
            });
            if (freeReservation.ok) {
              maxOutputTokens = freeMaxOutputTokens;
              routing = {
                user: routing.user,
                tier: 'free',
                routeSlot: freeDecision.routeSlot,
                ...freeRouting,
                kind: 'free',
                balanceAvailable:
                  freeReservation.balance - freeReservation.reserved,
                freeBalanceAvailable:
                  freeReservation.balance - freeReservation.reserved,
              };
              routingDescriptor = freeAdmission?.descriptor ?? null;
              reservationInputTokenCeiling =
                freeAdmission?.inputTokensForReservation ??
                estimatedInputTokens;
              estimatedCredits = freeEstimatedCredits;
              reservation = freeReservation;
            } else if (freeReservation.reason === 'free_spend_cap_reached') {
              reservation = freeReservation;
            }
          }
        }
      }
      if (
        !reservation.ok &&
        reservation.reason === 'request_already_reserved'
      ) {
        await markDedupFailed(dedupClaim);
        return withRequestId(
          Response.json(
            {
              error: {
                type: 'idempotency_conflict',
                message:
                  'Request execution was already finalized or is still in progress',
              },
            },
            { status: 409 },
          ),
          requestId,
        );
      }
      if (!reservation.ok && reservation.reason === 'free_spend_cap_reached') {
        await markDedupFailed(dedupClaim);
        return withRequestId(
          Response.json(freeSpendCapErrorBody(), { status: 503 }),
          requestId,
        );
      }
      if (!reservation.ok) {
        const available = reservation.balance - reservation.reserved;
        await markDedupFailed(dedupClaim);
        return withRequestId(
          Response.json(
            {
              error: {
                type: 'insufficient_credits',
                message: 'Request cost would exceed remaining balance',
                estimated_credits: estimatedCredits,
                balance: available,
                dashboard_url: DASHBOARD_URL,
              },
            },
            { status: 402 },
          ),
          requestId,
        );
      }
      reservationCommitted = true;
      log.info('proxy_preflight', {
        request_id: requestId,
        routing_ms: routingFinishedAt - startedAt,
        request_prepare_ms: requestPreparedAt - routingFinishedAt,
        dedup_ms: dedupFinishedAt - requestPreparedAt,
        rate_limit_ms: rateLimitFinishedAt - dedupFinishedAt,
        admission_ms: Date.now() - rateLimitFinishedAt,
        latency_ms: Date.now() - startedAt,
        body_bytes: requestBodyBytes.byteLength,
        estimated_input_tokens: estimatedInputTokens,
        ...requestShape,
      });
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
        ? cfg.rewriteRequestBody(requestBodyText, routing, maxOutputTokens)
        : requestBodyBytes;
    }

    const result = await tryUpstreamProviders(
      cfg,
      routing,
      forwardBody,
      requestBodyText,
      estimatedInputTokens,
      estimatedCredits,
      maxOutputTokens,
      requestedOutput,
      imageCount,
      completionCount,
      routingDescriptor,
      requestId,
    );
    if ('failure' in result) {
      if (result.status >= 500)
        Sentry.captureMessage('ai_chain_exhausted', 'error');
      await recordUsage(
        {
          user: routing.user,
          provider: result.provider,
          routeSlot: routing.routeSlot,
          activeModelId: result.modelId,
          costRow: result.costRow,
          kind: routing.kind,
          requestId,
          client,
          estimatedInputTokens,
          dedupIdempotencyKey: dedupEnabled ? idempotencyKey : undefined,
          dedupRequestHash: dedupEnabled ? requestHash : undefined,
        },
        { inputTokens: 0, outputTokens: 0 },
        result.status,
        Date.now() - startedAt,
        { status: 'failed', upstreamStatus: result.status },
      );
      return result.failure;
    }

    const upstreamResponse = result.response;
    const winningProvider = result.provider;
    const winningModelId = result.modelId;
    const winningCostRow = result.costRow;

    const billCtx: BillContext = {
      user: routing.user,
      provider: winningProvider,
      routeSlot: routing.routeSlot,
      activeModelId: winningModelId,
      costRow: winningCostRow,
      kind: routing.kind,
      requestId,
      client,
      estimatedInputTokens: result.estimatedInputTokens,
      reservationCredits: result.reservedCredits,
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
        {
          status: 'failed',
          upstreamStatus: upstreamResponse.status,
        },
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
        return killSwitchResponse(
          'ai_streaming',
          'AI streaming is temporarily disabled.',
          requestId,
        );
      }
      const transformed = teeStreamAndBill(
        upstreamResponse,
        cfg,
        billCtx,
        upstreamResponse.status,
        startedAt,
      );
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
    responseHeaders.forEach((value, key) => {
      headersObj[key] = value;
    });
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
      await markDedupFailed(dedupClaim).catch((dedupError) =>
        log.error('dedup_failed_update_failed', {
          request_id: requestId,
          error: dedupError,
        }),
      );
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
    log.warn('stream_usage_frame_parse_failed', {
      request_id: billCtx.requestId,
      error,
    });
  }

  function sniffStreamUsage(text: string, finalize = false) {
    try {
      cfg.parseUsageFromStreamChunk(text, acc, finalize);
    } catch (error) {
      log.warn('usage_sniffer_failed', {
        request_id: billCtx.requestId,
        error,
      });
    }
  }

  const acc: StreamUsageAccumulator = {
    inputTokens: 0,
    outputTokens: 0,
    cachedInputTokens: 0,
    cacheWriteInputTokens: 0,
    reasoningTokens: 0,
    observedOutputCharacters: 0,
    finalized: false,
    reportMalformedFrame: reportMalformedStreamUsageFrame,
  };
  const decoder = new TextDecoder('utf-8', { fatal: false });
  let terminal = false;
  let loggedFirstChunk = false;
  async function finish(
    naturalCompletion: boolean,
    outcome: 'completed' | 'failed' | 'cancelled' = naturalCompletion
      ? 'completed'
      : 'failed',
  ) {
    if (terminal) return;
    terminal = true;
    log.info('diagnostic_stream_terminal', {
      request_id: billCtx.requestId,
      diagnostic_outcome: outcome,
    });
    if (naturalCompletion) {
      sniffStreamUsage(decoder.decode(), true);
    }
    const observedOutput =
      acc.outputTokens > 0 || (acc.observedOutputCharacters ?? 0) > 0;
    const observedOutputTokens =
      acc.outputTokens > 0
        ? acc.outputTokens
        : Math.max(0, Math.ceil((acc.observedOutputCharacters ?? 0) / 4));
    const streamTokens =
      naturalCompletion || observedOutput
        ? resolveBillableTokens(
            billCtx,
            {
              inputTokens: acc.inputTokens,
              outputTokens: observedOutputTokens,
              cachedInputTokens: acc.cachedInputTokens,
              cacheWriteInputTokens: acc.cacheWriteInputTokens,
              reasoningTokens: acc.reasoningTokens,
              providerCostUsd: acc.providerCostUsd,
            },
            status,
          )
        : {
            inputTokens: acc.inputTokens,
            outputTokens: observedOutputTokens,
            cachedInputTokens: acc.cachedInputTokens,
            cacheWriteInputTokens: acc.cacheWriteInputTokens,
            reasoningTokens: acc.reasoningTokens,
            providerCostUsd: acc.providerCostUsd,
          };
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

  function finishNaturalStreamInBackground() {
    const finalization = finish(true).catch((error) => {
      captureStreamFailure(error, billCtx.requestId, 'complete');
      log.error('stream_finalize_failed', {
        request_id: billCtx.requestId,
        error,
      });
    });
    waitUntil(finalization);
  }

  const reader = upstreamResponse.body!.getReader();
  return new ReadableStream<Uint8Array>({
    async pull(controller) {
      try {
        const { value, done } = await reader.read();
        if (done) {
          finishNaturalStreamInBackground();
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
          controller.enqueue(value);
        }
      } catch (error) {
        if (!terminal) captureStreamFailure(error, billCtx.requestId);
        await finish(false).catch((finalizeError) =>
          log.error('stream_finalize_failed', {
            request_id: billCtx.requestId,
            error: finalizeError,
          }),
        );
        controller.error(error);
      }
    },
    async cancel(reason) {
      // Claim the terminal transition before cancelling the upstream reader:
      // reader.cancel() resolves a pending read as done, which must not race
      // cancellation into the natural-completion billing policy.
      const finalization = finish(false, 'cancelled').catch((error) =>
        log.error('stream_cancel_finalize_failed', {
          request_id: billCtx.requestId,
          error,
        }),
      );
      await reader.cancel(reason).catch(() => undefined);
      await finalization;
    },
  });
}

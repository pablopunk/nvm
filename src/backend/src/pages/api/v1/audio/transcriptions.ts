import { createHash } from 'node:crypto';
import type { APIRoute } from 'astro';
import { z } from 'zod';
import { PRODUCTION_WEB_ORIGIN } from '../../../../../../app/shared/public-origin';
import {
  finalizeReservation,
  reserveCredits,
} from '../../../../lib/credit-reservations';
import {
  backendKillSwitchEnabled,
  compatibilityError,
  compatibilityFeaturesForClient,
  compatibilityHeaders,
  desktopClientFromRequest,
  killSwitchResponse,
  requestIdFromHeaders,
  unsupportedClientReason,
} from '../../../../lib/compatibility';
import { log } from '../../../../lib/log';
import { markDedupFailed, handleDedup } from '../../../../lib/proxy';
import {
  rateLimitTranscription,
  tooManyRequests,
} from '../../../../lib/ratelimit';
import {
  getAudioModelRoute,
  isProviderEnabled,
  ModelNotConfiguredError,
} from '../../../../lib/settings';
import { getUserFromBearer } from '../../../../lib/tokens';
import { isCompatibleTranscriptionModel } from '../../../../lib/transcription-models';
import { getUpstreamConfig, UpstreamConfigError } from '../../../../lib/upstream';
import { ensureMonthlyFreeCredits, getBalances } from '../../../../lib/users';

export const config = { maxDuration: 60 };

const MAX_AUDIO_BYTES = Math.max(
  64 * 1024,
  Number(process.env.AUDIO_TRANSCRIPTION_MAX_BYTES ?? 4 * 1024 * 1024),
);
const MAX_REQUEST_BYTES = Math.ceil((MAX_AUDIO_BYTES * 4) / 3) + 8 * 1024;
const RESERVATION_CREDITS = Math.max(
  1,
  Number(process.env.AUDIO_TRANSCRIPTION_RESERVATION_CREDITS ?? 5),
);
const MAX_PROVIDER_COST_USD = Math.max(
  0.001,
  Number(process.env.AUDIO_TRANSCRIPTION_MAX_COST_USD ?? 0.1),
);
const UPSTREAM_TIMEOUT_MS = Math.max(
  1_000,
  Number(process.env.AUDIO_TRANSCRIPTION_TIMEOUT_MS ?? 15_000),
);
const DASHBOARD_URL = `${PRODUCTION_WEB_ORIGIN}/dashboard`;

const requestSchema = z
  .object({
    input_audio: z
      .object({
        data: z.string().min(1),
        format: z.literal('webm'),
      })
      .strict(),
  })
  .strict();

type OpenRouterTranscription = {
  text?: unknown;
  usage?: {
    seconds?: unknown;
    total_tokens?: unknown;
    input_tokens?: unknown;
    output_tokens?: unknown;
    cost?: unknown;
  };
};

function finiteNonnegative(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
    ? value
    : null;
}

function decodeBase64Audio(data: string) {
  if (!/^[A-Za-z0-9+/]*={0,2}$/.test(data) || data.length % 4 !== 0)
    return null;
  const bytes = Buffer.from(data, 'base64');
  if (!bytes.length || bytes.length > MAX_AUDIO_BYTES) return null;
  if (bytes.subarray(0, 4).toString('hex') !== '1a45dfa3') return null;
  return bytes;
}

function responseWithRequestId(body: unknown, status: number, requestId: string) {
  return Response.json(body, {
    status,
    headers: compatibilityHeaders(requestId),
  });
}

async function releaseAfterFailure(input: {
  requestId: string;
  userId: string;
  idempotencyKey: string;
  requestHash: string;
  model: string;
  status: number;
  startedAt: number;
}) {
  await finalizeReservation({
    requestId: input.requestId,
    outcome: 'release',
    model: input.model,
    provider: 'openrouter',
    modality: 'audio',
    status: input.status,
    latencyMs: Math.round(performance.now() - input.startedAt),
    dedup: {
      userId: input.userId,
      idempotencyKey: input.idempotencyKey,
      requestHash: input.requestHash,
      status: 'failed',
    },
  });
}

export const POST: APIRoute = async ({ request }) => {
  const startedAt = performance.now();
  const requestId = requestIdFromHeaders(request.headers);
  const client = desktopClientFromRequest(request);
  if (unsupportedClientReason(client)) return compatibilityError(request);
  const user = await getUserFromBearer(request.headers.get('authorization'));
  if (!user)
    return responseWithRequestId(
      { error: { type: 'unauthorized', message: 'Sign in to use API dictation.' } },
      401,
      requestId,
    );
  const features = compatibilityFeaturesForClient(client, {
    userId: user.id,
    plan: user.plan,
    requestId,
    route: '/api/v1/audio/transcriptions',
  });
  if (features.dictation_transcription_api !== true)
    return responseWithRequestId(
      {
        error: {
          type: 'feature_unavailable',
          message: 'API dictation is not available.',
        },
      },
      503,
      requestId,
    );
  if (backendKillSwitchEnabled('audio_transcription'))
    return killSwitchResponse(
      'audio_transcription',
      'API dictation is temporarily unavailable.',
      requestId,
    );
  const contentLength = Number(request.headers.get('content-length') ?? 0);
  if (contentLength > MAX_REQUEST_BYTES)
    return responseWithRequestId(
      { error: { type: 'payload_too_large', message: 'Dictation audio is too large.' } },
      413,
      requestId,
    );
  const rawBody = await request.text();
  if (Buffer.byteLength(rawBody) > MAX_REQUEST_BYTES)
    return responseWithRequestId(
      { error: { type: 'payload_too_large', message: 'Dictation audio is too large.' } },
      413,
      requestId,
    );
  let json: unknown;
  try {
    json = JSON.parse(rawBody);
  } catch {
    return responseWithRequestId(
      { error: { type: 'invalid_request', message: 'Invalid JSON body.' } },
      400,
      requestId,
    );
  }
  const parsed = requestSchema.safeParse(json);
  if (!parsed.success)
    return responseWithRequestId(
      { error: { type: 'invalid_request', message: 'Invalid transcription request.' } },
      400,
      requestId,
    );
  const audio = decodeBase64Audio(parsed.data.input_audio.data);
  if (!audio)
    return responseWithRequestId(
      { error: { type: 'invalid_audio', message: 'Invalid or oversized WebM audio.' } },
      400,
      requestId,
    );
  const idempotencyKey = request.headers.get('idempotency-key')?.trim();
  if (!idempotencyKey)
    return responseWithRequestId(
      { error: { type: 'invalid_request', message: 'Idempotency-Key is required.' } },
      400,
      requestId,
    );
  const requestHash = createHash('sha256')
    .update('/api/v1/audio/transcriptions\0')
    .update(rawBody)
    .digest('hex');
  const duplicate = await handleDedup(
    idempotencyKey,
    user.id,
    requestHash,
    requestId,
  );
  if (duplicate) return duplicate;
  await ensureMonthlyFreeCredits(user.id);
  const balances = await getBalances(user.id);
  const kind = balances.paid > 0 ? 'paid' : 'free';
  const rateLimit = await rateLimitTranscription(user.id);
  if (!rateLimit.ok) {
    await markDedupFailed({
      userId: user.id,
      idempotencyKey,
      requestId,
      requestHash,
    });
    return tooManyRequests(rateLimit);
  }
  let route: Awaited<ReturnType<typeof getAudioModelRoute>>;
  try {
    route = await getAudioModelRoute();
    if (route.provider !== 'openrouter')
      throw new Error('Audio model provider must be OpenRouter');
    if (!(await isProviderEnabled(route.provider)))
      throw new Error('Audio model provider is disabled');
    if (!(await isCompatibleTranscriptionModel(route.provider, route.modelId)))
      throw new Error('Audio model is no longer compatible');
  } catch (error) {
    await markDedupFailed({
      userId: user.id,
      idempotencyKey,
      requestId,
      requestHash,
    });
    const message =
      error instanceof ModelNotConfiguredError
        ? 'No audio transcription model is configured.'
        : 'Audio transcription is unavailable.';
    return responseWithRequestId(
      { error: { type: 'model_not_configured', message } },
      503,
      requestId,
    );
  }
  const reservation = await reserveCredits({
    requestId,
    userId: user.id,
    kind,
    credits: RESERVATION_CREDITS,
  });
  if (!reservation.ok) {
    await markDedupFailed({
      userId: user.id,
      idempotencyKey,
      requestId,
      requestHash,
    });
    return responseWithRequestId(
      {
        error: {
          type: 'insufficient_credits',
          message: 'Not enough credits for API dictation.',
          dashboard_url: DASHBOARD_URL,
        },
      },
      402,
      requestId,
    );
  }
  try {
    const upstream = getUpstreamConfig('openrouter');
    const upstreamResponse = await fetch(`${upstream.baseUrl}/audio/transcriptions`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${upstream.apiKey}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        model: route.modelId,
        input_audio: parsed.data.input_audio,
        response_format: 'json',
        provider: { zdr: true, data_collection: 'deny' },
      }),
      signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS),
    });
    const body = (await upstreamResponse.json().catch(() => null)) as
      | OpenRouterTranscription
      | null;
    if (!upstreamResponse.ok || typeof body?.text !== 'string') {
      await releaseAfterFailure({
        requestId,
        userId: user.id,
        idempotencyKey,
        requestHash,
        model: route.modelId,
        status: upstreamResponse.status,
        startedAt,
      });
      return responseWithRequestId(
        {
          error: {
            type: 'upstream_error',
            message: 'The transcription provider could not process the audio.',
          },
        },
        502,
        requestId,
      );
    }
    const providerCostUsd = finiteNonnegative(body.usage?.cost);
    if (providerCostUsd == null || providerCostUsd > MAX_PROVIDER_COST_USD) {
      log.error('audio_transcription_invalid_provider_cost', {
        request_id: requestId,
        provider: route.provider,
        model: route.modelId,
        provider_cost_usd: providerCostUsd,
      });
      await releaseAfterFailure({
        requestId,
        userId: user.id,
        idempotencyKey,
        requestHash,
        model: route.modelId,
        status: 502,
        startedAt,
      });
      return responseWithRequestId(
        {
          error: {
            type: 'upstream_contract_error',
            message: 'The transcription provider returned invalid billing data.',
          },
        },
        502,
        requestId,
      );
    }
    const seconds = finiteNonnegative(body.usage?.seconds);
    await finalizeReservation({
      requestId,
      outcome: 'settle',
      model: route.modelId,
      provider: route.provider,
      modality: 'audio',
      audioDurationMs: seconds == null ? 0 : Math.round(seconds * 1_000),
      tokens: {
        inputTokens: finiteNonnegative(body.usage?.input_tokens) ?? 0,
        outputTokens: finiteNonnegative(body.usage?.output_tokens) ?? 0,
      },
      providerCostUsd,
      status: upstreamResponse.status,
      latencyMs: Math.round(performance.now() - startedAt),
      dedup: {
        userId: user.id,
        idempotencyKey,
        requestHash,
        status: 'completed',
        upstreamStatus: upstreamResponse.status,
      },
    });
    log.info('audio_transcription', {
      request_id: requestId,
      user_id: user.id,
      provider: route.provider,
      model: route.modelId,
      status: upstreamResponse.status,
      latency_ms: Math.round(performance.now() - startedAt),
      audio_bytes: audio.length,
      audio_duration_ms: seconds == null ? undefined : Math.round(seconds * 1_000),
    });
    return responseWithRequestId(
      {
        text: body.text,
        requestId,
        ...(seconds == null ? {} : { usage: { seconds } }),
      },
      200,
      requestId,
    );
  } catch (error) {
    log.warn('audio_transcription_failed', {
      request_id: requestId,
      provider: route.provider,
      model: route.modelId,
      error:
        error instanceof UpstreamConfigError
          ? error.message
          : error instanceof Error
            ? error.name
            : 'unknown',
    });
    await releaseAfterFailure({
      requestId,
      userId: user.id,
      idempotencyKey,
      requestHash,
      model: route.modelId,
      status: 503,
      startedAt,
    });
    return responseWithRequestId(
      {
        error: {
          type: 'service_unavailable',
          message: 'API dictation is unavailable.',
        },
      },
      503,
      requestId,
    );
  }
};

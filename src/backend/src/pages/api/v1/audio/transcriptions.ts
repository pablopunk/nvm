import { createHash, randomUUID } from 'node:crypto';
import { waitUntil } from '@vercel/functions';
import type { APIRoute } from 'astro';
import { z } from 'zod';
import { PRODUCTION_WEB_ORIGIN } from '../../../../../../app/shared/public-origin';
import {
  backendKillSwitchEnabled,
  compatibilityError,
  compatibilityHeaders,
  desktopClientFromRequest,
  killSwitchResponse,
  requestIdFromHeaders,
  unsupportedClientReason,
} from '../../../../lib/compatibility';
import {
  normalizeTranscriptionAudio,
  TRANSCRIPTION_AUDIO_LIMITS,
} from '../../../../lib/audio-workload';
import {
  admitAudioReservation,
  cancelAudioPreparation,
  finalizeReservation,
  getAudioPreparationStatus,
  prepareAudioReservation,
} from '../../../../lib/credit-reservations';
import { usdToCredits } from '../../../../lib/cost';
import { freeSpendCapErrorBody } from '../../../../lib/free-spend-cap';
import { log } from '../../../../lib/log';
import { handleDedup, markDedupFailed } from '../../../../lib/proxy';
import {
  rateLimitTranscription,
  tooManyRequests,
} from '../../../../lib/ratelimit';
import { isRouteTripped } from '../../../../lib/route-breakers';
import {
  getAudioModelRoute,
  isProviderEnabled,
  ModelNotConfiguredError,
} from '../../../../lib/settings';
import { getUserFromBearer } from '../../../../lib/tokens';
import { lookupTranscriptionDurationPricing } from '../../../../lib/transcription-models';
import {
  getUpstreamConfig,
  UpstreamConfigError,
} from '../../../../lib/upstream';
import { ensureMonthlyFreeCredits } from '../../../../lib/users';

export const config = { maxDuration: 60 };

const MAX_AUDIO_BYTES = TRANSCRIPTION_AUDIO_LIMITS.inputBytes;
const MAX_REQUEST_BYTES = Math.ceil((MAX_AUDIO_BYTES * 4) / 3) + 8 * 1024;
const RESERVATION_CREDITS = Math.max(
  1,
  Number(process.env.AUDIO_TRANSCRIPTION_RESERVATION_CREDITS ?? 5),
);
const UPSTREAM_TIMEOUT_MS = Math.max(
  1000,
  Number(process.env.AUDIO_TRANSCRIPTION_TIMEOUT_MS ?? 15_000),
);
const RELEASED_CLIENT_TIMEOUT_MS = 16_000;
const CLIENT_RESPONSE_MARGIN_MS = 1_000;
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

type TranscriptionTimings = Record<string, number>;

function serverTimingHeader(timings: TranscriptionTimings) {
  return Object.entries(timings)
    .map(([name, duration]) => `${name};dur=${duration}`)
    .join(', ');
}

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

async function readTranscriptionAudio(request: Request) {
  const contentLength = Number(request.headers.get('content-length') ?? 0);
  if (contentLength > MAX_REQUEST_BYTES) return { error: 'too_large' as const };
  if (
    request.headers.get('content-type')?.split(';')[0].trim() === 'audio/webm'
  ) {
    const bytes = Buffer.from(await request.arrayBuffer());
    if (bytes.length > MAX_AUDIO_BYTES) return { error: 'too_large' as const };
    if (bytes.subarray(0, 4).toString('hex') !== '1a45dfa3')
      return { error: 'invalid_audio' as const };
    return {
      audio: bytes,
      inputAudio: { data: bytes.toString('base64'), format: 'webm' } as const,
      requestHash: bytes,
    };
  }
  const rawBody = await request.text();
  if (Buffer.byteLength(rawBody) > MAX_REQUEST_BYTES)
    return { error: 'too_large' as const };
  let json: unknown;
  try {
    json = JSON.parse(rawBody);
  } catch {
    return { error: 'invalid_json' as const };
  }
  const parsed = requestSchema.safeParse(json);
  if (!parsed.success) return { error: 'invalid_request' as const };
  const audio = decodeBase64Audio(parsed.data.input_audio.data);
  if (!audio) return { error: 'invalid_audio' as const };
  return { audio, inputAudio: parsed.data.input_audio, requestHash: rawBody };
}

function responseWithRequestId(
  body: unknown,
  status: number,
  requestId: string,
  timings?: TranscriptionTimings,
) {
  const headers = compatibilityHeaders(requestId);
  if (timings) headers.set('server-timing', serverTimingHeader(timings));
  return Response.json(body, {
    status,
    headers,
  });
}

function requestCancelledResponse(requestId: string) {
  return responseWithRequestId(
    {
      error: {
        type: 'request_cancelled',
        message: 'The transcription request was cancelled.',
      },
    },
    499,
    requestId,
  );
}

async function releaseAfterFailure(input: {
  reservationId: string;
  userId: string;
  idempotencyKey: string;
  requestHash: string;
  model: string;
  status: number;
  startedAt: number;
}) {
  await finalizeReservation({
    requestId: input.reservationId,
    expectedUserId: input.userId,
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
  const timings: TranscriptionTimings = {};
  async function timePhase<T>(name: string, task: () => Promise<T>) {
    const phaseStartedAt = performance.now();
    try {
      return await task();
    } finally {
      timings[name] =
        Math.round((performance.now() - phaseStartedAt) * 100) / 100;
    }
  }
  const requestId = requestIdFromHeaders(request.headers);
  const client = desktopClientFromRequest(request);
  if (unsupportedClientReason(client)) return compatibilityError(request);
  const user = await timePhase('auth', () =>
    getUserFromBearer(request.headers.get('authorization')),
  );
  if (!user)
    return responseWithRequestId(
      {
        error: {
          type: 'unauthorized',
          message: 'Sign in to use API dictation.',
        },
      },
      401,
      requestId,
    );
  if (backendKillSwitchEnabled('audio_transcription'))
    return killSwitchResponse(
      'audio_transcription',
      'API dictation is temporarily unavailable.',
      requestId,
    );
  const requestStartedAt = performance.now();
  const parsedAudio = await readTranscriptionAudio(request);
  if (parsedAudio.error === 'too_large')
    return responseWithRequestId(
      {
        error: {
          type: 'payload_too_large',
          message: 'Dictation audio is too large.',
        },
      },
      413,
      requestId,
    );
  if (parsedAudio.error === 'invalid_json')
    return responseWithRequestId(
      { error: { type: 'invalid_request', message: 'Invalid JSON body.' } },
      400,
      requestId,
    );
  if (parsedAudio.error === 'invalid_request')
    return responseWithRequestId(
      {
        error: {
          type: 'invalid_request',
          message: 'Invalid transcription request.',
        },
      },
      400,
      requestId,
    );
  if (parsedAudio.error === 'invalid_audio')
    return responseWithRequestId(
      {
        error: {
          type: 'invalid_audio',
          message: 'Invalid or oversized WebM audio.',
        },
      },
      400,
      requestId,
    );
  timings.request =
    Math.round((performance.now() - requestStartedAt) * 100) / 100;
  const idempotencyKey = request.headers.get('idempotency-key')?.trim();
  if (!idempotencyKey)
    return responseWithRequestId(
      {
        error: {
          type: 'invalid_request',
          message: 'Idempotency-Key is required.',
        },
      },
      400,
      requestId,
    );
  const requestedOperationId = request.headers
    .get('x-nevermind-dictation-operation-id')
    ?.trim();
  const operationId =
    requestedOperationId && requestedOperationId.length <= 200
      ? requestedOperationId
      : idempotencyKey;
  const requestHash = createHash('sha256')
    .update('/api/v1/audio/transcriptions\0')
    .update(parsedAudio.requestHash)
    .digest('hex');
  const preparationStatus = await timePhase('prepared', () =>
    getAudioPreparationStatus({
      requestId: idempotencyKey,
      userId: user.id,
      credits: RESERVATION_CREDITS,
      operationId,
    }),
  );
  if (preparationStatus === 'cancelled' || preparationStatus === 'unavailable')
    return responseWithRequestId(
      {
        error: {
          type: 'idempotency_conflict',
          message:
            'The recording preparation is no longer available. Retry with a new recording operation.',
        },
      },
      409,
      requestId,
    );
  if (request.signal.aborted) {
    if (preparationStatus === 'prepared') {
      await cancelAudioPreparation({
        reservationId: idempotencyKey,
        userId: user.id,
        reason: 'failed',
      }).catch(function ignoreAbortedPreparationCleanup() {});
    }
    return requestCancelledResponse(requestId);
  }
  const hasPreparation = preparationStatus === 'prepared';
  const reservationId = hasPreparation ? idempotencyKey : randomUUID();
  const duplicate = await timePhase('dedup', () =>
    handleDedup(idempotencyKey, user.id, requestHash, reservationId),
  );
  if (duplicate) {
    duplicate.headers.set('x-request-id', requestId);
    return duplicate;
  }
  let hasProvisionalHold = hasPreparation;
  let admitted = false;
  const authenticatedUser = user;
  const claimedIdempotencyKey: string = idempotencyKey;
  async function failClaim() {
    if (hasProvisionalHold)
      await cancelAudioPreparation({
        reservationId,
        userId: authenticatedUser.id,
        reason: 'failed',
      });
    await markDedupFailed({
      userId: authenticatedUser.id,
      idempotencyKey: claimedIdempotencyKey,
      requestId: reservationId,
      requestHash,
    });
  }
  async function abandonCancelledRequest() {
    await failClaim();
    return requestCancelledResponse(requestId);
  }
  try {
    const creditsPromise = hasPreparation
      ? Promise.resolve()
      : timePhase('credits', () => ensureMonthlyFreeCredits(user.id));
    const rateLimitPromise = timePhase('rate_limit', () =>
      rateLimitTranscription(user.id),
    );
    const routePromise = timePhase('model', async () => {
      const [route, providerEnabled] = await Promise.all([
        getAudioModelRoute(),
        isProviderEnabled('openrouter'),
      ]);
      if (route.provider !== 'openrouter')
        throw new Error('Audio model provider must be OpenRouter');
      if (!providerEnabled) throw new Error('Audio model provider is disabled');
      return route;
    }).then(
      (route) => ({ route }),
      (error: unknown) => ({ error }),
    );
    const [, rateLimit, routeResult] = await Promise.all([
      creditsPromise,
      rateLimitPromise,
      routePromise,
    ]);
    if (request.signal.aborted) return await abandonCancelledRequest();
    if (!rateLimit.ok) {
      await failClaim();
      return tooManyRequests(rateLimit);
    }
    if ('error' in routeResult) {
      await failClaim();
      const message =
        routeResult.error instanceof ModelNotConfiguredError
          ? 'No audio transcription model is configured.'
          : 'Audio transcription is unavailable.';
      return responseWithRequestId(
        { error: { type: 'model_not_configured', message } },
        503,
        requestId,
      );
    }
    const route = routeResult.route;
    // Audio routing intentionally has one configured OpenRouter model today;
    // unlike text proxy routes there is no provider-chain setting to fail over to.
    if (await isRouteTripped('audio', route.provider, route.modelId)) {
      await failClaim();
      return responseWithRequestId(
        {
          error: {
            type: 'model_not_configured',
            message: 'Audio transcription is temporarily unavailable.',
          },
        },
        503,
        requestId,
      );
    }
    const [pricingResult, audioResult] = await Promise.all([
      lookupTranscriptionDurationPricing(route.provider, route.modelId)
        .then((pricing) => ({ pricing }))
        .catch(() => ({ pricing: null })),
      timePhase('audio', () => normalizeTranscriptionAudio(parsedAudio.audio))
        .then((audio) => ({ audio }))
        .catch((error: unknown) => ({ error })),
    ]);
    const pricing = pricingResult.pricing;
    if ('error' in audioResult) {
      await failClaim();
      return responseWithRequestId(
        {
          error: {
            type: 'invalid_audio',
            message: 'The WebM recording could not be validated safely.',
          },
        },
        400,
        requestId,
      );
    }
    const normalizedAudio = audioResult.audio;
    if (request.signal.aborted) return await abandonCancelledRequest();
    if (!pricing) {
      log.warn('audio_transcription_pricing_unavailable', {
        request_id: requestId,
        reservation_id: reservationId,
        client_version: client.version,
        provider: route.provider,
        model: route.modelId,
      });
      await failClaim();
      return responseWithRequestId(
        {
          error: {
            type: 'pricing_unavailable',
            message: 'Audio transcription pricing is unavailable.',
          },
        },
        503,
        requestId,
      );
    }
    const estimatedProviderCostUsd =
      normalizedAudio.estimatedBillableSeconds * pricing.usdPerSecond;
    if (usdToCredits(estimatedProviderCostUsd) > RESERVATION_CREDITS) {
      await failClaim();
      return responseWithRequestId(
        {
          error: {
            type: 'audio_cost_limit',
            message: `This recording exceeds the ${RESERVATION_CREDITS}-credit per-request limit. Split it into shorter clips and retry.`,
          },
        },
        413,
        requestId,
      );
    }
    if (request.signal.aborted) return await abandonCancelledRequest();
    const preparation = hasPreparation
      ? null
      : await timePhase('prepare', () =>
          prepareAudioReservation({
            reservationId,
            operationId,
            userId: user.id,
            credits: RESERVATION_CREDITS,
          }),
        );
    if (preparation?.ok) hasProvisionalHold = true;
    if (request.signal.aborted) return await abandonCancelledRequest();
    const reservation =
      preparation && !preparation.ok
        ? preparation
        : await timePhase('reserve', () =>
            admitAudioReservation({
              reservationId,
              operationId,
              userId: user.id,
              credits: RESERVATION_CREDITS,
              estimatedProviderCostUsd,
              estimateCostSource: 'catalog_estimate',
            }),
          );
    if (reservation && !reservation.ok) {
      await failClaim();
      if (reservation.reason === 'free_spend_cap_reached') {
        return responseWithRequestId(freeSpendCapErrorBody(), 503, requestId);
      }
      if (reservation.reason !== 'insufficient_credits')
        return responseWithRequestId(
          {
            error: {
              type: 'idempotency_conflict',
              message:
                'The recording preparation is no longer available. Retry the recording.',
            },
          },
          409,
          requestId,
        );
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
    admitted = true;
    const providerTimeoutMs = Math.floor(
      Math.min(
        UPSTREAM_TIMEOUT_MS,
        RELEASED_CLIENT_TIMEOUT_MS -
          CLIENT_RESPONSE_MARGIN_MS -
          (performance.now() - startedAt),
      ),
    );
    if (providerTimeoutMs < 1_000)
      throw new Error(
        'Insufficient time remains for the transcription request',
      );
    try {
      const upstream = getUpstreamConfig('openrouter');
      const { upstreamResponse, body } = await timePhase(
        'provider',
        async () => {
          const upstreamResponse = await fetch(
            `${upstream.baseUrl}/audio/transcriptions`,
            {
              method: 'POST',
              headers: {
                Authorization: `Bearer ${upstream.apiKey}`,
                'Content-Type': 'application/json',
              },
              body: JSON.stringify({
                model: route.modelId,
                input_audio: normalizedAudio.inputAudio,
                response_format: 'json',
              }),
              signal: AbortSignal.any([
                request.signal,
                AbortSignal.timeout(providerTimeoutMs),
              ]),
            },
          );
          const body = (await upstreamResponse
            .json()
            .catch(() => null)) as OpenRouterTranscription | null;
          return { upstreamResponse, body };
        },
      );
      if (!upstreamResponse.ok || typeof body?.text !== 'string') {
        await releaseAfterFailure({
          reservationId,
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
              message:
                'The transcription provider could not process the audio.',
            },
          },
          502,
          requestId,
        );
      }
      const providerCostUsd = finiteNonnegative(body.usage?.cost);
      if (
        providerCostUsd == null ||
        providerCostUsd > Number.MAX_SAFE_INTEGER / 1e8
      ) {
        log.error('audio_transcription_invalid_provider_cost', {
          request_id: requestId,
          reservation_id: reservationId,
          provider: route.provider,
          model: route.modelId,
          provider_cost_usd: providerCostUsd,
          provider_cost_known: false,
        });
        await releaseAfterFailure({
          reservationId,
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
              message:
                'The transcription provider returned billing data that cannot be stored safely.',
            },
          },
          502,
          requestId,
        );
      }
      const seconds = finiteNonnegative(body.usage?.seconds);
      const settlementStartedAt = performance.now();
      const settlement = finalizeReservation({
        requestId: reservationId,
        expectedUserId: user.id,
        outcome: 'settle',
        model: route.modelId,
        provider: route.provider,
        routeSlot: 'audio',
        modality: 'audio',
        audioDurationMs: seconds == null ? 0 : Math.round(seconds * 1000),
        tokens: {
          inputTokens: finiteNonnegative(body.usage?.input_tokens) ?? 0,
          outputTokens: finiteNonnegative(body.usage?.output_tokens) ?? 0,
        },
        audioTokenCountsAvailable:
          finiteNonnegative(body.usage?.input_tokens) != null &&
          finiteNonnegative(body.usage?.output_tokens) != null,
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
      })
        .then(() => {
          log.info('audio_transcription_settled', {
            request_id: requestId,
            reservation_id: reservationId,
            settlement_ms:
              Math.round((performance.now() - settlementStartedAt) * 100) / 100,
          });
        })
        .catch((error) => {
          log.error('audio_transcription_settlement_failed', {
            request_id: requestId,
            reservation_id: reservationId,
            error,
          });
        });
      waitUntil(settlement);
      timings.total = Math.round((performance.now() - startedAt) * 100) / 100;
      log.info('audio_transcription', {
        request_id: requestId,
        reservation_id: reservationId,
        client_version: client.version,
        user_id: user.id,
        provider: route.provider,
        model: route.modelId,
        status: upstreamResponse.status,
        latency_ms: Math.round(performance.now() - startedAt),
        audio_bytes: parsedAudio.audio.length,
        audio_duration_ms:
          seconds == null ? undefined : Math.round(seconds * 1000),
        timings_ms: timings,
        settlement_deferred: true,
      });
      return responseWithRequestId(
        {
          text: body.text,
          requestId,
          ...(seconds == null ? {} : { usage: { seconds } }),
        },
        200,
        requestId,
        timings,
      );
    } catch (error) {
      log.warn('audio_transcription_failed', {
        request_id: requestId,
        reservation_id: reservationId,
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
        reservationId,
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
  } catch (error) {
    log.warn('audio_transcription_admission_failed', {
      request_id: requestId,
      reservation_id: reservationId,
      client_version: client.version,
      dispatched: admitted,
      error: error instanceof Error ? error.name : 'unknown',
    });
    try {
      if (admitted)
        await finalizeReservation({
          requestId: reservationId,
          expectedUserId: user.id,
          outcome: 'release',
          dedup: {
            userId: user.id,
            idempotencyKey,
            requestHash,
            status: 'failed',
          },
        });
      else await failClaim();
    } catch (cleanupError) {
      log.error('audio_transcription_cleanup_failed', {
        request_id: requestId,
        reservation_id: reservationId,
        error: cleanupError instanceof Error ? cleanupError.name : 'unknown',
      });
    }
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

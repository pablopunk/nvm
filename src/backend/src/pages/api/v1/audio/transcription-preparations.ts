import type { APIRoute } from 'astro';
import { z } from 'zod';
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
  cancelAudioPreparation,
  prepareAudioReservation,
} from '../../../../lib/credit-reservations';
import { freeSpendCapErrorBody } from '../../../../lib/free-spend-cap';
import { log } from '../../../../lib/log';
import {
  rateLimitTranscriptionCancellationTombstone,
  rateLimitTranscriptionPreparation,
  tooManyRequests,
} from '../../../../lib/ratelimit';
import { isRouteTripped } from '../../../../lib/route-breakers';
import {
  getAudioModelRoute,
  isProviderEnabled,
  ModelNotConfiguredError,
} from '../../../../lib/settings';
import { getUserFromBearer } from '../../../../lib/tokens';
import { ensureMonthlyFreeCredits } from '../../../../lib/users';

const RESERVATION_CREDITS = Math.max(
  1,
  Number(process.env.AUDIO_TRANSCRIPTION_RESERVATION_CREDITS ?? 5),
);
const preparationRequestSchema = z
  .object({ operationId: z.string().min(1).max(200) })
  .strict();

function response(body: unknown, status: number, requestId: string) {
  return Response.json(body, {
    status,
    headers: compatibilityHeaders(requestId),
  });
}

function rateLimitedResponse(
  decision: Parameters<typeof tooManyRequests>[0],
  requestId: string,
) {
  const limited = tooManyRequests(decision);
  compatibilityHeaders(requestId).forEach(
    function setCompatibilityHeader(value, key) {
      limited.headers.set(key, value);
    },
  );
  return limited;
}

async function authenticatedRequest(request: Request, checkKillSwitch = true) {
  const requestId = requestIdFromHeaders(request.headers);
  const client = desktopClientFromRequest(request);
  if (unsupportedClientReason(client))
    return { error: compatibilityError(request) };
  const user = await getUserFromBearer(request.headers.get('authorization'));
  if (!user)
    return {
      error: response(
        {
          error: {
            type: 'unauthorized',
            message: 'Sign in to use API dictation.',
          },
        },
        401,
        requestId,
      ),
    };
  if (checkKillSwitch && backendKillSwitchEnabled('audio_transcription'))
    return {
      error: killSwitchResponse(
        'audio_transcription',
        'API dictation is temporarily unavailable.',
        requestId,
      ),
    };
  return { requestId, user };
}

async function preparationRequestFromRequest(request: Request) {
  const parsed = preparationRequestSchema.safeParse(
    await request.json().catch(() => null),
  );
  return parsed.success ? parsed.data : null;
}

export const POST: APIRoute = async ({ request }) => {
  const auth = await authenticatedRequest(request);
  if (auth.error) return auth.error;
  const preparationRequest = await preparationRequestFromRequest(request);
  if (!preparationRequest)
    return response(
      {
        error: {
          type: 'invalid_request',
          message: 'A valid operationId is required.',
        },
      },
      400,
      auth.requestId,
    );
  const operationId = preparationRequest.operationId;
  const rateLimit = await rateLimitTranscriptionPreparation(auth.user.id);
  if (!rateLimit.ok) return rateLimitedResponse(rateLimit, auth.requestId);
  try {
    const [, route, providerEnabled] = await Promise.all([
      ensureMonthlyFreeCredits(auth.user.id),
      getAudioModelRoute(),
      isProviderEnabled('openrouter'),
    ]);
    if (route.provider !== 'openrouter' || !providerEnabled)
      throw new Error('Audio transcription model is unavailable');
    if (await isRouteTripped('audio', route.provider, route.modelId))
      return response(
        {
          error: {
            type: 'model_not_configured',
            message: 'Audio transcription is temporarily unavailable.',
          },
        },
        503,
        auth.requestId,
      );
    const reservation = await prepareAudioReservation({
      reservationId: operationId,
      userId: auth.user.id,
      credits: RESERVATION_CREDITS,
    });
    if (!reservation.ok && reservation.reason === 'free_spend_cap_reached')
      return response(freeSpendCapErrorBody(), 503, auth.requestId);
    if (
      !reservation.ok &&
      (reservation.reason === 'request_already_reserved' ||
        reservation.reason === 'invalid_audio_preparation')
    )
      return response(
        {
          error: {
            type: 'idempotency_conflict',
            message:
              'This recording preparation is no longer available. Start a new recording.',
          },
        },
        409,
        auth.requestId,
      );
    if (!reservation.ok)
      return response(
        {
          error: {
            type: 'insufficient_credits',
            message: 'Not enough credits for API dictation.',
          },
        },
        402,
        auth.requestId,
      );
    return response({ prepared: true, operationId }, 200, auth.requestId);
  } catch (error) {
    return response(
      {
        error: {
          type: 'preparation_failed',
          message:
            error instanceof ModelNotConfiguredError
              ? 'No audio transcription model is configured.'
              : 'API dictation preparation failed.',
        },
      },
      503,
      auth.requestId,
    );
  }
};

export const DELETE: APIRoute = async ({ request }) => {
  const auth = await authenticatedRequest(request, false);
  if (auth.error) return auth.error;
  const cancellationRequest = await preparationRequestFromRequest(request);
  if (!cancellationRequest)
    return response(
      {
        error: {
          type: 'invalid_request',
          message: 'A valid operationId is required.',
        },
      },
      400,
      auth.requestId,
    );
  const operationId = cancellationRequest.operationId;
  const isUserCancellation =
    request.headers.get('x-nevermind-dictation-cancellation-reason')?.trim() ===
    'user_cancel';
  let result: Awaited<ReturnType<typeof cancelAudioPreparation>>;
  try {
    result = await cancelAudioPreparation({
      reservationId: operationId,
      userId: auth.user.id,
      reason: isUserCancellation ? 'cancelled' : 'failed',
    });
    if (isUserCancellation && result === 'not_prepared') {
      let rateLimit: Awaited<
        ReturnType<typeof rateLimitTranscriptionCancellationTombstone>
      >;
      try {
        rateLimit = await rateLimitTranscriptionCancellationTombstone(
          auth.user.id,
        );
      } catch (error) {
        log.warn('audio_cancellation_rate_limit_failed_open', {
          request_id: auth.requestId,
          error: error instanceof Error ? error.name : 'unknown',
        });
        rateLimit = { ok: true } as const;
      }
      if (!rateLimit.ok) {
        // A preparation may have appeared while the tombstone limiter ran.
        // Always release a hold that now exists, even when the tombstone write
        // itself is over quota.
        result = await cancelAudioPreparation({
          reservationId: operationId,
          userId: auth.user.id,
          reason: 'cancelled',
        });
        if (result === 'not_prepared')
          return rateLimitedResponse(rateLimit, auth.requestId);
      } else {
        result = await cancelAudioPreparation({
          reservationId: operationId,
          userId: auth.user.id,
          reason: 'cancelled',
          tombstoneMissing: true,
        });
      }
    }
  } catch (error) {
    log.warn('audio_preparation_cancellation_failed', {
      request_id: auth.requestId,
      error: error instanceof Error ? error.name : 'unknown',
    });
    return response(
      {
        error: {
          type: 'cancellation_failed',
          message: 'Audio cancellation could not be recorded.',
        },
      },
      503,
      auth.requestId,
    );
  }
  return response(
    { released: result !== 'not_prepared', operationId },
    200,
    auth.requestId,
  );
};

import type { APIRoute } from 'astro';
import { z } from 'zod';
import {
  finalizeReservation,
  reserveCredits,
} from '../../../../lib/credit-reservations';
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
const requestSchema = z
  .object({ operationId: z.string().min(1).max(200) })
  .strict();

function response(body: unknown, status: number, requestId: string) {
  return Response.json(body, {
    status,
    headers: compatibilityHeaders(requestId),
  });
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
        { error: { type: 'unauthorized', message: 'Sign in to use API dictation.' } },
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

async function operationIdFromRequest(request: Request) {
  const parsed = requestSchema.safeParse(await request.json().catch(() => null));
  return parsed.success ? parsed.data.operationId : null;
}

export const POST: APIRoute = async ({ request }) => {
  const auth = await authenticatedRequest(request);
  if ('error' in auth) return auth.error;
  const operationId = await operationIdFromRequest(request);
  if (!operationId)
    return response(
      { error: { type: 'invalid_request', message: 'A valid operationId is required.' } },
      400,
      auth.requestId,
    );
  try {
    const [, route, providerEnabled] = await Promise.all([
      ensureMonthlyFreeCredits(auth.user.id),
      getAudioModelRoute(),
      isProviderEnabled('openrouter'),
    ]);
    if (route.provider !== 'openrouter' || !providerEnabled)
      throw new Error('Audio transcription model is unavailable');
    const reservation = await reserveCredits({
      requestId: operationId,
      userId: auth.user.id,
      credits: RESERVATION_CREDITS,
      allowExistingPending: true,
    });
    if (!reservation.ok)
      return response(
        { error: { type: 'insufficient_credits', message: 'Not enough credits for API dictation.' } },
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
  if ('error' in auth) return auth.error;
  const operationId = await operationIdFromRequest(request);
  if (!operationId)
    return response(
      { error: { type: 'invalid_request', message: 'A valid operationId is required.' } },
      400,
      auth.requestId,
    );
  try {
    await finalizeReservation({
      requestId: operationId,
      expectedUserId: auth.user.id,
      outcome: 'release',
    });
  } catch {}
  return response({ released: true, operationId }, 200, auth.requestId);
};

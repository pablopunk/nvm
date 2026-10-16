import { randomUUID } from 'node:crypto';
import {
  measureDebugPerformance,
  recordDebugPerformance,
} from '../diagnostics/debug-performance';
import { nevermindDesktopHeaders } from '../backend/nevermind-api';
import { getNevermindAuth } from '../backend/nevermind-auth';

const MAX_AUDIO_BYTES = 4_194_304;
const TRANSCRIPTION_TIMEOUT_MS = 16_000;
const TRAILING_SLASH_PATTERN = /\/$/;
const SERVER_TIMING_ENTRY_PATTERN = /^([a-z_]+);dur=([0-9.]+)$/;
const preparationRequests = new Map<string, Promise<void>>();
type DictationCancellationReason = 'user_cancel' | 'cleanup';

class ApiDictationUnavailableError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ApiDictationUnavailableError';
  }
}

function recordBackendTimings(
  header: string | null,
  detail: Record<string, unknown>,
) {
  if (!header) {
    return;
  }
  for (const rawEntry of header.split(',')) {
    const match = rawEntry.trim().match(SERVER_TIMING_ENTRY_PATTERN);
    if (!match) {
      continue;
    }
    const durationMs = Number(match[2]);
    if (!Number.isFinite(durationMs)) {
      continue;
    }
    recordDebugPerformance(`dictation.backend.${match[1]}`, durationMs, {
      ...detail,
      alwaysLog: true,
    });
  }
}

async function apiDictationIsAvailable() {
  const auth = await getNevermindAuth();
  return auth !== null;
}

async function requestDictationPreparation(
  operationId: string,
  method: 'POST' | 'DELETE',
  cancellationReason?: DictationCancellationReason,
) {
  const auth = await getNevermindAuth();
  if (!auth) {
    throw new ApiDictationUnavailableError('Sign in required');
  }
  const response = await fetch(
    `${auth.baseUrl.replace(TRAILING_SLASH_PATTERN, '')}/api/v1/audio/transcription-preparations`,
    {
      method,
      headers: nevermindDesktopHeaders(
        Object.fromEntries([
          ['Authorization', `Bearer ${auth.token}`],
          ['Content-Type', 'application/json'],
          ['X-Request-ID', randomUUID()],
          ...(method === 'DELETE' && cancellationReason === 'user_cancel'
            ? [
                [
                  'X-Nevermind-Dictation-Cancellation-Reason',
                  'user_cancel',
                ] as const,
              ]
            : []),
        ]),
      ),
      body: JSON.stringify({ operationId }),
      signal: AbortSignal.timeout(TRANSCRIPTION_TIMEOUT_MS),
    },
  );
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as {
      error?: { message?: unknown };
    } | null;
    throw new ApiDictationUnavailableError(
      typeof body?.error?.message === 'string'
        ? body.error.message
        : `API dictation preparation returned ${response.status}`,
    );
  }
}

function prepareDictationTranscription(operationId: string) {
  const existing = preparationRequests.get(operationId);
  if (existing) {
    return existing;
  }
  const preparation = measureDebugPerformance(
    'dictation.prepare',
    { operationId, alwaysLog: true },
    () => requestDictationPreparation(operationId, 'POST'),
  );
  preparationRequests.set(operationId, preparation);
  return preparation;
}

async function cancelDictationPreparation(
  operationId: string,
  cancellationReason: DictationCancellationReason = 'cleanup',
) {
  const preparation = preparationRequests.get(operationId);
  preparationRequests.delete(operationId);
  if (preparation) {
    await preparation.catch(ignorePreparationFailure);
  }
  await requestDictationPreparation(
    operationId,
    'DELETE',
    cancellationReason,
  ).catch(ignorePreparationFailure);
}

function ignorePreparationFailure() {
  return;
}

async function transcribeDictationAudio(input: {
  operationId: string;
  cancellationId?: string;
  audio: Uint8Array;
  mimeType: string;
  signal: AbortSignal;
}) {
  if (
    input.audio.byteLength === 0 ||
    input.audio.byteLength > MAX_AUDIO_BYTES ||
    !input.mimeType.toLowerCase().startsWith('audio/webm')
  ) {
    throw new ApiDictationUnavailableError('Unsupported dictation audio');
  }
  const preparation = preparationRequests.get(input.operationId);
  const prepared = preparation
    ? await measureDebugPerformance(
        'dictation.prepare-wait',
        { operationId: input.operationId, alwaysLog: true },
        () =>
          preparation.then(
            () => true,
            () => false,
          ),
      )
    : false;
  preparationRequests.delete(input.operationId);
  const timingDetail = { operationId: input.operationId, alwaysLog: true };
  const auth = await measureDebugPerformance(
    'dictation.auth',
    timingDetail,
    getNevermindAuth,
  );
  if (!auth) {
    throw new ApiDictationUnavailableError('Sign in required');
  }
  const timeout = AbortSignal.timeout(TRANSCRIPTION_TIMEOUT_MS);
  const signal = AbortSignal.any([input.signal, timeout]);
  const requestId = input.operationId;
  const response = await measureDebugPerformance(
    'dictation.http-request',
    { ...timingDetail, requestId, audioBytes: input.audio.byteLength },
    () => requestTranscription({ auth, input, signal, requestId, prepared }),
  );
  recordBackendTimings(response.headers.get('server-timing'), {
    operationId: input.operationId,
    requestId,
  });
  const body = await measureDebugPerformance(
    'dictation.response-parse',
    { ...timingDetail, requestId, status: response.status },
    async () =>
      (await response.json().catch(() => null)) as {
        text?: unknown;
        error?: { message?: unknown };
      } | null,
  );
  if (!response.ok || typeof body?.text !== 'string') {
    throw new ApiDictationUnavailableError(
      typeof body?.error?.message === 'string'
        ? body.error.message
        : `API dictation returned ${response.status}`,
    );
  }
  return body.text.trim();
}

async function requestTranscription(options: {
  auth: { baseUrl: string; token: string };
  input: {
    operationId: string;
    cancellationId?: string;
    audio: Uint8Array;
    signal: AbortSignal;
  };
  signal: AbortSignal;
  requestId: string;
  prepared: boolean;
}) {
  const { auth, input, signal, requestId, prepared } = options;
  const body = input.audio;
  let response: Response;
  try {
    response = await fetch(
      `${auth.baseUrl.replace(TRAILING_SLASH_PATTERN, '')}/api/v1/audio/transcriptions`,
      {
        method: 'POST',
        headers: nevermindDesktopHeaders(
          Object.fromEntries([
            ['Authorization', `Bearer ${auth.token}`],
            ['Content-Type', 'audio/webm'],
            ['Idempotency-Key', input.operationId],
            ['X-Request-ID', requestId],
            ...(input.cancellationId
              ? [
                  [
                    'X-Nevermind-Dictation-Operation-ID',
                    input.cancellationId,
                  ] as const,
                ]
              : []),
            ...(prepared
              ? [
                  [
                    'X-Nevermind-Dictation-Preparation',
                    input.operationId,
                  ] as const,
                ]
              : []),
          ]),
        ),
        body: Buffer.from(body),
        signal,
      },
    );
  } catch (error) {
    if (input.signal.aborted) {
      throw error;
    }
    throw new ApiDictationUnavailableError(
      error instanceof DOMException && error.name === 'TimeoutError'
        ? 'Cloud transcription timed out'
        : 'Cloud transcription is unavailable',
    );
  }
  return response;
}

export {
  ApiDictationUnavailableError,
  apiDictationIsAvailable,
  cancelDictationPreparation,
  prepareDictationTranscription,
  transcribeDictationAudio,
};

import { randomUUID } from 'node:crypto';
import {
  measureDebugPerformance,
  measureDebugPerformanceSync,
} from './debug-performance';
import { nevermindDesktopHeaders } from './nevermind-api';
import { getNevermindAuth } from './nevermind-auth';

const MAX_AUDIO_BYTES = 4_194_304;
const TRANSCRIPTION_TIMEOUT_MS = 16_000;
const TRAILING_SLASH_PATTERN = /\/$/;
const SERVER_TIMING_ENTRY_PATTERN = /^([a-z_]+);dur=([0-9.]+)$/;

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

async function transcribeDictationAudio(input: {
  operationId: string;
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
  const requestId = randomUUID();
  const response = await measureDebugPerformance(
    'dictation.http-request',
    { ...timingDetail, requestId, audioBytes: input.audio.byteLength },
    () => requestTranscription(auth, input, signal, requestId),
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

async function requestTranscription(
  auth: { baseUrl: string; token: string },
  input: {
    operationId: string;
    audio: Uint8Array;
    signal: AbortSignal;
  },
  signal: AbortSignal,
  requestId: string,
) {
  const timingDetail = {
    operationId: input.operationId,
    requestId,
    alwaysLog: true,
  };
  const body = measureDebugPerformanceSync(
    'dictation.request-encode',
    { ...timingDetail, audioBytes: input.audio.byteLength },
    () =>
      JSON.stringify(
        Object.fromEntries([
          [
            'input_audio',
            {
              data: Buffer.from(input.audio).toString('base64'),
              format: 'webm',
            },
          ],
        ]),
      ),
  );
  let response: Response;
  try {
    response = await fetch(
      `${auth.baseUrl.replace(TRAILING_SLASH_PATTERN, '')}/api/v1/audio/transcriptions`,
      {
        method: 'POST',
        headers: nevermindDesktopHeaders(
          Object.fromEntries([
            ['Authorization', `Bearer ${auth.token}`],
            ['Content-Type', 'application/json'],
            ['Idempotency-Key', input.operationId],
            ['X-Request-ID', requestId],
          ]),
        ),
        body,
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
  transcribeDictationAudio,
};

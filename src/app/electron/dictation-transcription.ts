import { randomUUID } from 'node:crypto';
import { nevermindDesktopHeaders } from './nevermind-api';
import { getNevermindAuth } from './nevermind-auth';
import { checkNevermindCompatibility } from './nevermind-compatibility';

const MAX_AUDIO_BYTES = 4_194_304;
const TRANSCRIPTION_TIMEOUT_MS = 16_000;
const TRAILING_SLASH_PATTERN = /\/$/;

class ApiDictationUnavailableError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ApiDictationUnavailableError';
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
  const auth = await getNevermindAuth();
  if (!auth) {
    throw new ApiDictationUnavailableError('Sign in required');
  }
  await checkNevermindCompatibility(auth.baseUrl);
  const timeout = AbortSignal.timeout(TRANSCRIPTION_TIMEOUT_MS);
  const signal = AbortSignal.any([input.signal, timeout]);
  const response = await requestTranscription(auth, input, signal);
  const body = (await response.json().catch(() => null)) as {
    text?: unknown;
    error?: { message?: unknown };
  } | null;
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
) {
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
            ['X-Request-ID', randomUUID()],
          ]),
        ),
        body: JSON.stringify(
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

import assert from 'node:assert/strict';
import { test } from 'node:test';

test('sends explicit cancellation intent and stable root operation identity', async function cancellationContract(t) {
  const requests: Array<{ url: string; init: RequestInit | undefined }> = [];
  t.mock.module('./debug-performance', {
    namedExports: {
      measureDebugPerformance<T>(
        _name: string,
        _detail: Record<string, unknown>,
        task: () => Promise<T> | T,
      ) {
        return Promise.resolve(task());
      },
      recordDebugPerformance() {},
    },
  });
  t.mock.module('./nevermind-api', {
    namedExports: {
      nevermindDesktopHeaders(headers: Record<string, string>) {
        return headers;
      },
    },
  });
  t.mock.module('./nevermind-auth', {
    namedExports: {
      async getNevermindAuth() {
        return { baseUrl: 'https://api.nvm.fyi', token: 'test-token' };
      },
    },
  });
  t.mock.method(
    globalThis,
    'fetch',
    async function captureRequest(
      input: RequestInfo | URL,
      init?: RequestInit,
    ) {
      requests.push({ url: String(input), init });
      return String(input).endsWith('/audio/transcriptions')
        ? Response.json({ text: 'transcript' })
        : new Response(null, { status: 200 });
    },
  );

  const { cancelDictationPreparation, transcribeDictationAudio } = await import(
    './dictation-transcription'
  );
  await cancelDictationPreparation('explicit-operation', 'user_cancel');
  await cancelDictationPreparation('cleanup-operation');
  await transcribeDictationAudio({
    operationId: 'segmented-operation-1',
    cancellationId: 'segmented-operation',
    audio: new Uint8Array([1]),
    mimeType: 'audio/webm',
    signal: new AbortController().signal,
  });

  const explicitCancel = requests[0];
  assert.ok(explicitCancel?.init?.body);
  assert.deepEqual(JSON.parse(String(explicitCancel.init.body)), {
    operationId: 'explicit-operation',
  });
  assert.equal(
    new Headers(explicitCancel.init.headers).get(
      'x-nevermind-dictation-cancellation-reason',
    ),
    'user_cancel',
  );

  const cleanup = requests[1];
  assert.ok(cleanup?.init?.body);
  assert.deepEqual(JSON.parse(String(cleanup.init.body)), {
    operationId: 'cleanup-operation',
  });
  assert.equal(
    new Headers(cleanup.init.headers).get(
      'x-nevermind-dictation-cancellation-reason',
    ),
    null,
  );

  const transcription = requests[2];
  assert.ok(transcription);
  const headers = new Headers(transcription.init?.headers);
  assert.equal(headers.get('idempotency-key'), 'segmented-operation-1');
  assert.equal(
    headers.get('x-nevermind-dictation-operation-id'),
    'segmented-operation',
  );
});

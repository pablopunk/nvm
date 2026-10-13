import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import type { APIContext } from 'astro';
import type { ReservationResult } from '../../../../lib/credit-reservations';

const MODEL_ID = 'nvidia/parakeet-tdt-0.6b-v3';
const USER_ID = 'audio_user';
const OPERATION_ID = 'dictation_operation';
const CORRELATION_ID = 'desktop_correlation';
const WEBM = Buffer.from('1a45dfa3', 'hex');
const WAV = Buffer.from('RIFFcanonical-wav').toString('base64');
const PRICING = {
  unit: 'second' as const,
  usdPerSecond: 0.0001,
  source: 'openrouter_transcription_catalog' as const,
};

type PreparationInput = {
  reservationId: string;
  operationId?: string;
  userId: string;
  credits: number;
};
type AdmissionInput = PreparationInput & {
  estimatedProviderCostUsd: number;
  estimateCostSource: string;
};
type PreparationStatus = 'prepared' | 'missing' | 'cancelled' | 'unavailable';
type CancellationInput = {
  reservationId: string;
  userId: string;
  reason?: 'cancelled' | 'failed';
  tombstoneMissing?: boolean;
};
type FailedDedupInput = {
  userId: string;
  idempotencyKey: string;
  requestId: string;
  requestHash: string;
};
type FinalizationInput = {
  requestId: string;
  outcome: string;
  model?: string;
  provider?: string;
  modality?: string;
  providerCostUsd?: number;
  dedup?: {
    userId: string;
    idempotencyKey: string;
    status: string;
  };
};

function createState() {
  let resolveSettlement!: () => void;
  const settlementGate = new Promise<void>(function captureResolve(resolve) {
    resolveSettlement = resolve;
  });
  return {
    preparations: new Map<
      string,
      'prepared' | 'admitted' | 'cancelled' | 'failed' | 'unavailable'
    >(),
    statusCalls: [] as {
      requestId: string;
      userId: string;
      credits: number;
      operationId?: string;
    }[],
    prepareCalls: [] as PreparationInput[],
    admissionCalls: [] as AdmissionInput[],
    cancelCalls: [] as CancellationInput[],
    failedDedupCalls: [] as FailedDedupInput[],
    finalizationCalls: [] as FinalizationInput[],
    settledRows: [] as FinalizationInput[],
    dedupCalls: [] as unknown[][],
    pricingCalls: [] as unknown[][],
    normalizationCalls: [] as Buffer[],
    upstreamBodies: [] as Record<string, unknown>[],
    providerSignals: [] as Array<AbortSignal | null>,
    backgroundTasks: [] as Promise<unknown>[],
    events: [] as string[],
    claimEvents: [] as string[],
    pricing: PRICING as typeof PRICING | null,
    normalizationError: null as Error | null,
    estimatedBillableSeconds: 13,
    admissionFailure: null as Extract<ReservationResult, { ok: false }> | null,
    preparedLookupSnapshot: null as PreparationStatus | null,
    rateLimitError: null as Error | null,
    creditsError: null as Error | null,
    breakerError: null as Error | null,
    preparationError: null as Error | null,
    admissionError: null as Error | null,
    upstreamError: null as Error | null,
    cancellationGate: null as Promise<void> | null,
    onCancellationStarted: null as (() => void) | null,
    normalizationGate: null as Promise<void> | null,
    onNormalizationStarted: null as (() => void) | null,
    admissionGate: null as Promise<void> | null,
    onAdmissionStarted: null as (() => void) | null,
    preparationRateLimitDenied: false,
    cancellationRateLimitDenied: false,
    preparationRateLimitCalls: 0,
    cancellationRateLimitCalls: 0,
    resolveSettlement,
    settlementGate,
  };
}

function audioRequest(
  path: 'transcriptions' | 'transcription-preparations',
  options: {
    method?: string;
    prepared?: boolean;
    legacy?: boolean;
    cancellationId?: string;
    cancellationReason?: 'user_cancel' | 'cleanup';
    clientVersion?: string;
    operationId?: string;
    signal?: AbortSignal;
  } = {},
) {
  const preparation = path === 'transcription-preparations';
  return new Request(`https://api.nvm.fyi/api/v1/audio/${path}`, {
    method: options.method ?? 'POST',
    headers: {
      authorization: 'Bearer nvm_pat_audio_test',
      'content-type':
        preparation || options.legacy ? 'application/json' : 'audio/webm',
      'x-nevermind-client': 'desktop',
      'x-nevermind-client-version': options.clientVersion ?? '0.22.1',
      'x-nevermind-api-version': '1',
      'x-request-id': CORRELATION_ID,
      'idempotency-key': options.operationId ?? OPERATION_ID,
      ...(options.cancellationId
        ? { 'x-nevermind-dictation-operation-id': options.cancellationId }
        : {}),
      ...(options.cancellationReason
        ? {
            'x-nevermind-dictation-cancellation-reason':
              options.cancellationReason,
          }
        : {}),
      ...(options.prepared
        ? { 'x-nevermind-dictation-preparation': OPERATION_ID }
        : {}),
    },
    body: preparation
      ? JSON.stringify({ operationId: options.operationId ?? OPERATION_ID })
      : options.legacy
        ? JSON.stringify({
            input_audio: { data: WEBM.toString('base64'), format: 'webm' },
          })
        : new Uint8Array(WEBM),
    ...(options.signal ? { signal: options.signal } : {}),
  });
}

function context(request: Request): APIContext {
  return { request, url: new URL(request.url) } as APIContext;
}

test('audio admission route regressions', async function audioAdmissionTests(t) {
  // Module mocks require a Node flag. Keep this focused suite runnable through
  // the normal backend test command without changing its shared runner.
  if (!process.execArgv.includes('--experimental-test-module-mocks')) {
    const environment = { ...process.env };
    delete environment.NODE_TEST_CONTEXT;
    const result = spawnSync(
      process.execPath,
      [
        '--experimental-test-module-mocks',
        '--import',
        'tsx',
        '--test',
        fileURLToPath(import.meta.url),
      ],
      { env: environment, encoding: 'utf8' },
    );
    assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
    return;
  }

  let state = createState();
  function mockLibrary(name: string, namedExports: Record<string, unknown>) {
    t.mock.module(new URL(`../../../../lib/${name}.ts`, import.meta.url).href, {
      namedExports,
    });
  }
  function forbiddenLegacyBilling() {
    assert.fail(
      'Audio routes must not use token pricing or generic reservation admission',
    );
  }
  function reservationSuccess(input: PreparationInput) {
    return {
      ok: true,
      reservationId: input.reservationId,
      reservation: {
        requestId: input.reservationId,
        userId: input.userId,
        purpose: 'audio',
        audioState: state.preparations.get(input.reservationId),
        status: 'pending',
        reservedCredits: 5,
        reservedFreeCredits: 5,
        reservedPaidCredits: 0,
      },
      balance: 100,
      reserved: 5,
    };
  }
  mockLibrary('credit-reservations', {
    reserveCredits: forbiddenLegacyBilling,
    async prepareAudioReservation(input: PreparationInput) {
      state.prepareCalls.push(input);
      if (state.preparationError) throw state.preparationError;
      if (
        state.preparations.get(input.operationId ?? input.reservationId) ===
        'cancelled'
      )
        return {
          ok: false,
          reason: 'invalid_audio_preparation',
          balance: 100,
          reserved: 0,
        };
      if (state.preparations.has(input.reservationId)) {
        return {
          ok: false,
          reason: 'invalid_audio_preparation',
          balance: 100,
          reserved: 0,
        };
      }
      state.preparations.set(input.reservationId, 'prepared');
      return reservationSuccess(input);
    },
    async getAudioPreparationStatus(input: {
      requestId: string;
      userId: string;
      credits: number;
    }) {
      state.statusCalls.push(input);
      if (state.preparedLookupSnapshot) return state.preparedLookupSnapshot;
      const preparation = state.preparations.get(input.requestId);
      if (!preparation || preparation === 'failed') return 'missing';
      if (input.userId !== USER_ID || input.credits !== 5) return 'unavailable';
      if (preparation === 'admitted') return 'unavailable';
      return preparation;
    },
    async admitAudioReservation(input: AdmissionInput) {
      state.admissionCalls.push(input);
      state.onAdmissionStarted?.();
      if (state.admissionGate) await state.admissionGate;
      if (state.admissionError) throw state.admissionError;
      if (state.admissionFailure) return state.admissionFailure;
      if (
        state.preparations.get(input.operationId ?? input.reservationId) ===
        'cancelled'
      ) {
        return {
          ok: false,
          reason: 'invalid_audio_preparation',
          balance: 100,
          reserved: 0,
        };
      }
      if (state.preparations.get(input.reservationId) !== 'prepared') {
        return {
          ok: false,
          reason: 'invalid_audio_preparation',
          balance: 100,
          reserved: 0,
        };
      }
      state.preparations.set(input.reservationId, 'admitted');
      state.events.push('admitted');
      return reservationSuccess(input);
    },
    async cancelAudioPreparation(input: CancellationInput) {
      state.cancelCalls.push(input);
      assert.equal(input.userId, USER_ID);
      state.claimEvents.push('cancel_started');
      state.onCancellationStarted?.();
      if (state.cancellationGate) await state.cancellationGate;
      if (!state.preparations.has(input.reservationId)) {
        if (input.reason === 'cancelled' && input.tombstoneMissing)
          state.preparations.set(input.reservationId, 'cancelled');
        state.claimEvents.push('cancel_completed');
        return input.tombstoneMissing ? 'released' : 'not_prepared';
      }
      if (state.preparations.get(input.reservationId) === 'prepared') {
        state.preparations.set(
          input.reservationId,
          input.reason === 'cancelled' ? 'cancelled' : 'failed',
        );
        state.claimEvents.push('cancel_completed');
        return 'released';
      }
      state.claimEvents.push('cancel_completed');
      return 'not_prepared';
    },
    async finalizeReservation(input: FinalizationInput) {
      state.finalizationCalls.push(input);
      if (input.outcome === 'settle') {
        await state.settlementGate;
        assert.equal(state.preparations.get(input.requestId), 'admitted');
        state.settledRows.push(input);
        return 'settled';
      }
      return 'released';
    },
  });
  mockLibrary('tokens', {
    async getUserFromBearer(bearer: string) {
      assert.equal(bearer, 'Bearer nvm_pat_audio_test');
      return {
        id: USER_ID,
        email: 'audio@example.com',
        role: 'user',
        plan: 'free',
      };
    },
  });
  mockLibrary('users', {
    async ensureMonthlyFreeCredits() {
      if (state.creditsError) throw state.creditsError;
    },
  });
  mockLibrary('ratelimit', {
    async rateLimitTranscription() {
      if (state.rateLimitError) throw state.rateLimitError;
      return { ok: true };
    },
    async rateLimitTranscriptionPreparation() {
      state.preparationRateLimitCalls += 1;
      if (state.rateLimitError) throw state.rateLimitError;
      return state.preparationRateLimitDenied
        ? {
            ok: false,
            retryAfterSec: 30,
            scope: 'transcription_preparation:minute',
          }
        : { ok: true };
    },
    async rateLimitTranscriptionCancellationTombstone() {
      state.cancellationRateLimitCalls += 1;
      if (state.rateLimitError) throw state.rateLimitError;
      return state.cancellationRateLimitDenied
        ? {
            ok: false,
            retryAfterSec: 30,
            scope: 'transcription_cancellation:minute',
          }
        : { ok: true };
    },
    tooManyRequests(decision: { retryAfterSec: number }) {
      return Response.json(
        {},
        {
          status: 429,
          headers: { 'Retry-After': String(decision.retryAfterSec) },
        },
      );
    },
  });
  mockLibrary('settings', {
    async getAudioModelRoute() {
      return { provider: 'openrouter', modelId: MODEL_ID };
    },
    async isProviderEnabled() {
      return true;
    },
    ModelNotConfiguredError: class ModelNotConfiguredError extends Error {},
  });
  mockLibrary('route-breakers', {
    async isRouteTripped() {
      if (state.breakerError) throw state.breakerError;
      return false;
    },
  });
  mockLibrary('audio-breaker', {
    handleTrippedAudioRoute: forbiddenLegacyBilling,
  });
  mockLibrary('free-spend-cap', {
    freeSpendCapErrorBody() {
      return { error: { type: 'free_spend_cap_reached' } };
    },
  });
  mockLibrary('cost', {
    usdToCredits(costUsd: number) {
      return Math.max(1, Math.ceil((costUsd * 5) / 0.01));
    },
    lookupModelCost: forbiddenLegacyBilling,
    estimateAudioProviderCostUsd: forbiddenLegacyBilling,
  });
  mockLibrary('transcription-models', {
    async lookupTranscriptionDurationPricing(...args: unknown[]) {
      state.pricingCalls.push(args);
      return state.pricing;
    },
  });
  mockLibrary('audio-workload', {
    TRANSCRIPTION_AUDIO_LIMITS: { inputBytes: 4 * 1024 * 1024 },
    async normalizeTranscriptionAudio(audio: Buffer) {
      state.normalizationCalls.push(audio);
      state.onNormalizationStarted?.();
      if (state.normalizationGate) await state.normalizationGate;
      if (state.normalizationError) throw state.normalizationError;
      return {
        inputAudio: { data: WAV, format: 'wav' },
        durationSeconds: state.estimatedBillableSeconds - 0.75,
        estimatedBillableSeconds: state.estimatedBillableSeconds,
      };
    },
  });
  mockLibrary('proxy', {
    async handleDedup(...args: unknown[]) {
      state.dedupCalls.push(args);
      state.claimEvents.push('claim');
      return null;
    },
    async markDedupFailed(input: FailedDedupInput) {
      state.failedDedupCalls.push(input);
      state.claimEvents.push('dedup_failed');
    },
  });
  mockLibrary('upstream', {
    getUpstreamConfig() {
      return { baseUrl: 'https://openrouter.ai/api/v1', apiKey: 'test_key' };
    },
    UpstreamConfigError: class UpstreamConfigError extends Error {},
  });
  mockLibrary('log', { log: { info() {}, warn() {}, error() {} } });
  t.mock.module('@vercel/functions', {
    namedExports: {
      waitUntil(task: Promise<unknown>) {
        state.backgroundTasks.push(task);
      },
    },
  });
  t.mock.method(
    globalThis,
    'fetch',
    async function upstreamFetch(input: RequestInfo | URL, init?: RequestInit) {
      if (state.upstreamError) throw state.upstreamError;
      assert.equal(
        String(input),
        'https://openrouter.ai/api/v1/audio/transcriptions',
      );
      assert.equal(init?.method, 'POST');
      assert.equal(
        state.events.at(-1),
        'admitted',
        'Provider dispatch requires successful admission',
      );
      const signal = init?.signal as AbortSignal | undefined;
      state.providerSignals.push(signal ?? null);
      if (signal?.aborted)
        throw new DOMException('The request was cancelled.', 'AbortError');
      state.events.push('dispatch');
      state.upstreamBodies.push(JSON.parse(String(init?.body)));
      return Response.json({
        text: 'Released client dictation.',
        usage: { seconds: 12.25, cost: 0.001225 },
      });
    },
  );

  const { POST: transcribe } = await import('./transcriptions');
  const { POST: prepare, DELETE: cancel } = await import(
    './transcription-preparations'
  );

  for (const mode of [
    'prepared',
    'prepared_without_header',
    'direct',
    'missing_preparation',
  ] as const) {
    const prepared = mode === 'prepared' || mode === 'missing_preparation';
    const existingPreparation =
      mode === 'prepared' || mode === 'prepared_without_header';
    for (const legacy of [false, true]) {
      await t.test(
        `desktop ${mode} ${legacy ? 'base64 JSON compatibility' : 'raw WebM'} settles its billing ID`,
        async function successfulTranscription() {
          state = createState();
          if (existingPreparation) {
            const response = await prepare(
              context(audioRequest('transcription-preparations')),
            );
            assert.equal(response.status, 200);
            assert.deepEqual(await response.json(), {
              prepared: true,
              operationId: OPERATION_ID,
            });
            assert.deepEqual(state.prepareCalls, [
              {
                reservationId: OPERATION_ID,
                userId: USER_ID,
                credits: 5,
              },
            ]);
            assert.equal(
              state.pricingCalls.length,
              0,
              'Preparation must not fetch pricing',
            );
            assert.equal(state.normalizationCalls.length, 0);
            assert.equal(state.admissionCalls.length, 0);
            assert.equal(state.upstreamBodies.length, 0);
          }
          const response = await transcribe(
            context(audioRequest('transcriptions', { prepared, legacy })),
          );
          assert.equal(response.status, 200);
          assert.equal(response.headers.get('x-request-id'), CORRELATION_ID);
          const body = await response.json();
          assert.equal(body.text, 'Released client dictation.');
          assert.equal(body.requestId, CORRELATION_ID);
          assert.deepEqual(state.statusCalls, [
            {
              requestId: OPERATION_ID,
              userId: USER_ID,
              credits: 5,
              operationId: OPERATION_ID,
            },
          ]);
          assert.deepEqual(state.pricingCalls, [['openrouter', MODEL_ID]]);
          assert.deepEqual(state.normalizationCalls, [WEBM]);
          assert.equal(state.admissionCalls.length, 1);
          const admission = state.admissionCalls[0]!;
          assert.notEqual(admission.reservationId, CORRELATION_ID);
          if (existingPreparation)
            assert.equal(admission.reservationId, OPERATION_ID);
          else {
            assert.notEqual(admission.reservationId, OPERATION_ID);
            assert.match(
              admission.reservationId,
              /^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/i,
            );
          }
          assert.deepEqual(admission, {
            reservationId: admission.reservationId,
            operationId: OPERATION_ID,
            userId: USER_ID,
            credits: 5,
            estimatedProviderCostUsd: 13 * PRICING.usdPerSecond,
            estimateCostSource: 'catalog_estimate',
          });
          assert.deepEqual(
            state.prepareCalls,
            existingPreparation
              ? [
                  {
                    reservationId: OPERATION_ID,
                    userId: USER_ID,
                    credits: 5,
                  },
                ]
              : [
                  {
                    reservationId: admission.reservationId,
                    operationId: OPERATION_ID,
                    userId: USER_ID,
                    credits: 5,
                  },
                ],
          );
          assert.equal(state.dedupCalls.length, 1);
          assert.equal(state.dedupCalls[0]![0], OPERATION_ID);
          assert.equal(state.dedupCalls[0]![1], USER_ID);
          assert.equal(state.dedupCalls[0]![3], admission.reservationId);
          assert.deepEqual(state.upstreamBodies, [
            {
              model: MODEL_ID,
              input_audio: { data: WAV, format: 'wav' },
              response_format: 'json',
            },
          ]);
          assert.deepEqual(state.events, ['admitted', 'dispatch']);
          assert.equal(state.backgroundTasks.length, 1);
          assert.equal(
            state.settledRows.length,
            0,
            'Response must not await settlement',
          );
          state.resolveSettlement();
          await Promise.all(state.backgroundTasks);
          assert.equal(state.settledRows.length, 1);
          const row = state.settledRows[0]!;
          assert.equal(row.requestId, admission.reservationId);
          assert.equal(row.outcome, 'settle');
          assert.equal(row.model, MODEL_ID);
          assert.equal(row.provider, 'openrouter');
          assert.equal(row.modality, 'audio');
          assert.equal(row.providerCostUsd, 0.001225);
          assert.equal(row.dedup?.userId, USER_ID);
          assert.equal(row.dedup?.idempotencyKey, OPERATION_ID);
          assert.equal(row.dedup?.status, 'completed');
        },
      );
    }
  }

  for (const prepared of [true, false]) {
    for (const failure of ['pricing', 'media', 'admission'] as const) {
      await t.test(
        `${prepared ? 'prepared' : 'direct'} ${failure} failure cannot dispatch`,
        async function rejectedTranscription() {
          state = createState();
          if (prepared) state.preparations.set(OPERATION_ID, 'prepared');
          if (failure === 'pricing') state.pricing = null;
          if (failure === 'media')
            state.normalizationError = new Error(
              'Unsupported or malformed audio',
            );
          if (failure === 'admission') {
            state.admissionFailure = {
              ok: false,
              reason: 'free_spend_cap_reached',
              balance: 100,
              reserved: 5,
            };
          }
          const response = await transcribe(
            context(audioRequest('transcriptions', { prepared })),
          );
          assert.ok(response.status >= 400 && response.status < 600);
          if (failure === 'pricing') {
            assert.equal(response.status, 503);
            assert.equal(
              (await response.json()).error.type,
              'pricing_unavailable',
            );
          }
          if (failure === 'media') {
            assert.equal(response.status, 400);
            assert.equal((await response.json()).error.type, 'invalid_audio');
          }
          if (failure === 'admission') {
            assert.equal(response.status, 503);
            assert.equal(
              (await response.json()).error.type,
              'free_spend_cap_reached',
            );
          }
          if (failure !== 'admission')
            assert.equal(state.admissionCalls.length, 0);
          else assert.equal(state.admissionCalls.length, 1);
          assert.equal(state.upstreamBodies.length, 0);
          assert.equal(state.backgroundTasks.length, 0);
          assert.equal(state.settledRows.length, 0);
          assert.equal(state.failedDedupCalls.length, 1);
          assert.equal(
            state.failedDedupCalls[0]!.requestId,
            state.dedupCalls[0]![3],
          );
          if (prepared || failure === 'admission') {
            assert.deepEqual(state.cancelCalls, [
              {
                reservationId: state.failedDedupCalls[0]!.requestId,
                userId: USER_ID,
                reason: 'failed',
              },
            ]);
            assert.deepEqual(state.claimEvents, [
              'claim',
              'cancel_started',
              'cancel_completed',
              'dedup_failed',
            ]);
          } else {
            assert.equal(state.cancelCalls.length, 0);
            assert.deepEqual(state.claimEvents, ['claim', 'dedup_failed']);
          }
          assert.ok(
            state.finalizationCalls.every(function isRelease(call) {
              return call.outcome === 'release';
            }),
          );
        },
      );
    }
  }

  await t.test(
    'rate-limit rejection releases the existing hold before publishing a retryable dedup claim',
    async function retryAfterRateLimitRejection() {
      state = createState();
      state.preparations.set(OPERATION_ID, 'prepared');
      state.rateLimitError = new Error('Rate-limit service unavailable');
      let resolveCancellation!: () => void;
      let notifyCancellationStarted!: () => void;
      state.cancellationGate = new Promise<void>(
        function captureCancellation(resolve) {
          resolveCancellation = resolve;
        },
      );
      const cancellationStarted = new Promise<void>(
        function captureNotification(resolve) {
          notifyCancellationStarted = resolve;
        },
      );
      state.onCancellationStarted = notifyCancellationStarted;
      const pendingResponse = transcribe(
        context(audioRequest('transcriptions', { prepared: true })),
      );
      await cancellationStarted;
      assert.deepEqual(state.claimEvents, ['claim', 'cancel_started']);
      assert.equal(state.preparations.get(OPERATION_ID), 'prepared');
      assert.equal(
        state.failedDedupCalls.length,
        0,
        'Retry cannot be published while the old hold remains reusable',
      );
      assert.equal(state.upstreamBodies.length, 0);
      resolveCancellation();
      const response = await pendingResponse;
      assert.equal(response.status, 503);
      assert.equal((await response.json()).error.type, 'service_unavailable');
      assert.equal(response.headers.get('x-request-id'), CORRELATION_ID);
      assert.deepEqual(state.cancelCalls, [
        { reservationId: OPERATION_ID, userId: USER_ID, reason: 'failed' },
      ]);
      assert.deepEqual(state.claimEvents, [
        'claim',
        'cancel_started',
        'cancel_completed',
        'dedup_failed',
      ]);
      assert.equal(state.preparations.get(OPERATION_ID), 'failed');
      assert.equal(state.failedDedupCalls[0]!.requestId, OPERATION_ID);
      assert.equal(state.prepareCalls.length, 0);
      assert.equal(state.admissionCalls.length, 0);
      assert.equal(state.finalizationCalls.length, 0);

      state.rateLimitError = null;
      state.cancellationGate = null;
      state.onCancellationStarted = null;
      const retry = await transcribe(
        context(audioRequest('transcriptions', { prepared: true })),
      );
      assert.equal(
        retry.status,
        200,
        'Internal failure must not become permanent user cancellation',
      );
      const retryId = state.admissionCalls[0]!.reservationId;
      assert.notEqual(retryId, OPERATION_ID);
      assert.notEqual(retryId, CORRELATION_ID);
      assert.equal(state.dedupCalls[1]![3], retryId);
      assert.deepEqual(state.prepareCalls, [
        {
          reservationId: retryId,
          operationId: OPERATION_ID,
          userId: USER_ID,
          credits: 5,
        },
      ]);
      assert.equal(state.upstreamBodies.length, 1);
      state.resolveSettlement();
      await Promise.all(state.backgroundTasks);
      assert.equal(state.settledRows[0]!.requestId, retryId);
      assert.equal(state.preparations.get(OPERATION_ID), 'failed');
    },
  );

  for (const prepared of [true, false]) {
    for (const failure of [
      'credits',
      'breaker',
      'preparation',
      'admission',
    ] as const) {
      if (prepared && (failure === 'credits' || failure === 'preparation'))
        continue;
      await t.test(
        `${prepared ? 'prepared' : 'direct'} thrown ${failure} failure cleans its post-claim state`,
        async function postClaimFailure() {
          state = createState();
          if (prepared) state.preparations.set(OPERATION_ID, 'prepared');
          const error = new Error(`${failure} dependency rejected`);
          if (failure === 'credits') state.creditsError = error;
          if (failure === 'breaker') state.breakerError = error;
          if (failure === 'preparation') state.preparationError = error;
          if (failure === 'admission') state.admissionError = error;
          const response = await transcribe(
            context(audioRequest('transcriptions', { prepared })),
          );
          assert.equal(response.status, 503);
          assert.equal(
            (await response.json()).error.type,
            'service_unavailable',
          );
          assert.equal(state.failedDedupCalls.length, 1);
          const billingId = state.failedDedupCalls[0]!.requestId;
          assert.equal(billingId, state.dedupCalls[0]![3]);
          assert.equal(state.upstreamBodies.length, 0);
          assert.equal(state.backgroundTasks.length, 0);
          assert.equal(state.finalizationCalls.length, 0);
          if (prepared || failure === 'admission') {
            assert.deepEqual(state.cancelCalls, [
              { reservationId: billingId, userId: USER_ID, reason: 'failed' },
            ]);
            assert.equal(state.preparations.get(billingId), 'failed');
            assert.deepEqual(state.claimEvents, [
              'claim',
              'cancel_started',
              'cancel_completed',
              'dedup_failed',
            ]);
          } else {
            assert.equal(state.cancelCalls.length, 0);
            assert.deepEqual(state.claimEvents, ['claim', 'dedup_failed']);
          }
        },
      );
    }
  }

  for (const preparationStatus of ['cancelled', 'unavailable'] as const) {
    for (const prepared of [true, false]) {
      await t.test(
        `${preparationStatus} operation is rejected before claiming, with preparation header ${prepared}`,
        async function terminalPreparation() {
          state = createState();
          // Unavailable represents an untagged pending legacy reservation, not a
          // reusable provisional audio hold. Neither status permits UUID fallback.
          state.preparations.set(OPERATION_ID, preparationStatus);
          const response = await transcribe(
            context(audioRequest('transcriptions', { prepared })),
          );
          assert.equal(response.status, 409);
          assert.equal(
            (await response.json()).error.type,
            'idempotency_conflict',
          );
          assert.equal(response.headers.get('x-request-id'), CORRELATION_ID);
          assert.equal(state.statusCalls.length, 1);
          assert.equal(state.dedupCalls.length, 0);
          assert.equal(state.failedDedupCalls.length, 0);
          assert.equal(
            state.prepareCalls.length,
            0,
            'Do not create a second hold for pending legacy or cancelled operations',
          );
          assert.equal(state.admissionCalls.length, 0);
          assert.equal(state.cancelCalls.length, 0);
          assert.equal(state.finalizationCalls.length, 0);
          assert.equal(state.pricingCalls.length, 0);
          assert.equal(state.normalizationCalls.length, 0);
          assert.equal(state.upstreamBodies.length, 0);
          assert.equal(state.preparations.size, 1);
          assert.equal(state.preparations.get(OPERATION_ID), preparationStatus);
        },
      );
    }
  }

  await t.test(
    'cancellation before preparation creates a tombstone that blocks late prepare and transcription',
    async function cancellationBeforePrepare() {
      state = createState();
      state.preparationRateLimitDenied = true;
      const preparation = await prepare(
        context(audioRequest('transcription-preparations')),
      );
      assert.equal(preparation.status, 429);
      assert.equal(preparation.headers.get('x-request-id'), CORRELATION_ID);
      assert.equal(preparation.headers.get('Retry-After'), '30');
      const cancellation = await cancel(
        context(
          audioRequest('transcription-preparations', {
            method: 'DELETE',
            cancellationReason: 'user_cancel',
          }),
        ),
      );
      assert.equal(cancellation.status, 200);
      assert.equal(state.preparations.get(OPERATION_ID), 'cancelled');
      assert.equal(state.preparationRateLimitCalls, 1);
      assert.equal(state.cancellationRateLimitCalls, 1);

      state.preparationRateLimitDenied = false;
      const latePreparation = await prepare(
        context(audioRequest('transcription-preparations')),
      );
      assert.equal(latePreparation.status, 409);
      assert.equal(
        (await latePreparation.json()).error.type,
        'idempotency_conflict',
      );
      assert.equal(state.prepareCalls.length, 1);

      const transcription = await transcribe(
        context(audioRequest('transcriptions', { prepared: true })),
      );
      assert.equal(transcription.status, 409);
      assert.equal(state.dedupCalls.length, 0);
      assert.equal(state.upstreamBodies.length, 0);
    },
  );

  await t.test(
    'tombstone rate limit returns an explicit compatible 429 when no hold exists',
    async function tombstoneRateLimit() {
      state = createState();
      state.cancellationRateLimitDenied = true;
      const response = await cancel(
        context(
          audioRequest('transcription-preparations', {
            method: 'DELETE',
            cancellationReason: 'user_cancel',
          }),
        ),
      );
      assert.equal(response.status, 429);
      assert.equal(response.headers.get('x-request-id'), CORRELATION_ID);
      assert.equal(response.headers.get('Retry-After'), '30');
      assert.equal(state.cancellationRateLimitCalls, 1);
      assert.equal(state.cancelCalls.length, 2);
      assert.equal(state.preparations.has(OPERATION_ID), false);
    },
  );

  await t.test(
    'legacy cleanup after failed preparation does not tombstone a v0.21.3 same-ID retry',
    async function legacyCleanupAllowsRetry() {
      state = createState();
      state.preparationError = new Error('Preparation temporarily unavailable');
      const preparation = await prepare(
        context(
          audioRequest('transcription-preparations', {
            clientVersion: '0.21.3',
          }),
        ),
      );
      assert.equal(preparation.status, 503);

      const cleanup = await cancel(
        context(
          audioRequest('transcription-preparations', {
            method: 'DELETE',
            clientVersion: '0.21.3',
          }),
        ),
      );
      assert.equal(cleanup.status, 200);
      assert.deepEqual(await cleanup.json(), {
        released: false,
        operationId: OPERATION_ID,
      });
      assert.equal(state.preparations.has(OPERATION_ID), false);
      assert.deepEqual(state.cancelCalls, [
        {
          reservationId: OPERATION_ID,
          userId: USER_ID,
          reason: 'failed',
        },
      ]);

      state.preparationError = null;
      const retry = await transcribe(
        context(
          audioRequest('transcriptions', {
            legacy: true,
            clientVersion: '0.21.3',
          }),
        ),
      );
      assert.equal(retry.status, 200, await retry.clone().text());
      assert.equal(state.admissionCalls.length, 1);
      assert.equal(state.admissionCalls[0]?.operationId, OPERATION_ID);
      assert.notEqual(state.admissionCalls[0]?.reservationId, OPERATION_ID);
      assert.equal(state.upstreamBodies.length, 1);
      assert.equal(state.backgroundTasks.length, 1);
      state.resolveSettlement();
      await Promise.all(state.backgroundTasks);
      assert.equal(state.settledRows.length, 1);
    },
  );

  await t.test(
    'explicit cancellation racing fallback preparation blocks before a billing hold is created',
    async function cancellationBeforeFallbackPrepare() {
      state = createState();
      let markNormalizationStarted!: () => void;
      const normalizationStarted = new Promise<void>(
        function startGate(resolve) {
          markNormalizationStarted = resolve;
        },
      );
      let resumeNormalization!: () => void;
      state.normalizationGate = new Promise<void>(
        function holdNormalization(resolve) {
          resumeNormalization = resolve;
        },
      );
      state.onNormalizationStarted = markNormalizationStarted;

      const transcription = transcribe(
        context(
          audioRequest('transcriptions', { cancellationId: OPERATION_ID }),
        ),
      );
      await normalizationStarted;
      const cancellation = await cancel(
        context(
          audioRequest('transcription-preparations', {
            method: 'DELETE',
            cancellationReason: 'user_cancel',
          }),
        ),
      );
      assert.equal(cancellation.status, 200);
      resumeNormalization();

      const response = await transcription;
      assert.equal(response.status, 409);
      assert.equal(state.prepareCalls.length, 1);
      assert.notEqual(state.prepareCalls[0]?.reservationId, OPERATION_ID);
      assert.equal(state.prepareCalls[0]?.operationId, OPERATION_ID);
      assert.equal(state.admissionCalls.length, 0);
      assert.equal(state.upstreamBodies.length, 0);
      assert.equal(state.preparations.get(OPERATION_ID), 'cancelled');
    },
  );

  await t.test(
    'explicit cancellation racing fallback admission releases the provisional billing hold',
    async function cancellationBeforeFallbackAdmission() {
      state = createState();
      let markAdmissionStarted!: () => void;
      const admissionStarted = new Promise<void>(function startGate(resolve) {
        markAdmissionStarted = resolve;
      });
      let resumeAdmission!: () => void;
      state.admissionGate = new Promise<void>(function holdAdmission(resolve) {
        resumeAdmission = resolve;
      });
      state.onAdmissionStarted = markAdmissionStarted;

      const transcription = transcribe(
        context(
          audioRequest('transcriptions', { cancellationId: OPERATION_ID }),
        ),
      );
      await admissionStarted;
      const billingId = state.prepareCalls[0]?.reservationId;
      assert.ok(billingId);
      assert.notEqual(billingId, OPERATION_ID);
      const cancellation = await cancel(
        context(
          audioRequest('transcription-preparations', {
            method: 'DELETE',
            cancellationReason: 'user_cancel',
          }),
        ),
      );
      assert.equal(cancellation.status, 200);
      resumeAdmission();

      const response = await transcription;
      assert.equal(response.status, 409);
      assert.equal(state.preparations.get(OPERATION_ID), 'cancelled');
      assert.equal(state.preparations.get(billingId), 'failed');
      assert.equal(state.upstreamBodies.length, 0);
    },
  );

  await t.test(
    'request cancellation signal reaches provider dispatch and prevents a network call',
    async function abortsProviderDispatch() {
      state = createState();
      const controller = new AbortController();
      let markAdmissionStarted!: () => void;
      const admissionStarted = new Promise<void>(function startGate(resolve) {
        markAdmissionStarted = resolve;
      });
      let resumeAdmission!: () => void;
      state.admissionGate = new Promise<void>(function holdAdmission(resolve) {
        resumeAdmission = resolve;
      });
      state.onAdmissionStarted = markAdmissionStarted;

      const transcription = transcribe(
        context(
          audioRequest('transcriptions', {
            signal: controller.signal,
          }),
        ),
      );
      await admissionStarted;
      controller.abort();
      resumeAdmission();

      const response = await transcription;
      assert.equal(response.status, 503);
      assert.equal(state.providerSignals.length, 1);
      assert.equal(state.providerSignals[0]?.aborted, true);
      assert.deepEqual(state.events, ['admitted']);
      assert.equal(state.upstreamBodies.length, 0);
    },
  );

  await t.test(
    'provider rejection uses transactional finalization, not provisional cleanup',
    async function providerFailure() {
      state = createState();
      state.preparations.set(OPERATION_ID, 'prepared');
      state.upstreamError = new Error('Provider connection failed');
      const response = await transcribe(
        context(audioRequest('transcriptions')),
      );
      assert.equal(response.status, 503);
      assert.equal((await response.json()).error.type, 'service_unavailable');
      assert.equal(state.admissionCalls.length, 1);
      assert.equal(state.cancelCalls.length, 0);
      assert.equal(state.failedDedupCalls.length, 0);
      assert.equal(state.finalizationCalls.length, 1);
      const finalization = state.finalizationCalls[0]!;
      assert.equal(finalization.requestId, OPERATION_ID);
      assert.equal(finalization.outcome, 'release');
      assert.equal(finalization.dedup?.status, 'failed');
      assert.equal(finalization.dedup?.idempotencyKey, OPERATION_ID);
    },
  );

  await t.test(
    'audio whose estimate exceeds the preserved five-credit ceiling is rejected before dispatch',
    async function rejectsOverBudgetAudio() {
      state = createState();
      state.preparations.set(OPERATION_ID, 'prepared');
      state.estimatedBillableSeconds = 101;
      const response = await transcribe(
        context(audioRequest('transcriptions')),
      );
      assert.equal(response.status, 413);
      assert.equal((await response.json()).error.type, 'audio_cost_limit');
      assert.equal(state.admissionCalls.length, 0);
      assert.deepEqual(state.cancelCalls, [
        {
          reservationId: OPERATION_ID,
          userId: USER_ID,
          reason: 'failed',
        },
      ]);
      assert.equal(state.upstreamBodies.length, 0);
    },
  );

  await t.test(
    'cancellation delegates to the provisional helper; a stale prepared lookup cannot claim a cancelled operation',
    async function cancelledPreparation() {
      state = createState();
      assert.equal(
        (await prepare(context(audioRequest('transcription-preparations'))))
          .status,
        200,
      );
      // Model a lookup that observed preparation before DELETE won the claim race.
      state.preparedLookupSnapshot = 'prepared';
      const response = await cancel(
        context(
          audioRequest('transcription-preparations', {
            method: 'DELETE',
            cancellationReason: 'user_cancel',
          }),
        ),
      );
      assert.equal(response.status, 200);
      assert.deepEqual(await response.json(), {
        released: true,
        operationId: OPERATION_ID,
      });
      assert.deepEqual(state.cancelCalls, [
        {
          reservationId: OPERATION_ID,
          userId: USER_ID,
          reason: 'cancelled',
        },
      ]);
      assert.equal(
        state.finalizationCalls.length,
        0,
        'DELETE must not use generic finalization',
      );
      assert.equal(state.pricingCalls.length, 0);
      assert.equal(state.upstreamBodies.length, 0);
      const transcription = await transcribe(
        context(audioRequest('transcriptions', { prepared: true })),
      );
      assert.equal(transcription.status, 409);
      assert.equal(state.admissionCalls.length, 1);
      assert.equal(state.admissionCalls[0]!.reservationId, OPERATION_ID);
      assert.equal(state.preparations.get(OPERATION_ID), 'cancelled');
      assert.equal(state.upstreamBodies.length, 0);
      assert.equal(state.settledRows.length, 0);
    },
  );
});

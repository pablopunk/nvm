import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http';
import { setTimeout as delay } from 'node:timers/promises';
import { and, eq } from 'drizzle-orm';
import { type Database, getDb } from '../src/db/client';
import {
  appSettings,
  creditLedger,
  creditReservations,
  freeSpendCapConfig,
  freeSpendMonthlyBudgets,
  providers,
  requestDedup,
  usage,
  users,
} from '../src/db/schema';
import { usdToCredits, usdToMicrocents } from '../src/lib/cost';
import {
  ensureFreeSpendMonthlyBudget,
  utcMonthKey,
} from '../src/lib/free-spend-cap';
import { createApiToken } from '../src/lib/tokens';
import { refreshTranscriptionCatalog, resetTranscriptionModelCacheForTests } from '../src/lib/transcription-models';
import {
  ensureMonthlyFreeCredits,
  MONTHLY_FREE_CREDITS,
} from '../src/lib/users';

const MODEL = 'nvidia/parakeet-tdt-0.6b-v3';
const USD_PER_SECOND = 0.000025;
const SAMPLES = 53_760;
const SECONDS = SAMPLES / 16_000;
const ESTIMATE = Math.ceil(SECONDS) * USD_PER_SECOND;
const PROVIDER_COST = SECONDS * USD_PER_SECOND;
const TEXT = 'Loopback transcription of the Chromium speech fixture.';

/** Real routes and decoding, isolated PostgreSQL billing; only the provider is stubbed. */
export async function runAudioJourneyAssertions(
  database: Database,
  suffix: string,
) {
  assert.equal(getDb(), database, 'journey must run inside withTestDb');
  assert.equal(process.env.NVM_DB_DRIVER, 'postgres');
  assert.ok(process.env.NVM_TEST_ADMIN_DATABASE_URL);
  const target = new URL(process.env.DATABASE_URL ?? '');
  assert.match(target.pathname, /^\/nvm_test_[a-zA-Z0-9_]+$/);
  assert.ok(
    ['localhost', '127.0.0.1', '::1', 'postgres'].includes(target.hostname),
  );
  assert.notEqual(process.env.VERCEL_ENV, 'production');
  assert.notEqual(process.env.NODE_ENV, 'production');
  for (const key of [
    'UPSTASH_REDIS_REST_URL',
    'UPSTASH_REDIS_REST_TOKEN',
    'KV_REST_API_URL',
    'KV_REST_API_TOKEN',
    'AXIOM_TOKEN',
  ])
    assert.ok(
      !process.env[key],
      `${key} must be absent: no external rate-limit or logging calls`,
    );
  assert.equal(
    MONTHLY_FREE_CREDITS,
    500,
    'journey requires the default free grant',
  );
  assert.equal(
    Number(process.env.AUDIO_TRANSCRIPTION_RESERVATION_CREDITS ?? 5),
    5,
  );

  const [{ POST: prepare }, { POST: transcribe }] = await Promise.all([
    import('../src/pages/api/v1/audio/transcription-preparations'),
    import('../src/pages/api/v1/audio/transcriptions'),
  ]);
  const recording = await readFile(
    new URL(
      ['..', 'src', 'fixtures', 'audio', 'chromium-speech.webm'].join('/'),
      import.meta.url,
    ),
  );
  const [savedRoute] = await database
    .select()
    .from(appSettings)
    .where(eq(appSettings.key, 'audio_model_route'));
  const [savedProvider] = await database
    .select()
    .from(providers)
    .where(eq(providers.id, 'openrouter'));
  const [savedCap] = await database
    .select()
    .from(freeSpendCapConfig)
    .where(eq(freeSpendCapConfig.id, 'global'));
  assert.ok(savedCap);
  const monthKey = utcMonthKey(new Date());
  const savedBudget = await database.transaction(
    function materializeBudget(tx) {
      return ensureFreeSpendMonthlyBudget(tx, monthKey, savedCap.capMicrocents);
    },
  );
  const savedBaseUrl = process.env.OPENROUTER_BASE_URL;
  const savedApiKey = process.env.OPENROUTER_API_KEY;
  let activeOperationId = '';
  let baselineReserved = savedBudget.reservedFreeCostMicrocents;
  let providerCalls = 0;
  let catalogCalls = 0;
  let stubFailure: unknown;

  async function readBudget() {
    const [budget] = await database
      .select()
      .from(freeSpendMonthlyBudgets)
      .where(eq(freeSpendMonthlyBudgets.monthKey, monthKey));
    assert.ok(budget);
    return budget;
  }

  async function serveProvider(
    request: IncomingMessage,
    response: ServerResponse,
  ) {
    try {
      const url = new URL(request.url ?? '/', 'http://127.0.0.1');
      response.setHeader('content-type', 'application/json');
      if (request.method === 'GET' && url.pathname === '/api/v1/models') {
        assert.equal(request.headers.authorization, undefined);
        assert.equal(url.searchParams.get('input_modalities'), 'audio');
        assert.equal(
          url.searchParams.get('output_modalities'),
          'transcription',
        );
        catalogCalls += 1;
        response.end(
          JSON.stringify({
            data: [
              {
                id: MODEL,
                name: 'Parakeet fixture',
                pricing: { prompt: '0.000025', completion: '0' },
                architecture: {
                  input_modalities: ['audio'],
                  output_modalities: ['transcription'],
                },
              },
            ],
          }),
        );
        return;
      }
      assert.equal(
        request.headers.authorization,
        'Bearer integration-test-key',
      );
      assert.equal(request.method, 'POST');
      assert.equal(url.pathname, '/api/v1/audio/transcriptions');
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString()) as {
        model: string;
        response_format: string;
        input_audio: { format: string; data: string };
      };
      assert.equal(body.model, MODEL);
      assert.equal(body.response_format, 'json');
      assert.equal(body.input_audio.format, 'wav');
      const wav = Buffer.from(body.input_audio.data, 'base64');
      assert.equal(wav.toString('ascii', 0, 4), 'RIFF');
      assert.equal(wav.toString('ascii', 8, 16), 'WAVEfmt ');
      assert.equal(wav.readUInt32LE(4), wav.length - 8);
      assert.equal(wav.readUInt32LE(16), 16);
      assert.equal(wav.readUInt16LE(20), 1);
      assert.equal(wav.readUInt16LE(22), 1);
      assert.equal(wav.readUInt32LE(24), 16_000);
      assert.equal(wav.readUInt32LE(28), 16_000 * 2);
      assert.equal(wav.readUInt16LE(32), 2);
      assert.equal(wav.readUInt16LE(34), 16);
      assert.equal(wav.toString('ascii', 36, 40), 'data');
      assert.equal(wav.readUInt32LE(40), SAMPLES * 2);
      assert.equal(wav.length, 44 + SAMPLES * 2);
      assert.ok(
        wav.subarray(44).some(function isSpeechByte(byte) {
          return byte !== 0;
        }),
      );
      const [hold] = await database
        .select()
        .from(creditReservations)
        .where(eq(creditReservations.requestId, activeOperationId));
      assert.ok(hold);
      assert.equal(hold.status, 'pending');
      assert.equal(hold.audioState, 'dispatched');
      assert.equal(hold.reservedCredits, 5);
      assert.equal(hold.reservedFreeCredits, 5);
      assert.equal(hold.reservedPaidCredits, 0);
      assert.equal(
        hold.estimatedProviderCostMicrocents,
        usdToMicrocents(ESTIMATE),
      );
      assert.equal(hold.reservedFreeCostMicrocents, usdToMicrocents(ESTIMATE));
      assert.equal(hold.admissionMonth, monthKey);
      assert.equal(
        (await readBudget()).reservedFreeCostMicrocents,
        baselineReserved + usdToMicrocents(ESTIMATE),
      );
      providerCalls += 1;
      response.end(
        JSON.stringify({
          text: TEXT,
          usage: { seconds: SECONDS, cost: PROVIDER_COST },
        }),
      );
    } catch (error) {
      stubFailure = error;
      response.statusCode = 500;
      response.end(
        JSON.stringify({ error: 'integration provider assertion failed' }),
      );
    }
  }

  const server = createServer(function handleProvider(request, response) {
    void serveProvider(request, response);
  });
  try {
    await new Promise<void>(function listen(resolve, reject) {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const address = server.address();
    assert.ok(address && typeof address !== 'string');
    process.env.OPENROUTER_BASE_URL = `http://127.0.0.1:${address.port}/api/v1`;
    process.env.OPENROUTER_API_KEY = 'integration-test-key';
    resetTranscriptionModelCacheForTests();
    await refreshTranscriptionCatalog();
    await database
      .insert(appSettings)
      .values({
        key: 'audio_model_route',
        value: JSON.stringify({ provider: 'openrouter', modelId: MODEL }),
      })
      .onConflictDoUpdate({
        target: appSettings.key,
        set: {
          value: JSON.stringify({ provider: 'openrouter', modelId: MODEL }),
        },
      });
    await database
      .insert(providers)
      .values({ id: 'openrouter', displayName: 'OpenRouter', enabled: 'true' })
      .onConflictDoUpdate({ target: providers.id, set: { enabled: 'true' } });
    await database
      .update(freeSpendCapConfig)
      .set({ capMicrocents: null })
      .where(eq(freeSpendCapConfig.id, 'global'));
    await database
      .update(freeSpendMonthlyBudgets)
      .set({ capMicrocents: null, halted: false })
      .where(eq(freeSpendMonthlyBudgets.monthKey, monthKey));

    const [user] = await database
      .insert(users)
      .values({
        workosUserId: `audio-journey-${suffix}`,
        email: `audio-journey-${suffix}@example.test`,
      })
      .returning();
    assert.ok(user);
    await database.insert(creditLedger).values([
      {
        userId: user.id,
        delta: 500,
        kind: 'free',
        reason: 'grant_free_monthly',
        refId: monthKey,
      },
      {
        userId: user.id,
        delta: 10,
        kind: 'paid',
        reason: 'integration.audio_journey_paid',
        refId: suffix,
      },
    ]);
    await ensureMonthlyFreeCredits(user.id);
    const token = await createApiToken(user.id, 'audio journey');
    const charge = usdToCredits(PROVIDER_COST);
    // Both tagged desktop clients use raw WebM. The final entry independently
    // covers the legacy base64 JSON envelope accepted by the route.
    const journeyCases = [
      { version: '0.22.1', base64Json: false },
      { version: '0.21.3', base64Json: false },
      { version: '0.21.3', base64Json: true },
    ];
    for (const [index, journeyCase] of journeyCases.entries()) {
      const { version, base64Json } = journeyCase;
      activeOperationId = `audio-journey-${index}-${suffix}`;
      const correlationId = `audio-correlation-${index}-${suffix}`;
      assert.notEqual(correlationId, activeOperationId);
      const headers = {
        authorization: `Bearer ${token.token}`,
        'x-nevermind-client': 'desktop',
        'x-nevermind-client-version': version,
        'x-nevermind-api-version': '1',
        'x-nevermind-platform': 'darwin',
        'x-nevermind-arch': 'arm64',
        'x-request-id': correlationId,
      };
      const before = await readBudget();
      baselineReserved = before.reservedFreeCostMicrocents;
      const preparationResponse = await prepare({
        request: new Request(
          'http://localhost/api/v1/audio/transcription-preparations',
          {
            method: 'POST',
            headers: { ...headers, 'content-type': 'application/json' },
            body: JSON.stringify({ operationId: activeOperationId }),
          },
        ),
      } as Parameters<typeof prepare>[0]);
      assert.ok(preparationResponse instanceof Response);
      assert.equal(
        preparationResponse.status,
        200,
        await preparationResponse.clone().text(),
      );
      assert.deepEqual(await preparationResponse.json(), {
        prepared: true,
        operationId: activeOperationId,
      });
      const [prepared] = await database
        .select()
        .from(creditReservations)
        .where(eq(creditReservations.requestId, activeOperationId));
      assert.ok(prepared);
      assert.equal(prepared.audioState, 'prepared');
      assert.equal(prepared.reservedCredits, 5);
      assert.equal(prepared.estimatedProviderCostMicrocents, null);
      assert.equal(prepared.reservedFreeCostMicrocents, 0);
      assert.equal(
        (await readBudget()).reservedFreeCostMicrocents,
        baselineReserved,
      );

      const response = await transcribe({
        request: new Request('http://localhost/api/v1/audio/transcriptions', {
          method: 'POST',
          headers: {
            ...headers,
            'idempotency-key': activeOperationId,
            'content-type': base64Json ? 'application/json' : 'audio/webm',
          },
          body: base64Json
            ? JSON.stringify({
                input_audio: {
                  data: recording.toString('base64'),
                  format: 'webm',
                },
              })
            : new Uint8Array(recording),
        }),
      } as Parameters<typeof transcribe>[0]);
      if (stubFailure) throw stubFailure;
      assert.ok(response instanceof Response);
      assert.equal(response.status, 200, await response.clone().text());
      assert.equal(response.headers.get('x-request-id'), correlationId);
      assert.deepEqual(await response.json(), {
        text: TEXT,
        requestId: correlationId,
        usage: { seconds: SECONDS },
      });

      // waitUntil is real; outside Vercel the promise still runs. Require durable convergence.
      const deadline = Date.now() + 5_000;
      for (;;) {
        const [reservation] = await database
          .select()
          .from(creditReservations)
          .where(eq(creditReservations.requestId, activeOperationId));
        const [dedup] = await database
          .select()
          .from(requestDedup)
          .where(
            and(
              eq(requestDedup.userId, user.id),
              eq(requestDedup.idempotencyKey, activeOperationId),
            ),
          );
        if (
          reservation?.status === 'settled' &&
          dedup?.status === 'completed'
        ) {
          assert.equal(dedup.requestId, activeOperationId);
          assert.equal(dedup.upstreamStatus, 200);
          break;
        }
        assert.ok(
          Date.now() < deadline,
          `audio settlement did not converge: ${activeOperationId}`,
        );
        await delay(20);
      }
      const [settled] = await database
        .select()
        .from(creditReservations)
        .where(eq(creditReservations.requestId, activeOperationId));
      assert.ok(settled);
      assert.equal(settled.actualCredits, charge);
      assert.equal(settled.actualFreeCredits, charge);
      assert.equal(settled.actualPaidCredits, 0);
      assert.equal(
        settled.actualFreeCostMicrocents,
        usdToMicrocents(PROVIDER_COST),
      );
      const debits = await database
        .select()
        .from(creditLedger)
        .where(
          and(
            eq(creditLedger.userId, user.id),
            eq(creditLedger.reason, 'ai_usage'),
          ),
        );
      assert.equal(debits.length, index + 1);
      assert.equal(
        debits.find(function isThisDebit(row) {
          return row.refId === `${activeOperationId}:free`;
        })?.delta,
        -charge,
      );
      assert.ok(
        debits.every(function isFreeDebit(row) {
          return row.kind === 'free';
        }),
      );
      const records = await database
        .select()
        .from(usage)
        .where(eq(usage.requestId, activeOperationId));
      assert.equal(records.length, 1);
      assert.equal(records[0].model, MODEL);
      assert.equal(records[0].provider, 'openrouter');
      assert.equal(records[0].modality, 'audio');
      assert.equal(records[0].audioDurationMs, Math.round(SECONDS * 1000));
      assert.equal(records[0].status, 200);
      assert.equal(records[0].costCredits, charge);
      assert.equal(records[0].upstreamCostSource, 'provider_reported');
      assert.equal(
        records[0].upstreamCostMicrocents,
        usdToMicrocents(PROVIDER_COST),
      );
      assert.equal(records[0].costOverrun, false);
      const after = await readBudget();
      assert.equal(after.reservedFreeCostMicrocents, baselineReserved);
      assert.equal(
        after.actualFreeCostMicrocents,
        before.actualFreeCostMicrocents + usdToMicrocents(PROVIDER_COST),
      );
    }
    assert.equal(providerCalls, journeyCases.length);
    assert.equal(
      catalogCalls,
      1,
      'the seeded snapshot serves the journey without request-time catalog fetches',
    );
    const grants = await database
      .select()
      .from(creditLedger)
      .where(
        and(
          eq(creditLedger.userId, user.id),
          eq(creditLedger.reason, 'grant_free_monthly'),
        ),
      );
    assert.equal(
      grants.length,
      1,
      'preparation must not duplicate the seeded monthly grant',
    );
    assert.equal(grants[0].delta, 500);
  } finally {
    if (savedBaseUrl === undefined) delete process.env.OPENROUTER_BASE_URL;
    else process.env.OPENROUTER_BASE_URL = savedBaseUrl;
    if (savedApiKey === undefined) delete process.env.OPENROUTER_API_KEY;
    else process.env.OPENROUTER_API_KEY = savedApiKey;
    resetTranscriptionModelCacheForTests();
    server.closeAllConnections();
    await new Promise<void>(function close(resolve) {
      server.close(function closed() {
        resolve();
      });
    });
    if (savedRoute)
      await database
        .update(appSettings)
        .set(savedRoute)
        .where(eq(appSettings.key, 'audio_model_route'));
    else
      await database
        .delete(appSettings)
        .where(eq(appSettings.key, 'audio_model_route'));
    if (savedProvider)
      await database
        .update(providers)
        .set(savedProvider)
        .where(eq(providers.id, 'openrouter'));
    else await database.delete(providers).where(eq(providers.id, 'openrouter'));
    await database
      .update(freeSpendCapConfig)
      .set(savedCap)
      .where(eq(freeSpendCapConfig.id, 'global'));
    await database
      .update(freeSpendMonthlyBudgets)
      .set({
        capMicrocents: savedBudget.capMicrocents,
        halted: savedBudget.halted,
      })
      .where(eq(freeSpendMonthlyBudgets.monthKey, monthKey));
  }
}

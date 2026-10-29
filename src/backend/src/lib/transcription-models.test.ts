import assert from 'node:assert/strict';
import { afterEach, beforeEach, test } from 'node:test';
import { expirePricingCacheForTests, resetPricingCacheForTests } from './pricing';
import { setStaleAdmissionsHaltedForTests } from './pricing-overrun-cap';
import {
  SNAPSHOT_CURRENT_MAX_AGE_MS,
  SNAPSHOT_MAX_AGE_MS,
} from './pricing-snapshot-store';
import {
  listTranscriptionModels,
  lookupTranscriptionDurationPricing,
  refreshTranscriptionCatalog,
} from './transcription-models';

const PARAKEET = 'nvidia/parakeet-tdt-0.6b-v3';
const PARAKEET_CATALOG = {
  data: [
    {
      id: PARAKEET,
      name: 'Parakeet',
      pricing: { prompt: '0.000025', completion: '0' },
      architecture: { input_modalities: ['audio'], output_modalities: ['transcription'] },
    },
    {
      id: 'text-only',
      architecture: { input_modalities: ['text'], output_modalities: ['text'] },
    },
  ],
};
const PARAKEET_PRICING = {
  unit: 'second',
  usdPerSecond: 0.000025,
  source: 'openrouter_transcription_catalog',
};

let originalFetch: typeof globalThis.fetch;
let requests: Array<{ url: string; authorization: string | null }>;

function answerCatalogWith(body: unknown, status = 200) {
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    requests.push({
      url: String(input instanceof Request ? input.url : input),
      authorization: new Headers(init?.headers).get('authorization'),
    });
    return Response.json(body, { status });
  }) as unknown as typeof globalThis.fetch;
}

beforeEach(() => {
  originalFetch = globalThis.fetch;
  requests = [];
  resetPricingCacheForTests();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

test('a refresh publishes the audio-to-transcription catalog and lists only its models, without a key', async () => {
  answerCatalogWith(PARAKEET_CATALOG);
  await refreshTranscriptionCatalog();

  assert.deepEqual(await listTranscriptionModels(), [
    {
      provider: 'openrouter',
      modelId: PARAKEET,
      name: 'Parakeet',
      durationPricing: PARAKEET_PRICING,
    },
  ]);
  assert.equal(requests.length, 1);
  assert.match(requests[0].url, /\/models\?input_modalities=audio&output_modalities=transcription$/);
  assert.equal(requests[0].authorization, null);
});

test('lookups read the validated catalog and never reach the network', async () => {
  answerCatalogWith(PARAKEET_CATALOG);
  await refreshTranscriptionCatalog();
  const requestsBeforeLookups = requests.length;

  assert.deepEqual(
    await lookupTranscriptionDurationPricing('openrouter', PARAKEET),
    PARAKEET_PRICING,
  );
  assert.equal(await lookupTranscriptionDurationPricing('openrouter', 'text-only'), null);
  assert.equal(await lookupTranscriptionDurationPricing('openai', PARAKEET), null);
  assert.equal(requests.length, requestsBeforeLookups);
});

test('an unvalidated catalog lists and prices nothing, and a lookup never fetches it', async () => {
  answerCatalogWith(PARAKEET_CATALOG);

  assert.deepEqual(await listTranscriptionModels(), []);
  assert.equal(await lookupTranscriptionDurationPricing('openrouter', PARAKEET), null);
  assert.equal(requests.length, 0);
});

test('the overrun cap refuses a stale audio price only while it is reached, and listing is never refused', async () => {
  answerCatalogWith(PARAKEET_CATALOG);
  await refreshTranscriptionCatalog();
  try {
    setStaleAdmissionsHaltedForTests(true);
    assert.deepEqual(
      await lookupTranscriptionDurationPricing('openrouter', PARAKEET),
      PARAKEET_PRICING,
      'a current price admits under the cap',
    );

    expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);
    assert.equal(await lookupTranscriptionDurationPricing('openrouter', PARAKEET), null);
    assert.equal((await listTranscriptionModels()).length, 1, 'the catalog still lists the model; only admission is refused');

    setStaleAdmissionsHaltedForTests(false);
    assert.deepEqual(await lookupTranscriptionDurationPricing('openrouter', PARAKEET), {
      ...PARAKEET_PRICING,
      stale: true,
    });
  } finally {
    setStaleAdmissionsHaltedForTests(null);
  }
  assert.equal(requests.length, 1, 'lookups never reach the network');
});

test('a failed refresh keeps the last validated catalog until the hard bound, and never prices past it', async () => {
  answerCatalogWith(PARAKEET_CATALOG);
  await refreshTranscriptionCatalog();

  answerCatalogWith({}, 503);
  await assert.rejects(refreshTranscriptionCatalog());
  assert.deepEqual(
    await lookupTranscriptionDurationPricing('openrouter', PARAKEET),
    PARAKEET_PRICING,
  );

  expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);
  assert.deepEqual(
    await lookupTranscriptionDurationPricing('openrouter', PARAKEET),
    { ...PARAKEET_PRICING, stale: true },
  );
  assert.equal((await listTranscriptionModels()).length, 1);

  expirePricingCacheForTests(SNAPSHOT_MAX_AGE_MS + 1);
  assert.equal(await lookupTranscriptionDurationPricing('openrouter', PARAKEET), null);
  assert.deepEqual(await listTranscriptionModels(), []);
  assert.equal(requests.length, 2, 'only the two refresh attempts may reach the network');
});

import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isCompatibleTranscriptionModel,
  listTranscriptionModels,
  lookupTranscriptionDurationPricing,
  resetTranscriptionModelCacheForTests,
  TRANSCRIPTION_CATALOG_TTL_MS,
} from './transcription-models';

test('lists only OpenRouter audio-to-transcription models without an API key', async () => {
  const originalFetch = globalThis.fetch;
  const originalKey = process.env.OPENROUTER_API_KEY;
  delete process.env.OPENROUTER_API_KEY;
  globalThis.fetch = async (input, init) => {
    assert.match(String(input), /output_modalities=transcription/);
    assert.equal(
      new Headers(init?.headers).get('authorization'),
      null,
    );
    return Response.json({
      data: [
        {
          id: 'nvidia/parakeet',
          name: 'Parakeet',
          architecture: {
            input_modalities: ['audio'],
            output_modalities: ['transcription'],
          },
        },
        {
          id: 'text-only',
          architecture: {
            input_modalities: ['text'],
            output_modalities: ['text'],
          },
        },
      ],
    });
  };
  resetTranscriptionModelCacheForTests();
  try {
    assert.deepEqual(await listTranscriptionModels(), [
      {
        provider: 'openrouter',
        modelId: 'nvidia/parakeet',
        name: 'Parakeet',
      },
    ]);
    assert.equal(
      await isCompatibleTranscriptionModel('openrouter', 'nvidia/parakeet'),
      true,
    );
    assert.equal(
      await isCompatibleTranscriptionModel('openrouter', 'text-only'),
      false,
    );
  } finally {
    globalThis.fetch = originalFetch;
    if (originalKey === undefined) delete process.env.OPENROUTER_API_KEY;
    else process.env.OPENROUTER_API_KEY = originalKey;
    resetTranscriptionModelCacheForTests();
  }
});

test('audio admission resolves verified duration pricing without a text catalog', async () => {
  const originalFetch = globalThis.fetch;
  const originalKey = process.env.OPENROUTER_API_KEY;
  process.env.OPENROUTER_API_KEY = 'test-key';
  const modelId = 'nvidia/parakeet-tdt-0.6b-v3';
  let pricing: { prompt: string; completion: string } = {
    prompt: '0.000025',
    completion: '0',
  };
  let failure = false;
  let removed = false;
  let requests = 0;
  const originalNow = Date.now;
  let now = originalNow();
  Date.now = function mockedNow() {
    return now;
  };
  globalThis.fetch = async (input) => {
    requests += 1;
    assert.match(
      String(input),
      /input_modalities=audio&output_modalities=transcription/,
    );
    if (failure) throw new Error('Catalog unavailable');
    return Response.json({
      data: removed
        ? []
        : [
            {
              id: modelId,
              architecture: {
                input_modalities: ['audio'],
                output_modalities: ['transcription'],
              },
              pricing,
            },
          ],
    });
  };
  resetTranscriptionModelCacheForTests();
  try {
    assert.deepEqual(
      await lookupTranscriptionDurationPricing('openrouter', modelId),
      {
        unit: 'second',
        usdPerSecond: 0.000025,
        source: 'openrouter_transcription_catalog',
      },
    );
    assert.equal(
      await lookupTranscriptionDurationPricing('openai', modelId),
      null,
    );
    assert.equal(requests, 1);
    assert.equal(
      (await lookupTranscriptionDurationPricing('openrouter', modelId))
        ?.usdPerSecond,
      0.000025,
    );
    assert.equal(
      requests,
      1,
      'fresh pricing metadata is shared within its TTL',
    );
    now += TRANSCRIPTION_CATALOG_TTL_MS + 1;
    failure = true;
    await assert.rejects(
      lookupTranscriptionDurationPricing('openrouter', modelId),
      /Catalog unavailable/,
    );
    failure = false;
    removed = true;
    assert.equal(
      await lookupTranscriptionDurationPricing('openrouter', modelId),
      null,
    );
    assert.equal(requests, 3);
    for (const invalidRate of [
      '',
      '-1',
      'NaN',
      'Infinity',
      '1e999',
      ' 0.000025 ',
    ]) {
      resetTranscriptionModelCacheForTests();
      removed = false;
      pricing = { prompt: invalidRate, completion: '0' };
      assert.equal(
        await lookupTranscriptionDurationPricing('openrouter', modelId),
        null,
      );
    }
    resetTranscriptionModelCacheForTests();
    pricing = { prompt: '0.000025', completion: '0.1' };
    assert.equal(
      await lookupTranscriptionDurationPricing('openrouter', modelId),
      null,
    );
    resetTranscriptionModelCacheForTests();
    pricing = { prompt: '0', completion: '0' };
    assert.equal(
      (await lookupTranscriptionDurationPricing('openrouter', modelId))
        ?.usdPerSecond,
      0,
    );
  } finally {
    globalThis.fetch = originalFetch;
    Date.now = originalNow;
    if (originalKey === undefined) delete process.env.OPENROUTER_API_KEY;
    else process.env.OPENROUTER_API_KEY = originalKey;
    resetTranscriptionModelCacheForTests();
  }
});

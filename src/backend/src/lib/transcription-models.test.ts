import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isCompatibleTranscriptionModel,
  listTranscriptionModels,
  resetTranscriptionModelCacheForTests,
} from './transcription-models';

test('lists only OpenRouter audio-to-transcription models', async () => {
  const originalFetch = globalThis.fetch;
  const originalKey = process.env.OPENROUTER_API_KEY;
  process.env.OPENROUTER_API_KEY = 'test-key';
  globalThis.fetch = async (input, init) => {
    assert.match(String(input), /output_modalities=transcription/);
    assert.equal(
      new Headers(init?.headers).get('authorization'),
      'Bearer test-key',
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
      await isCompatibleTranscriptionModel(
        'openrouter',
        'nvidia/parakeet',
      ),
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

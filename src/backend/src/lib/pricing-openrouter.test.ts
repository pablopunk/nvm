import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { parseOpenRouterEndpoints } from './pricing-openrouter';

const MODEL = 'vendor/model';

function listing(endpoints: Array<Record<string, unknown>>, id = MODEL) {
  return {
    data: {
      id,
      name: id,
      endpoints: endpoints.map((pricing) => ({ provider_name: 'P', status: 0, pricing })),
    },
  };
}

test('the captured live endpoint listing prices every rate at its most expensive endpoint', () => {
  const body = JSON.parse(
    readFileSync(
      new URL(
        '../fixtures/contracts/openrouter/endpoints-deepseek-v4-flash.json',
        import.meta.url,
      ),
      'utf8',
    ),
  );

  assert.deepEqual(parseOpenRouterEndpoints('deepseek/deepseek-v4-flash-0731', body), {
    valid: true,
    cost: {
      provider: 'openrouter',
      modelId: 'deepseek/deepseek-v4-flash-0731',
      inputUsdPerMtok: 0.44,
      outputUsdPerMtok: 1.32,
      cacheReadUsdPerMtok: 0.2,
      cacheWriteUsdPerMtok: 0.44,
    },
  });
});

test('a tier override applies from its threshold while the base still covers smaller prompts', () => {
  const parsed = parseOpenRouterEndpoints(
    MODEL,
    listing([
      {
        prompt: '0.000001',
        completion: '0.000001',
        input_cache_read: '0.0000001',
        input_cache_write: '0.000002',
        overrides: [
          { min_prompt_tokens: 272000, prompt: '0.000004', completion: '0.000015' },
        ],
      },
      { prompt: '0.000002', completion: '0.000002' },
    ]),
  );

  assert.deepEqual(parsed, {
    valid: true,
    cost: {
      provider: 'openrouter',
      modelId: MODEL,
      inputUsdPerMtok: 2,
      outputUsdPerMtok: 2,
      cacheReadUsdPerMtok: 2,
      cacheWriteUsdPerMtok: 2,
      tiers: [
        {
          thresholdTokens: 272000,
          inputUsdPerMtok: 4,
          outputUsdPerMtok: 15,
          cacheReadUsdPerMtok: 2,
          cacheWriteUsdPerMtok: 2,
        },
      ],
    },
  });
});

test('time-of-day windows fold into the base at their most expensive rate', () => {
  const parsed = parseOpenRouterEndpoints(
    MODEL,
    listing([
      {
        prompt: '0.000001',
        completion: '0.000001',
        overrides: [
          { utc_start: 0, utc_end: 1400, prompt: '0.000003', completion: '0.000001' },
          { utc_start: 1400, utc_end: 0, prompt: '0.0000005', completion: '0.000001' },
        ],
      },
    ]),
  );

  assert.equal(parsed.valid && parsed.cost?.inputUsdPerMtok, 3);
  assert.equal(parsed.valid && parsed.cost && 'tiers' in parsed.cost, false);
});

test('day-of-week overrides fold into the base like time windows, as the live listing shapes them', () => {
  // Shape captured from GET /api/v1/models/deepseek/deepseek-v4-pro-0813/endpoints.
  const parsed = parseOpenRouterEndpoints(
    MODEL,
    listing([
      {
        prompt: '0.00000066',
        completion: '0.00000198',
        input_cache_read: '0.000000022',
        overrides: [
          {
            utc_days: ['saturday', 'sunday'],
            prompt: '0.00000066',
            completion: '0.00000198',
            input_cache_read: '0.000000022',
          },
          {
            utc_start: 100,
            utc_end: 400,
            utc_days: ['monday', 'tuesday', 'wednesday', 'thursday', 'friday'],
            prompt: '0.00000132',
            completion: '0.00000396',
            input_cache_read: '0.000000044',
          },
        ],
      },
    ]),
  );

  assert.equal(parsed.valid && parsed.cost?.inputUsdPerMtok, 1.32);
  assert.equal(parsed.valid && parsed.cost?.outputUsdPerMtok, 3.96);
  assert.equal(parsed.valid && parsed.cost?.cacheReadUsdPerMtok, 0.044);
});

test('image input raises the input ceiling and internal reasoning raises the output ceiling', () => {
  const parsed = parseOpenRouterEndpoints(
    MODEL,
    listing([
      {
        prompt: '0.000001',
        completion: '0.000002',
        image: '0.000003',
        internal_reasoning: '0.000004',
      },
    ]),
  );

  assert.equal(parsed.valid && parsed.cost?.inputUsdPerMtok, 3);
  assert.equal(parsed.valid && parsed.cost?.outputUsdPerMtok, 4);
});

test('a zero per-request fee is priced, a non-zero one is refused', () => {
  const free = parseOpenRouterEndpoints(
    MODEL,
    listing([{ prompt: '0.000001', completion: '0.000001', request: '0' }]),
  );
  const charged = parseOpenRouterEndpoints(
    MODEL,
    listing([{ prompt: '0.000001', completion: '0.000001', request: '0.01' }]),
  );

  assert.equal(free.valid && free.cost?.inputUsdPerMtok, 1);
  assert.deepEqual(charged, { valid: true, cost: null });
});

test('a dynamic router price of -1 is unpriceable rather than a negative rate', () => {
  assert.deepEqual(
    parseOpenRouterEndpoints(MODEL, listing([{ prompt: '-1', completion: '-1' }])),
    { valid: true, cost: null },
  );
});

test('one malformed endpoint makes the whole model unpriced, never partially priced', () => {
  const parsed = parseOpenRouterEndpoints(
    MODEL,
    listing([
      { prompt: '0.000001', completion: '0.000001' },
      { prompt: 'cheap', completion: '0.000001' },
    ]),
  );

  assert.deepEqual(parsed, { valid: true, cost: null });
});

test('an override with neither a threshold nor a time window is refused', () => {
  const parsed = parseOpenRouterEndpoints(
    MODEL,
    listing([
      {
        prompt: '0.000001',
        completion: '0.000001',
        overrides: [{ prompt: '0.000009' }],
      },
    ]),
  );

  assert.deepEqual(parsed, { valid: true, cost: null });
});

test('a listing with no endpoints is a valid answer that prices nothing', () => {
  assert.deepEqual(parseOpenRouterEndpoints(MODEL, listing([])), {
    valid: true,
    cost: null,
  });
});

test('a body that is not the requested model’s endpoint listing is rejected outright', () => {
  assert.deepEqual(
    parseOpenRouterEndpoints(MODEL, listing([{ prompt: '0.000001', completion: '0.000001' }], 'other/model')),
    { valid: false },
  );
  assert.deepEqual(parseOpenRouterEndpoints(MODEL, { data: { id: MODEL } }), {
    valid: false,
  });
  assert.deepEqual(parseOpenRouterEndpoints(MODEL, null), { valid: false });
});

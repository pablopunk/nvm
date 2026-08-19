import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { computeUsdCost, usdToCredits, usdToMicrocents, CREDIT_USD, MARKUP } from './cost';
import type { ModelCost } from './pricing';

const makeCost = (input: number, output: number): ModelCost => ({
  provider: 'test',
  modelId: 'test-model',
  inputUsdPerMtok: input,
  outputUsdPerMtok: output,
});

describe('computeUsdCost', () => {
  test('zero tokens yields zero cost', () => {
    assert.strictEqual(computeUsdCost(makeCost(10, 30), 0, 0), 0);
  });

  test('input-only tokens', () => {
    const result = computeUsdCost(makeCost(10, 0), 500_000, 0);
    assert.strictEqual(result, 5);
  });

  test('output-only tokens', () => {
    const result = computeUsdCost(makeCost(0, 30), 0, 100_000);
    assert.strictEqual(result, 3);
  });

  test('mixed input and output tokens', () => {
    const result = computeUsdCost(makeCost(10, 30), 500_000, 100_000);
    assert.strictEqual(result, 8);
  });

  test('sub-million token counts', () => {
    const result = computeUsdCost(makeCost(1, 5), 100, 200);
    assert.strictEqual(result, 0.0011);
  });
});

describe('usdToCredits', () => {
  test('applies markup and ceil division by credit_usd', () => {
    assert.strictEqual(usdToCredits(0.01), 5);
  });

  test('floors at 1 credit minimum', () => {
    assert.strictEqual(usdToCredits(0.0001), 1);
    assert.strictEqual(usdToCredits(0), 1);
    assert.strictEqual(usdToCredits(-0.01), 1);
  });

  test('ceils fractional credits', () => {
    assert.strictEqual(usdToCredits(0.0021), 2);
  });

  test('rounding at the ceil boundary', () => {
    assert.strictEqual(usdToCredits(0.002), 1);
    assert.strictEqual(usdToCredits(0.00201), 2);
  });
});

describe('usdToMicrocents', () => {
  test('converts 1 cent to 1M microcents', () => {
    assert.strictEqual(usdToMicrocents(0.01), 1_000_000);
  });

  test('converts 1 dollar to 100M microcents', () => {
    assert.strictEqual(usdToMicrocents(1), 100_000_000);
  });

  test('zero cost → zero microcents', () => {
    assert.strictEqual(usdToMicrocents(0), 0);
  });

  test('rounds to nearest microcent', () => {
    assert.strictEqual(usdToMicrocents(0.000000001), 0);
    assert.strictEqual(usdToMicrocents(0.000000005), 1);
  });
});

describe('CREDIT_USD and MARKUP defaults', () => {
  test('CREDIT_USD defaults to 0.01', () => {
    assert.strictEqual(CREDIT_USD, 0.01);
  });

  test('MARKUP defaults to 5', () => {
    assert.strictEqual(MARKUP, 5);
  });
});

import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  declaredCatalogKeys,
  pricingContractForProvider,
  trustedReportedCostUsd,
} from './provider-pricing-contract';

test('only providers that declare usage-cost semantics may settle from a reported cost', () => {
  assert.equal(trustedReportedCostUsd('openrouter', 0.0123), 0.0123);
  assert.equal(trustedReportedCostUsd('openrouter', 0), 0);
  assert.equal(trustedReportedCostUsd('anthropic', 0.0123), undefined);
  assert.equal(trustedReportedCostUsd('opencode_zen', 0.0123), undefined);
  assert.equal(trustedReportedCostUsd('undeclared-provider', 0.0123), undefined);
});

test('reported costs must be finite and non-negative to be trusted', () => {
  assert.equal(trustedReportedCostUsd('openrouter', undefined), undefined);
  assert.equal(trustedReportedCostUsd('openrouter', null), undefined);
  assert.equal(trustedReportedCostUsd('openrouter', -1), undefined);
  assert.equal(trustedReportedCostUsd('openrouter', Number.NaN), undefined);
  assert.equal(trustedReportedCostUsd('openrouter', Number.POSITIVE_INFINITY), undefined);
});

test('undeclared providers have no price discovery or capability source', () => {
  assert.deepEqual(pricingContractForProvider('undeclared-provider'), {
    priceDiscovery: 'none',
    capabilitySource: 'none',
    reportedCost: 'none',
    catalogKey: '',
  });
  assert.equal(pricingContractForProvider('opencode_zen').catalogKey, 'opencode');
  assert.equal(pricingContractForProvider('openrouter').capabilitySource, 'models_dev');
});

test('every declared models.dev section is read once, and OpenRouter prices from endpoints', () => {
  assert.deepEqual(
    [...declaredCatalogKeys()].sort(),
    ['anthropic', 'google', 'openai', 'opencode', 'openrouter'],
  );
  assert.equal(pricingContractForProvider('openrouter').priceDiscovery, 'openrouter_endpoints');
  assert.equal(pricingContractForProvider('anthropic').priceDiscovery, 'models_dev');
});

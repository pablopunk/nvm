import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  DEFAULT_NEVERMIND_MAX_INPUT_TOKENS,
  effectiveNevermindContextWindow,
} from './ai-context-window';

const LARGE_PROVIDER_CONTEXT_WINDOW = 1_000_000;
const SMALL_PROVIDER_CONTEXT_WINDOW = 64_000;

test('caps a larger provider context at the backend input limit', () => {
  assert.equal(
    effectiveNevermindContextWindow(
      LARGE_PROVIDER_CONTEXT_WINDOW,
      DEFAULT_NEVERMIND_MAX_INPUT_TOKENS,
    ),
    DEFAULT_NEVERMIND_MAX_INPUT_TOKENS,
  );
});

test('keeps a provider context below the backend input limit', () => {
  assert.equal(
    effectiveNevermindContextWindow(
      SMALL_PROVIDER_CONTEXT_WINDOW,
      DEFAULT_NEVERMIND_MAX_INPUT_TOKENS,
    ),
    SMALL_PROVIDER_CONTEXT_WINDOW,
  );
});

test('uses the default backend input limit for older or invalid descriptors', () => {
  assert.equal(
    effectiveNevermindContextWindow(LARGE_PROVIDER_CONTEXT_WINDOW),
    DEFAULT_NEVERMIND_MAX_INPUT_TOKENS,
  );
  assert.equal(
    effectiveNevermindContextWindow(LARGE_PROVIDER_CONTEXT_WINDOW, 0),
    DEFAULT_NEVERMIND_MAX_INPUT_TOKENS,
  );
});

import assert from 'node:assert/strict';
import test from 'node:test';
import { BUILDER_REQUEST_GUIDANCE } from './ai-builder-request-guidance';

test('builds from a detailed first message instead of requiring confirmation', () => {
  assert.match(BUILDER_REQUEST_GUIDANCE, /first message is their request/);
  assert.match(BUILDER_REQUEST_GUIDANCE, /never ask a generic/);
  assert.doesNotMatch(BUILDER_REQUEST_GUIDANCE, /until the user has confirmed/);
});

test('routes live-while-typing requests to root searchItems', () => {
  assert.match(BUILDER_REQUEST_GUIDANCE, /no per-keystroke callback/);
  assert.match(BUILDER_REQUEST_GUIDANCE, /searchItems\(ctx, query\)/);
});

test('requires the completion message to name every key press', () => {
  assert.match(BUILDER_REQUEST_GUIDANCE, /every key press/);
});

test('names the action menu shortcut for every platform and keeps it out of extension copy', () => {
  assert.match(
    BUILDER_REQUEST_GUIDANCE,
    /Command\+K on macOS and Control\+K on Windows and Linux/,
  );
  assert.match(
    BUILDER_REQUEST_GUIDANCE,
    /never write a keyboard shortcut into extension/,
  );
});

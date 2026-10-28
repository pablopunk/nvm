import assert from 'node:assert/strict';
import { test } from 'node:test';
import { resolveRendererUrl } from './renderer-url';

test('packaged builds ignore an injected renderer URL', () => {
  assert.equal(
    resolveRendererUrl({ ELECTRON_RENDERER_URL: 'https://evil.test/' }, true),
    undefined,
  );
});

test('unpackaged builds use the trimmed renderer URL', () => {
  assert.equal(
    resolveRendererUrl(
      { ELECTRON_RENDERER_URL: ' http://localhost:5173/ ' },
      false,
    ),
    'http://localhost:5173/',
  );
});

test('unpackaged builds without a renderer URL resolve to undefined', () => {
  assert.equal(resolveRendererUrl({}, false), undefined);
  assert.equal(
    resolveRendererUrl({ ELECTRON_RENDERER_URL: '   ' }, false),
    undefined,
  );
});

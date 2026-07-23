import assert from 'node:assert/strict';
import test from 'node:test';
import { createDesignTokenStudioServer } from './design-token-studio-server';

const origin = 'http://127.0.0.1:5173';

test('design token studio server requires its origin and token', async () => {
  let overrides = {};
  const server = await createDesignTokenStudioServer({
    allowedOrigin: origin,
    getState: () => ({ enabled: true, defaults: {}, overrides, values: {} }),
    setState: (next) => {
      overrides = next;
      return { enabled: true, defaults: {}, overrides, values: {} };
    },
    resetState: () => {
      overrides = {};
      return { enabled: true, defaults: {}, overrides, values: {} };
    },
  });
  try {
    const forbidden = await fetch(server.apiUrl);
    assert.equal(forbidden.status, 403);
    const saved = await fetch(server.apiUrl, {
      method: 'PUT',
      headers: {
        origin,
        'content-type': 'application/json',
        'x-nvm-token': server.token,
      },
      body: JSON.stringify({ '--radius-lg': '24px' }),
    });
    assert.equal(saved.status, 200);
    assert.deepEqual((await saved.json()).overrides, { '--radius-lg': '24px' });
    const reset = await fetch(server.apiUrl, {
      method: 'DELETE',
      headers: { origin, 'x-nvm-token': server.token },
    });
    assert.equal(reset.status, 200);
    assert.deepEqual((await reset.json()).overrides, {});
  } finally {
    await server.close();
  }
});

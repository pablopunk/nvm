import assert from 'node:assert/strict';
import test from 'node:test';
import {
  DESIGN_TOKEN_DEFAULTS,
  type DesignTokenOverrides,
  resolveDesignTokens,
} from '../design-tokens';
import { createDesignTokenStudioServer } from './design-token-studio-server';

const origin = 'http://127.0.0.1:5173';

test('design token studio server requires its origin and token', async () => {
  let overrides: DesignTokenOverrides = {};
  const state = () => ({
    enabled: true,
    defaults: { ...DESIGN_TOKEN_DEFAULTS },
    overrides,
    values: resolveDesignTokens(overrides),
  });
  const server = await createDesignTokenStudioServer({
    allowedOrigin: origin,
    getState: state,
    setState: (next) => {
      overrides = next;
      return state();
    },
    resetState: () => {
      overrides = {};
      return state();
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

test('design token studio server routes authenticated RPC calls', async () => {
  const calls: Array<{ method: string; params: unknown }> = [];
  const server = await createDesignTokenStudioServer({
    allowedOrigin: origin,
    getState: () => ({
      enabled: true,
      defaults: { ...DESIGN_TOKEN_DEFAULTS },
      overrides: {},
      values: resolveDesignTokens({}),
    }),
    setState: () => {
      throw new Error('unused');
    },
    resetState: () => {
      throw new Error('unused');
    },
    rpc: (method, params) => {
      calls.push({ method, params });
      return { ok: true };
    },
  });
  try {
    const response = await fetch(server.rpcUrl, {
      method: 'POST',
      headers: {
        authorization: `Bearer ${server.token}`,
        origin,
        'content-type': 'application/json',
      },
      body: JSON.stringify({ method: 'search', params: { query: 'app' } }),
    });
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), { ok: true });
    assert.deepEqual(calls, [{ method: 'search', params: { query: 'app' } }]);
  } finally {
    await server.close();
  }
});

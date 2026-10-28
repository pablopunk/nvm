import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { getVersion: () => '0.24.1-nightly.20261028.1' },
  },
});

const { nevermindDesktopHeaders } = await import('./nevermind-api');

test('sends the installed nightly version to the backend without changing it', () => {
  const headers = nevermindDesktopHeaders({ authorization: 'Bearer test' });
  assert.equal(
    headers['X-Nevermind-Client-Version'],
    '0.24.1-nightly.20261028.1',
  );
  assert.equal(headers['X-Nevermind-API-Version'], '1');
  assert.equal(new Headers(headers).get('authorization'), 'Bearer test');
});

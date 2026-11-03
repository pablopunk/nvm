import assert from 'node:assert/strict';
import http from 'node:http';
import type { AddressInfo } from 'node:net';
import test from 'node:test';
import { fetchPublicHttp, isPublicAddress } from './public-http-fetch';

test('isPublicAddress rejects loopback, private, link-local, and mapped addresses', () => {
  for (const address of [
    '127.0.0.1',
    '10.1.2.3',
    '172.20.0.1',
    '192.168.0.9',
    '169.254.169.254',
    '100.64.0.1',
    '0.0.0.0',
    '::1',
    'fe80::1',
    'fd00::1',
    '::ffff:127.0.0.1',
    '64:ff9b::7f00:1',
    'not-an-ip',
  ])
    assert.equal(isPublicAddress(address), false, address);
  for (const address of ['8.8.8.8', '93.184.216.34', '2606:4700::1111'])
    assert.equal(isPublicAddress(address), true, address);
});

function lookupTable(table: Record<string, string>) {
  return ((hostname: string, _options: unknown, callback: any) => {
    const address = table[hostname];
    if (!address) return callback(new Error('ENOTFOUND'));
    callback(null, [{ address, family: 4 }]);
  }) as any;
}

async function withServer(
  handler: http.RequestListener,
  run: (port: number) => Promise<void>,
) {
  const server = http.createServer(handler);
  await new Promise<void>((resolve) =>
    server.listen(0, '127.0.0.1', () => resolve()),
  );
  try {
    await run((server.address() as AddressInfo).port);
  } finally {
    server.close();
  }
}

const fetchOptions = {
  accept: 'text/html',
  maxBytes: 1024,
  timeoutMs: 3000,
};

test('refuses loopback literals and hosts resolving to private addresses', async () => {
  await assert.rejects(
    fetchPublicHttp('http://127.0.0.1:9/', fetchOptions),
    /Non-public/,
  );
  await assert.rejects(
    fetchPublicHttp('http://[::1]:9/', fetchOptions),
    /Non-public/,
  );
  await assert.rejects(
    fetchPublicHttp('http://rebind.test/', {
      ...fetchOptions,
      lookup: lookupTable({ 'rebind.test': '10.0.0.5' }),
    }),
    /public address/,
  );
});

test('re-validates every redirect hop', async () => {
  await withServer(
    (_request, response) => {
      response.writeHead(302, { location: 'http://blocked.test/secret' });
      response.end();
    },
    async (port) => {
      await assert.rejects(
        fetchPublicHttp(`http://ok.test:${port}/`, {
          ...fetchOptions,
          lookup: lookupTable({
            'ok.test': '127.0.0.1',
            'blocked.test': '10.0.0.5',
          }),
          addressPolicy: (address) => address === '127.0.0.1',
        }),
        /public address/,
      );
    },
  );
});

test('follows an allowed redirect and bounds the body', async () => {
  await withServer(
    (request, response) => {
      if (request.url === '/') {
        response.writeHead(302, { location: '/final' });
        return response.end();
      }
      response.writeHead(200, { 'content-type': 'text/html' });
      response.end('x'.repeat(4096));
    },
    async (port) => {
      const options = {
        ...fetchOptions,
        lookup: lookupTable({ 'ok.test': '127.0.0.1' }),
        addressPolicy: () => true,
      };
      await assert.rejects(
        fetchPublicHttp(`http://ok.test:${port}/`, options),
        /too large/,
      );
      const response = await fetchPublicHttp(`http://ok.test:${port}/`, {
        ...options,
        maxBytes: 1024,
        isComplete: () => true,
      });
      assert.equal(response.finalUrl, `http://ok.test:${port}/final`);
      assert.ok(response.body.byteLength > 0);
    },
  );
});

// biome-ignore-all lint: Table-driven regression cases keep the address matrix readable.
import assert from 'node:assert/strict';
import test from 'node:test';
import { isUnsafeNevermindHostname } from './nevermind-url';

test('rejects loopback and private IPv4 variants', () => {
  for (const hostname of [
    '127.0.0.1',
    '127.0.0.2',
    '0.0.0.0',
    '10.0.0.4',
    '172.16.0.8',
    '192.168.1.20',
  ])
    assert.equal(isUnsafeNevermindHostname(hostname), true, hostname);
});

test('rejects normalized localhost and IPv4-mapped IPv6 variants', () => {
  for (const hostname of [
    'localhost.',
    'DEV.LOCALHOST',
    '::1',
    '::ffff:127.0.0.1',
    '::ffff:7f00:2',
    'fc00::1',
    'fe80::1',
  ])
    assert.equal(isUnsafeNevermindHostname(hostname), true, hostname);
});

test('allows public addresses', () => {
  for (const hostname of ['api.nvm.fyi', '8.8.8.8', '2001:4860:4860::8888'])
    assert.equal(isUnsafeNevermindHostname(hostname), false, hostname);
});

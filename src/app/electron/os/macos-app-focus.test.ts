import assert from 'node:assert/strict';
import test from 'node:test';
import { parseLsappinfoTarget } from './macos-app-focus';

test('parses the legacy lsappinfo format', () => {
  assert.deepEqual(
    parseLsappinfoTarget('"CFBundleIdentifier"="com.apple.finder"\n"pid"=412'),
    { bundleId: 'com.apple.finder', pid: 412 },
  );
});

test('parses the macOS 27 lsappinfo format', () => {
  const output = `[ NULL ]  ASN:0x0-0x8ee8ee: (in front)
    bundleID="com.t3tools.t3code"
    bundle path=[ NULL ]
    pid = 57371 !cgsConnection !signalled type=[ NULL ]`;
  assert.deepEqual(parseLsappinfoTarget(output), {
    bundleId: 'com.t3tools.t3code',
    pid: 57_371,
  });
});

test('returns null when bundle id or pid is missing', () => {
  assert.equal(parseLsappinfoTarget('pid = 12'), null);
  assert.equal(parseLsappinfoTarget('bundleID="com.apple.finder"'), null);
  assert.equal(parseLsappinfoTarget(''), null);
});

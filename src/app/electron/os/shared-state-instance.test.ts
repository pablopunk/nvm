import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { acquireSharedStateInstance } from './shared-state-instance';

test('shared identity is used for the singleton and the original profile is restored', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-instance-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  let userData = path.join(directory, 'private');
  let name = 'Another build';
  const observed: Array<{ userData: string; name: string }> = [];
  const app = {
    getPath: () => userData,
    setPath: (_key: 'userData', value: string) => {
      userData = value;
    },
    getName: () => name,
    setName: (value: string) => {
      name = value;
    },
    hasSingleInstanceLock: () => false,
    requestSingleInstanceLock: () => {
      observed.push({ userData, name });
      return true;
    },
    releaseSingleInstanceLock: () => {},
  };
  assert.equal(
    await acquireSharedStateInstance(app, path.join(directory, 'shared')),
    true,
  );
  assert.equal(
    observed[0]?.userData,
    path.join(await fs.realpath(directory), '.shared-instance'),
  );
  assert.equal(observed[0]?.name, 'Nevermind shared state');
  assert.equal(userData, path.join(directory, 'private'));
  assert.equal(name, 'Another build');
});

test('a refused lock or singleton error leaves the original profile intact', async (t) => {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-instance-error-'),
  );
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  let userData = 'original';
  let name = 'original-name';
  let fail = false;
  const app = {
    getPath: () => userData,
    setPath: (_key: 'userData', value: string) => {
      userData = value;
    },
    getName: () => name,
    setName: (value: string) => {
      name = value;
    },
    hasSingleInstanceLock: () => false,
    requestSingleInstanceLock: () => {
      if (fail) throw new Error('unavailable');
      return false;
    },
    releaseSingleInstanceLock: () => {},
  };
  assert.equal(
    await acquireSharedStateInstance(app, path.join(directory, 'shared')),
    false,
  );
  fail = true;
  await assert.rejects(
    acquireSharedStateInstance(app, path.join(directory, 'shared')),
    /unavailable/,
  );
  assert.equal(userData, 'original');
  assert.equal(name, 'original-name');
});

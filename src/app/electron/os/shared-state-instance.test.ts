import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {
  acquireDefaultSharedStateInstance,
  acquireSharedStateInstance,
} from './shared-state-instance';

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
    await acquireSharedStateInstance(
      app,
      path.join(directory, 'shared'),
      directory,
    ),
    true,
  );
  assert.equal(
    observed[0]?.userData,
    path.join(
      directory,
      createHash('sha256')
        .update(path.join(await fs.realpath(directory), 'shared'))
        .digest('hex'),
    ),
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
    await acquireSharedStateInstance(
      app,
      path.join(directory, 'shared'),
      directory,
    ),
    false,
  );
  fail = true;
  await assert.rejects(
    acquireSharedStateInstance(app, path.join(directory, 'shared'), directory),
    /unavailable/,
  );
  assert.equal(userData, 'original');
  assert.equal(name, 'original-name');
});

test('symlinked paths to the same state use the same singleton identity', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-alias-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const root = path.join(directory, 'state');
  const alias = path.join(directory, 'alias');
  await fs.mkdir(root);
  await fs.symlink(root, alias, 'junction');
  let userData = 'original';
  const identities: string[] = [];
  const app = {
    getPath: () => userData,
    setPath: (_key: 'userData', value: string) => {
      userData = value;
    },
    getName: () => 'original',
    setName: (_value: string) => {},
    hasSingleInstanceLock: () => false,
    requestSingleInstanceLock: () => {
      identities.push(userData);
      return true;
    },
    releaseSingleInstanceLock: () => {},
  };
  const identityRoot = path.join(directory, 'locks');
  await acquireSharedStateInstance(app, root, identityRoot);
  await acquireSharedStateInstance(app, alias, identityRoot);
  assert.equal(identities[0], identities[1]);
  assert.equal(path.dirname(identities[0] ?? ''), identityRoot);
});

test('test mode keeps the singleton identity within its isolated state root', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-test-lock-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  let userData = 'original';
  let identity = '';
  const app = {
    getPath: () => userData,
    setPath: (_key: 'userData', value: string) => {
      userData = value;
    },
    getName: () => 'original',
    setName: (_value: string) => {},
    hasSingleInstanceLock: () => false,
    requestSingleInstanceLock: () => {
      identity = userData;
      return true;
    },
    releaseSingleInstanceLock: () => {},
  };
  await acquireDefaultSharedStateInstance(
    app,
    true,
    path.join(directory, 'state'),
  );
  assert.equal(
    path.dirname(identity),
    path.join(await fs.realpath(directory), 'state', '.instance-locks'),
  );
});

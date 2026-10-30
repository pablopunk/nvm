import assert from 'node:assert/strict';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { defaultFileIndexRoots } from './file-index-roots';

test('default index roots cover the standard personal folders', () => {
  assert.deepEqual(
    defaultFileIndexRoots(),
    ['Desktop', 'Documents', 'Downloads', 'Pictures', 'Videos', 'Movies'].map(
      (name) => path.join(os.homedir(), name),
    ),
  );
});

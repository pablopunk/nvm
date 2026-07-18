import assert from 'node:assert/strict';
import test from 'node:test';
import { writePrivateFile } from './private-file';

const PRIVATE_FILE_MODE = 0o600;

function recordingFileSystem() {
  const calls: Array<{ method: string; args: unknown[] }> = [];
  return {
    calls,
    fileSystem: {
      writeFile: (...args: unknown[]) => {
        calls.push({ method: 'writeFile', args });
        return Promise.resolve();
      },
      chmod: (...args: unknown[]) => {
        calls.push({ method: 'chmod', args });
        return Promise.resolve();
      },
    },
  };
}

test('Windows private files use creation metadata without pretending chmod configures ACLs', async () => {
  const { calls, fileSystem } = recordingFileSystem();
  const filePath = String.raw`\\server\profile\Nevermind data\clave-密钥.json`;

  await writePrivateFile(filePath, '{"token":"redacted"}', {
    fileSystem: fileSystem as never,
    processPlatform: 'win32',
  });

  assert.deepEqual(calls, [
    {
      method: 'writeFile',
      args: [filePath, '{"token":"redacted"}', { mode: 0o600 }],
    },
  ]);
});

test('POSIX private files enforce mode after writing', async () => {
  const { calls, fileSystem } = recordingFileSystem();
  await writePrivateFile('/tmp/never mind/auth.json', '{}', {
    fileSystem: fileSystem as never,
    processPlatform: 'darwin',
  });

  assert.deepEqual(
    calls.map((call) => call.method),
    ['writeFile', 'chmod'],
  );
  assert.deepEqual(calls[1]?.args, [
    '/tmp/never mind/auth.json',
    PRIVATE_FILE_MODE,
  ]);
});

import assert from 'node:assert/strict';
import test from 'node:test';
import { createSystemAudioMuteCapability } from './system-audio';

test('temporarily mutes macOS output and restores the prior unmuted state once', async () => {
  const scripts: string[] = [];
  const capability = createSystemAudioMuteCapability({
    platform: 'darwin',
    runAppleScript: async (script) => {
      scripts.push(script);
      return {
        stdout: scripts.length === 1 ? 'false\n' : '',
        stderr: '',
        exitCode: 0,
      };
    },
  });

  const lease = await capability.temporarilyMute();
  await lease.restore();
  await lease.restore();

  assert.match(scripts[0], /set volume with output muted/);
  assert.equal(scripts[1], 'set volume with output muted false');
  assert.equal(scripts.length, 2);
});

test('restores an output that was already muted to muted', async () => {
  const scripts: string[] = [];
  const capability = createSystemAudioMuteCapability({
    platform: 'darwin',
    runAppleScript: async (script) => {
      scripts.push(script);
      return {
        stdout: scripts.length === 1 ? 'true' : '',
        stderr: '',
        exitCode: 0,
      };
    },
  });

  const lease = await capability.temporarilyMute();
  await lease.restore();

  assert.equal(scripts[1], 'set volume with output muted true');
});

test('allows a failed restoration to be retried', async () => {
  let calls = 0;
  const capability = createSystemAudioMuteCapability({
    platform: 'darwin',
    runAppleScript: async () => {
      calls += 1;
      return calls === 2
        ? { stdout: '', stderr: 'temporary failure', exitCode: 1 }
        : { stdout: 'false', stderr: '', exitCode: 0 };
    },
  });

  const lease = await capability.temporarilyMute();
  await assert.rejects(lease.restore(), /temporary failure/);
  await lease.restore();

  assert.equal(calls, 3);
});

test('does nothing on unsupported systems', async () => {
  let calls = 0;
  const capability = createSystemAudioMuteCapability({
    platform: 'linux',
    runAppleScript: async () => {
      calls += 1;
      return { stdout: '', stderr: '', exitCode: 0 };
    },
  });

  const lease = await capability.temporarilyMute();
  await lease.restore();

  assert.equal(calls, 0);
});

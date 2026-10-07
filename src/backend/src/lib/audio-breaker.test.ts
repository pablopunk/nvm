import assert from 'node:assert/strict';
import { test } from 'node:test';
import { handleTrippedAudioRoute } from './audio-breaker';

test('tripped audio route releases a prepared reservation and fails dedup', async () => {
  const calls: string[] = [];
  await handleTrippedAudioRoute({
    prepared: true,
    releasePreparedReservation: async () => {
      calls.push('release');
    },
    failDedup: async () => {
      calls.push('dedup-failed');
    },
  });
  assert.deepEqual(calls, ['release', 'dedup-failed']);
});

test('tripped unprepared audio route fails dedup without releasing a reservation', async () => {
  const calls: string[] = [];
  await handleTrippedAudioRoute({
    prepared: false,
    releasePreparedReservation: async () => {
      calls.push('release');
    },
    failDedup: async () => {
      calls.push('dedup-failed');
    },
  });
  assert.deepEqual(calls, ['dedup-failed']);
});

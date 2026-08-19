import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createDictationService,
  type DictationRendererCommand,
} from './dictation-service';

test('resolves start only after the renderer confirms audio capture', async () => {
  const commands: DictationRendererCommand[] = [];
  const service = createDictationService((command) => commands.push(command));

  let started = false;
  const start = service
    .start({ deviceId: 'default', modelKeepAliveMs: 300_000 })
    .then(() => {
      started = true;
    });
  assert.equal(await service.status(), 'recording');
  assert.deepEqual(commands, [
    { type: 'start', deviceId: 'default', modelKeepAliveMs: 300_000 },
  ]);
  await Promise.resolve();
  assert.equal(started, false);
  service.reply({ type: 'recording' });
  await start;
  assert.equal(started, true);

  const stopped = service.stop();
  assert.equal(await service.status(), 'transcribing');
  service.reply({ type: 'result', text: 'hello world' });
  assert.equal(await stopped, 'hello world');
  assert.equal(await service.status(), 'idle');
});

test('restores system audio before transcription starts', async () => {
  const events: string[] = [];
  const service = createDictationService(
    (command) => events.push(`command:${command.type}`),
    {
      muteSystemAudio: async () => {
        events.push('mute');
        return {
          restore: async () => {
            events.push('restore');
          },
        };
      },
    },
  );

  const start = service.start({ muteSystemAudioWhileRecording: true });
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(events, ['mute', 'command:start']);
  service.reply({ type: 'recording' });
  await start;

  const stop = service.stop();
  assert.equal(await service.status(), 'transcribing');
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(events, [
    'mute',
    'command:start',
    'restore',
    'command:stop',
  ]);
  service.reply({ type: 'result', text: 'hello' });
  assert.equal(await stop, 'hello');
});

test('restores system audio when recording fails', async () => {
  let restores = 0;
  const service = createDictationService(() => {}, {
    muteSystemAudio: async () => ({
      restore: async () => {
        restores += 1;
      },
    }),
  });

  const start = service.start({ muteSystemAudioWhileRecording: true });
  await new Promise((resolve) => setImmediate(resolve));
  service.reply({ type: 'error', message: 'Microphone failed' });

  await assert.rejects(start, /Microphone failed/);
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(restores, 1);
});

test('returns renderer microphone devices', async () => {
  const commands: DictationRendererCommand[] = [];
  const service = createDictationService((command) => commands.push(command));

  const devices = service.devices();
  assert.deepEqual(commands, [{ type: 'devices' }]);
  service.reply({
    type: 'devices',
    devices: [{ id: 'default', title: 'System Default', isDefault: true }],
  });
  assert.deepEqual(await devices, [
    { id: 'default', title: 'System Default', isDefault: true },
  ]);
});

test('rejects start when renderer microphone startup fails', async () => {
  const service = createDictationService(() => {});

  const start = service.start();
  service.reply({ type: 'error', message: 'Microphone did not become ready' });

  await assert.rejects(start, /Microphone did not become ready/);
  assert.equal(await service.status(), 'idle');
});

test('checks the model cache without loading the model and prepares it on demand', async () => {
  const commands: DictationRendererCommand[] = [];
  const service = createDictationService((command) => commands.push(command));

  const cacheStatus = service.modelCacheStatus();
  assert.deepEqual(commands, [{ type: 'model-cache-status' }]);
  service.reply({ type: 'model-cache-status', cached: false });
  assert.equal(await cacheStatus, 'missing');

  const preparation = service.prepareModel({ modelKeepAliveMs: 300_000 });
  assert.deepEqual(commands, [
    { type: 'model-cache-status' },
    { type: 'prepare-model', modelKeepAliveMs: 300_000 },
  ]);
  service.reply({ type: 'model-ready' });
  await preparation;
});

test('cancels a pending transcription', async () => {
  const service = createDictationService(() => {});
  const start = service.start();
  service.reply({ type: 'recording' });
  await start;
  const stopped = service.stop();
  await service.cancel();
  await assert.rejects(stopped, /Dictation cancelled/);
  assert.equal(await service.status(), 'idle');
});

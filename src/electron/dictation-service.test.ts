import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createDictationService,
  type DictationRendererCommand,
} from './dictation-service';

test('starts immediately and resolves transcription when the renderer replies', async () => {
  const commands: DictationRendererCommand[] = [];
  const service = createDictationService((command) => commands.push(command));

  await service.start({ deviceId: 'default', modelKeepAliveMs: 300_000 });
  assert.equal(await service.status(), 'recording');
  assert.deepEqual(commands, [
    { type: 'start', deviceId: 'default', modelKeepAliveMs: 300_000 },
  ]);

  const stopped = service.stop();
  assert.equal(await service.status(), 'transcribing');
  service.reply({ type: 'result', text: 'hello world' });
  assert.equal(await stopped, 'hello world');
  assert.equal(await service.status(), 'idle');
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
  await service.start();
  const stopped = service.stop();
  await service.cancel();
  await assert.rejects(stopped, /Dictation cancelled/);
  assert.equal(await service.status(), 'idle');
});

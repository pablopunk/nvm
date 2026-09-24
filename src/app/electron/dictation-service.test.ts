import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createDictationService,
  type DictationRendererCommand,
} from './dictation-service';

const MICROPHONE_FAILED_PATTERN = /Microphone failed/;
const MICROPHONE_NOT_READY_PATTERN = /Microphone did not become ready/;
const DICTATION_CANCELLED_PATTERN = /Dictation cancelled/;
const CLOUD_FAILED_PATTERN = /Cloud transcription failed/;
const WEBM_AUDIO = new TextEncoder().encode('webm');

function ignoreCommand(command: DictationRendererCommand) {
  return command.type;
}

test('resolves start only after the renderer confirms audio capture', async () => {
  const commands: DictationRendererCommand[] = [];
  const service = createDictationService((command) => commands.push(command), {
    transcribeAudio: () => Promise.resolve('hello world'),
  });

  let started = false;
  const start = service.start({ deviceId: 'default' }).then(() => {
    started = true;
  });
  assert.equal(await service.status(), 'recording');
  assert.equal(commands[0]?.type, 'start');
  const operationId = (
    commands[0] as Extract<DictationRendererCommand, { type: 'start' }>
  ).operationId;
  assert.equal(typeof operationId, 'string');
  await Promise.resolve();
  assert.equal(started, false);
  service.reply({ type: 'recording', operationId });
  await start;
  assert.equal(started, true);

  const stopped = service.stop();
  assert.equal(await service.status(), 'transcribing');
  service.reply({
    type: 'audio',
    operationId,
    audio: WEBM_AUDIO,
    mimeType: 'audio/webm;codecs=opus',
  });
  assert.equal(await stopped, 'hello world');
  assert.equal(await service.status(), 'idle');
});

test('restores system audio before transcription starts', async () => {
  const events: string[] = [];
  const commands: DictationRendererCommand[] = [];
  const service = createDictationService(
    (command) => {
      commands.push(command);
      events.push(`command:${command.type}`);
    },
    {
      muteSystemAudio: () => {
        events.push('mute');
        return Promise.resolve({
          restore: () => {
            events.push('restore');
            return Promise.resolve();
          },
        });
      },
      transcribeAudio: () => Promise.resolve('hello'),
    },
  );

  const start = service.start({ muteSystemAudioWhileRecording: true });
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(events, ['mute', 'command:start']);
  const operationId = (
    commands[0] as Extract<DictationRendererCommand, { type: 'start' }>
  ).operationId;
  service.reply({
    type: 'recording',
    operationId,
  });
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
  service.reply({
    type: 'audio',
    operationId,
    audio: WEBM_AUDIO,
    mimeType: 'audio/webm;codecs=opus',
  });
  assert.equal(await stop, 'hello');
});

test('restores system audio when recording fails', async () => {
  let restores = 0;
  const service = createDictationService(ignoreCommand, {
    muteSystemAudio: () =>
      Promise.resolve({
        restore: () => {
          restores += 1;
          return Promise.resolve();
        },
      }),
  });

  const start = service.start({ muteSystemAudioWhileRecording: true });
  await new Promise((resolve) => setImmediate(resolve));
  service.reply({ type: 'error', message: 'Microphone failed' });

  await assert.rejects(start, MICROPHONE_FAILED_PATTERN);
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
  const service = createDictationService(ignoreCommand);

  const start = service.start();
  service.reply({ type: 'error', message: 'Microphone did not become ready' });

  await assert.rejects(start, MICROPHONE_NOT_READY_PATTERN);
  assert.equal(await service.status(), 'idle');
});

test('cancels a pending transcription', async () => {
  const commands: DictationRendererCommand[] = [];
  const prepared: string[] = [];
  const cancelledPreparations: string[] = [];
  const service = createDictationService((command) => commands.push(command), {
    prepareTranscription: (operationId) => {
      prepared.push(operationId);
      return Promise.resolve();
    },
    cancelPreparedTranscription: (operationId) => {
      cancelledPreparations.push(operationId);
      return Promise.resolve();
    },
  });
  const start = service.start();
  const operationId = (
    commands[0] as Extract<DictationRendererCommand, { type: 'start' }>
  ).operationId;
  service.reply({ type: 'recording', operationId });
  await start;
  const stopped = service.stop();
  await service.cancel();
  await assert.rejects(stopped, DICTATION_CANCELLED_PATTERN);
  assert.equal(await service.status(), 'idle');
  assert.deepEqual(prepared, [operationId]);
  assert.deepEqual(cancelledPreparations, [operationId]);
});

test('sends captured audio to cloud transcription and releases the operation', async () => {
  const commands: DictationRendererCommand[] = [];
  const inputs: unknown[] = [];
  const service = createDictationService((command) => commands.push(command), {
    transcribeAudio: (input) => {
      inputs.push(input);
      return Promise.resolve('cloud transcript');
    },
  });
  const start = service.start();
  const operationId = (
    commands[0] as Extract<DictationRendererCommand, { type: 'start' }>
  ).operationId;
  service.reply({ type: 'recording', operationId });
  await start;
  const stop = service.stop();
  await Promise.resolve();
  service.reply({
    type: 'audio',
    operationId,
    audio: WEBM_AUDIO,
    mimeType: 'audio/webm;codecs=opus',
  });
  assert.equal(await stop, 'cloud transcript');
  assert.equal(inputs.length, 1);
  assert.equal(commands.at(-1)?.type, 'release');
});

test('keeps failed segmented recordings for retry and removes them after transcription', async () => {
  const commands: DictationRendererCommand[] = [];
  const saved = new Map<string, Uint8Array[]>();
  const calls: string[] = [];
  let shouldFail = true;
  const service = createDictationService((command) => commands.push(command), {
    recordings: {
      save: async (id, segments) => {
        saved.set(id, segments);
      },
      list: async () =>
        [...saved.keys()].map((id) => ({ id, createdAt: 1, segmentCount: 2 })),
      load: async (id) => saved.get(id) ?? [],
      remove: async (id) => {
        saved.delete(id);
      },
    },
    transcribeAudio: async ({ operationId }) => {
      calls.push(operationId);
      if (shouldFail) throw new Error('Upload rejected');
      return operationId.endsWith('-1') ? 'world' : 'hello';
    },
  });
  const started = service.start();
  const operationId = (
    commands[0] as Extract<DictationRendererCommand, { type: 'start' }>
  ).operationId;
  service.reply({ type: 'recording', operationId });
  await started;
  const stopped = service.stop();
  service.reply({
    type: 'audio',
    operationId,
    segments: [WEBM_AUDIO, WEBM_AUDIO],
    mimeType: 'audio/webm',
  });
  await assert.rejects(stopped, /Upload rejected/);
  assert.equal((await service.recordings()).length, 1);
  shouldFail = false;
  assert.equal(await service.retry(operationId), 'hello world');
  assert.deepEqual(calls, [operationId, operationId, `${operationId}-1`]);
  assert.equal((await service.recordings()).length, 1);
  await service.deleteRecording(operationId);
  assert.deepEqual(await service.recordings(), []);
});

test('rejects cloud transcription failures and releases the operation', async () => {
  const commands: DictationRendererCommand[] = [];
  const service = createDictationService((command) => commands.push(command), {
    transcribeAudio: () =>
      Promise.reject(new Error('Cloud transcription failed')),
  });
  const start = service.start();
  const operationId = (
    commands[0] as Extract<DictationRendererCommand, { type: 'start' }>
  ).operationId;
  service.reply({ type: 'recording', operationId });
  await start;
  const stop = service.stop();
  service.reply({
    type: 'audio',
    operationId,
    audio: WEBM_AUDIO,
    mimeType: 'audio/webm;codecs=opus',
  });

  await assert.rejects(stop, CLOUD_FAILED_PATTERN);
  assert.equal(await service.status(), 'idle');
  assert.deepEqual(commands.at(-1), { type: 'release', operationId });
});

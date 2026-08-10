import assert from 'node:assert/strict';
import test from 'node:test';
import { createDictationExtension } from './dictation';

test('does not declare a default dictation shortcut', () => {
  const extension = createDictationExtension();
  const command = extension.commands[0] as any;
  assert.equal(extension.id, 'nevermind.dictation');
  assert.equal(command.globalShortcut, undefined);
  assert.equal(command.shortcutScope, undefined);
  assert.equal(command.background, true);
});

test('toggles recording and returns a concealed paste action', async () => {
  const extension = createDictationExtension();
  const command = extension.commands[0] as any;
  let recording = false;
  const starts: unknown[] = [];
  const pastes: unknown[] = [];
  const context = {
    storage: {
      get: async () => ({
        deviceId: 'default',
        keepAliveMs: 300_000,
        dictionary: '',
      }),
    },
    dictation: {
      status: async () => (recording ? 'recording' : 'idle'),
      modelCacheStatus: async () => 'cached',
      prepareModel: async () => {},
      start: async (options: unknown) => {
        recording = true;
        starts.push(options);
      },
      stop: async () => {
        recording = false;
        return 'hello world';
      },
    },
    ui: {
      toast: (input: unknown) => input,
      indicator: {
        show: () => {},
        update: () => {},
        hide: () => {},
      },
    },
    actions: {
      pasteText: (text: string, title: string, options: unknown) => ({
        type: 'pasteText',
        text,
        title,
        options,
      }),
    },
    navigation: {
      run: (action: unknown) => {
        pastes.push(action);
        return { action };
      },
    },
  };

  await command.run(context as never);
  assert.deepEqual(starts, [
    { deviceId: 'default', modelKeepAliveMs: 300_000 },
  ]);
  const result = await command.run(context as never);
  assert.deepEqual(result, { action: pastes[0] });
  assert.deepEqual(pastes, [
    {
      type: 'pasteText',
      text: 'hello world',
      title: 'Paste Dictation',
      options: {
        concealed: true,
        restoreClipboard: true,
        dismissAfterRun: 'auto',
      },
    },
  ]);
});

test('prepares a missing model before opening the microphone', async () => {
  const extension = createDictationExtension();
  const command = extension.commands[0] as any;
  const events: string[] = [];
  const prepared: unknown[] = [];
  const context = {
    storage: {
      get: async () => ({
        deviceId: 'default',
        keepAliveMs: 300_000,
        dictionary: '',
      }),
    },
    dictation: {
      status: async () => 'idle',
      modelCacheStatus: async () => {
        events.push('cache-status');
        return 'missing';
      },
      prepareModel: async (options: unknown) => {
        events.push('prepare-model');
        prepared.push(options);
      },
      start: async () => {
        events.push('start');
      },
    },
    ui: {
      toast: (input: unknown) => input,
      indicator: {
        show: () => {},
        update: () => {},
        hide: () => {},
      },
    },
  };

  const result = await command.run(context as never);
  assert.deepEqual(events, ['cache-status', 'prepare-model', 'start']);
  assert.deepEqual(prepared, [{ modelKeepAliveMs: 300_000 }]);
  assert.deepEqual(result, { message: 'Listening...', tone: 'info' });
});

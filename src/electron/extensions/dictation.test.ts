import assert from 'node:assert/strict';
import test from 'node:test';
import { createDictationExtension } from './dictation';

function actionFactory(title: string, handler: unknown, options = {}) {
  return { ...options, type: 'runExtensionAction', title, __handler: handler };
}

function actionBuilders(overrides: Record<string, unknown> = {}) {
  return {
    background: (title: string, handler: unknown, options = {}) =>
      actionFactory(title, handler, { ...options, dismissAfterRun: 'auto' }),
    run: (title: string, handler: unknown, options = {}) =>
      actionFactory(title, handler, options),
    ...overrides,
  };
}

function rootItemFor(context: any) {
  const extension = createDictationExtension();
  const items = extension.rootItems(context);
  assert.equal(items.length, 1);
  return items[0] as any;
}

test('exposes one Dictate root item with settings under Cmd-K', () => {
  const extension = createDictationExtension();
  const item = rootItemFor({ actions: actionBuilders() });
  const panelActions = item.actionPanel.sections[0].actions;

  assert.equal('commands' in extension, false);
  assert.equal(item.id, 'dictation');
  assert.equal(item.title, 'Dictate');
  assert.equal(item.primaryAction.title, 'Dictate');
  assert.deepEqual(
    panelActions.map((action: any) => action.title),
    ['Dictate', 'Settings'],
  );
  assert.equal(panelActions[1].icon, 'settings-2');
});

test('does not declare a default dictation shortcut', () => {
  const extension = createDictationExtension();
  const item = rootItemFor({ actions: actionBuilders() });
  assert.equal(extension.id, 'nevermind.dictation');
  assert.equal(item.globalShortcut, undefined);
  assert.equal(item.shortcutScope, undefined);
  assert.equal(item.primaryAction.dismissAfterRun, 'auto');
});

test('toggles recording and returns a concealed paste action', async () => {
  let recording = false;
  const starts: unknown[] = [];
  const pastes: unknown[] = [];
  const context = {
    storage: {
      get: async () => ({
        deviceId: 'default',
        keepAliveMs: 300_000,
        dictionary: '',
        copyToClipboard: false,
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
    actions: actionBuilders({
      pasteText: (text: string, title: string, options: unknown) => ({
        type: 'pasteText',
        text,
        title,
        options,
      }),
    }),
    navigation: {
      run: (action: unknown) => {
        pastes.push(action);
        return { action };
      },
    },
  };
  const item = rootItemFor(context);

  await item.primaryAction.__handler(context, {});
  assert.deepEqual(starts, [
    { deviceId: 'default', modelKeepAliveMs: 300_000 },
  ]);
  const result = await item.primaryAction.__handler(context, {});
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
    actions: actionBuilders(),
  };
  const item = rootItemFor(context);

  const result = await item.primaryAction.__handler(context, {});
  assert.deepEqual(events, ['cache-status', 'prepare-model', 'start']);
  assert.deepEqual(prepared, [{ modelKeepAliveMs: 300_000 }]);
  assert.deepEqual(result, { message: 'Listening...', tone: 'info' });
});

test('leaves the transcription on the clipboard when enabled', async () => {
  const pastes: unknown[] = [];
  let recording = false;
  const context = {
    storage: {
      get: async () => ({
        deviceId: 'default',
        keepAliveMs: 300_000,
        dictionary: '',
        copyToClipboard: true,
      }),
    },
    dictation: {
      status: async () => (recording ? 'recording' : 'idle'),
      modelCacheStatus: async () => 'cached',
      prepareModel: async () => {},
      start: async () => {
        recording = true;
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
    actions: actionBuilders({
      pasteText: (text: string, title: string, options: unknown) => ({
        type: 'pasteText',
        text,
        title,
        options,
      }),
    }),
    navigation: {
      run: (action: unknown) => {
        pastes.push(action);
        return { action };
      },
    },
  };
  const item = rootItemFor(context);

  await item.primaryAction.__handler(context, {});
  await item.primaryAction.__handler(context, {});
  assert.deepEqual(pastes[0], {
    type: 'pasteText',
    text: 'hello world',
    title: 'Paste Dictation',
    options: {
      concealed: false,
      restoreClipboard: false,
      dismissAfterRun: 'auto',
    },
  });
});

test('renders and saves multiline dictionary settings', async () => {
  const saved: unknown[] = [];
  let saveHandler:
    | ((context: any, action: any) => Promise<unknown>)
    | undefined;
  const context = {
    storage: {
      get: async () => ({
        deviceId: 'default',
        keepAliveMs: 300_000,
        dictionary: 'Nevermind\nParakeet',
        copyToClipboard: true,
      }),
      set: async (...input: unknown[]) => {
        saved.push(input);
      },
    },
    dictation: {
      devices: async () => [{ id: 'default', title: 'System Default' }],
    },
    actions: actionBuilders({
      run: (title: string, handler: typeof saveHandler) => {
        saveHandler = handler;
        return { type: 'runExtensionAction', title, __handler: handler };
      },
    }),
    navigation: {
      pop: () => ({ navigation: 'pop' }),
    },
    ui: {
      form: (view: unknown) => view,
    },
  };
  const item = rootItemFor(context);
  const settingsAction = item.actionPanel.sections[0].actions[1];

  const result = await settingsAction.__handler(context, {});
  const view = result.view as any;
  assert.equal(
    view.fields.find((field: any) => field.id === 'dictionary').value,
    'Nevermind\nParakeet',
  );
  assert.equal(
    view.fields.find((field: any) => field.id === 'copyToClipboard').value,
    true,
  );
  if (!saveHandler) throw new Error('Settings save handler was not registered');

  await saveHandler(context, {
    formValues: {
      deviceId: 'default',
      keepAliveMs: '1800000',
      dictionary: 'Nevermind\nParakeet\nWASM',
      copyToClipboard: false,
    },
  });
  assert.deepEqual(saved, [
    [
      'settings',
      {
        deviceId: 'default',
        keepAliveMs: 1_800_000,
        dictionary: 'Nevermind\nParakeet\nWASM',
        copyToClipboard: false,
      },
    ],
  ]);
});

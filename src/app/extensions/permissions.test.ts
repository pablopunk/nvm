import assert from 'node:assert/strict';
import test, { mock } from 'node:test';
import type { PermissionReader } from './permissions';

mock.module('electron', {
  namedExports: {
    systemPreferences: {
      getMediaAccessStatus: () => 'unknown',
      isTrustedAccessibilityClient: () => false,
      askForMediaAccess: async () => true,
    },
  },
});

const {
  createPermissionsExtension,
  normalizeMediaPermissionState,
  permissionsView,
  readOsPermissions,
} = await import('./permissions');

const PRIVACY_PANE_ID = 'com.apple.settings.PrivacySecurity.extension';

function fakeReader(
  states: Record<string, string>,
  requested: string[] = [],
): PermissionReader {
  return {
    platform: 'darwin',
    mediaStatus: (kind) => states[kind] ?? 'unknown',
    accessibilityTrusted: () => states.accessibility === 'granted',
    requestMediaAccess: async (kind) => {
      requested.push(kind);
      return true;
    },
    requestAccessibilityAccess: () => {
      requested.push('accessibility');
      return true;
    },
  };
}

function testContext(capturedViews: unknown[] = []) {
  return {
    actions: {
      run: (title: string, handler: unknown, options = {}) => ({
        ...options,
        type: 'runExtensionAction',
        title,
        __handler: handler,
      }),
      system: {
        openSystemSettings: (title: string, options = {}) => ({
          type: 'openSystemSettings',
          title,
          ...options,
        }),
      },
    },
    ui: {
      list: (view: unknown) => {
        capturedViews.push(view);
        return view;
      },
    },
  };
}

test('normalizes media permission states', () => {
  assert.equal(normalizeMediaPermissionState('granted'), 'allowed');
  assert.equal(normalizeMediaPermissionState('denied'), 'denied');
  assert.equal(normalizeMediaPermissionState('restricted'), 'denied');
  assert.equal(
    normalizeMediaPermissionState('not-determined'),
    'not-determined',
  );
  assert.equal(normalizeMediaPermissionState('unknown'), 'unknown');
  assert.equal(normalizeMediaPermissionState(undefined), 'unknown');
});

test('reads permission rows with attention first', () => {
  const rows = readOsPermissions(
    fakeReader({
      microphone: 'granted',
      camera: 'denied',
      screen: 'not-determined',
      accessibility: 'granted',
    }),
  );
  assert.deepEqual(
    rows.map((row) => row.key),
    [
      'camera',
      'screen-recording',
      'microphone',
      'accessibility',
      'files-and-folders',
    ],
  );
  assert.equal(
    rows.find((row) => row.key === 'files-and-folders')?.state,
    'unknown',
  );
});

test('exposes the Nevermind OS Permissions root item', () => {
  const extension = createPermissionsExtension();
  const context = testContext();
  const [item] = extension.rootItems(context) as any[];
  assert.equal(extension.id, 'nevermind.permissions');
  assert.equal(item.id, 'os-permissions');
  assert.equal(item.title, 'Nevermind OS Permissions');
  assert.equal(item.primaryAction.type, 'runExtensionAction');
  assert.deepEqual(extension.searchItems(context, 'microphone').length, 1);
  assert.deepEqual(extension.searchItems(context, 'calculator').length, 0);
  assert.deepEqual(extension.searchItems(context, '').length, 1);
});

test('renders one row per permission with state subtitles', async () => {
  const context = testContext();
  const reader = fakeReader({
    microphone: 'not-determined',
    camera: 'denied',
    screen: 'granted',
    accessibility: 'granted',
  });
  const view = (await permissionsView(context, reader)) as any;
  assert.equal(view.id, 'os-permissions');
  assert.equal(view.subtitle, '2 of 4 allowed');
  const subtitles = Object.fromEntries(
    view.items.map((item: any) => [item.id, item.subtitle]),
  );
  assert.match(subtitles['os-permission:microphone'], /Not asked yet/);
  assert.match(subtitles['os-permission:camera'], /Not allowed/);
  assert.match(subtitles['os-permission:screen-recording'], /Allowed/);
  assert.match(
    subtitles['os-permission:files-and-folders'],
    /Review in Settings/,
  );
});

test('asks for microphone access when it was never requested', async () => {
  const requested: string[] = [];
  const context = testContext();
  const reader = fakeReader({ microphone: 'not-determined' }, requested);
  const view = (await permissionsView(context, reader)) as any;
  const microphone = view.items.find(
    (item: any) => item.id === 'os-permission:microphone',
  );
  const result = await microphone.primaryAction.__handler(context);
  assert.deepEqual(requested, ['microphone']);
  assert.equal(result.navigation, 'replace');
  assert.equal(result.view.subtitle, '0 of 2 allowed');
  assert.deepEqual(microphone.actions, [
    {
      type: 'openSystemSettings',
      title: 'Microphone',
      paneId: PRIVACY_PANE_ID,
      anchor: 'Microphone',
    },
  ]);
});

test('opens the privacy pane for denied permissions', async () => {
  const context = testContext();
  const reader = fakeReader({ camera: 'denied' });
  const view = (await permissionsView(context, reader)) as any;
  const camera = view.items.find(
    (item: any) => item.id === 'os-permission:camera',
  );
  assert.deepEqual(camera.primaryAction, {
    type: 'openSystemSettings',
    title: 'Camera',
    paneId: PRIVACY_PANE_ID,
    anchor: 'Camera',
  });
});

test('requests accessibility access through the system prompt', async () => {
  const requested: string[] = [];
  const context = testContext();
  const reader = fakeReader({ accessibility: 'missing' }, requested);
  const view = (await permissionsView(context, reader)) as any;
  const accessibility = view.items.find(
    (item: any) => item.id === 'os-permission:accessibility',
  );
  assert.match(accessibility.subtitle, /Not allowed/);
  const result = await accessibility.primaryAction.__handler(context);
  assert.deepEqual(requested, ['accessibility']);
  assert.equal(result.navigation, 'replace');
});

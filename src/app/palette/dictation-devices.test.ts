import assert from 'node:assert/strict';
import test from 'node:test';
import { dictationDevices } from './dictation-devices';

test('keeps the system default separate from its current physical device', () => {
  assert.deepEqual(
    dictationDevices([
      {
        deviceId: 'default',
        kind: 'audioinput',
        label: 'Default - Blue Yeti',
      },
      { deviceId: 'yeti-id', kind: 'audioinput', label: 'Blue Yeti' },
      { deviceId: 'camera-id', kind: 'videoinput', label: 'Camera' },
    ]),
    [
      { id: 'default', title: 'Default', isDefault: true },
      { id: 'yeti-id', title: 'Blue Yeti', isDefault: false },
    ],
  );
});

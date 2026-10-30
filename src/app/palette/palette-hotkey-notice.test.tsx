import assert from 'node:assert/strict';
import test from 'node:test';
import { renderToString } from 'react-dom/server';
import {
  PaletteHotkeyNotice,
  paletteHotkeyNoticeCopy,
  paletteHotkeyNoticeIsVisible,
} from './palette-hotkey-notice';

const unconfigured = {
  accelerator: '',
  configured: false,
  registered: false,
  platform: 'darwin',
} as const;

const failed = {
  accelerator: 'Alt+Space',
  configured: true,
  registered: false,
  recoveryAccelerator: 'Command+Alt+K',
  platform: 'darwin',
} as const;

test('an unset palette shortcut asks for one without alarming the user', () => {
  assert.deepEqual(paletteHotkeyNoticeCopy(unconfigured), {
    state: 'unconfigured',
    title: 'No shortcut set',
    detail: 'Set one to open Nevermind from anywhere.',
    actionLabel: 'Set shortcut',
  });
});

test('a failed registration names the shortcut and the recovery route', () => {
  assert.equal(paletteHotkeyNoticeCopy(failed).title, '⌥Space is unavailable');
  assert.equal(
    paletteHotkeyNoticeCopy(failed).detail,
    'Use ⌘⌥K to reopen Nevermind.',
  );
  assert.equal(
    paletteHotkeyNoticeCopy({ ...failed, recoveryAccelerator: undefined })
      .detail,
    'Reopen Nevermind from your app launcher.',
  );
});

test('the notice stays a single slim row with one shortcut action', () => {
  const html = renderToString(
    <PaletteHotkeyNotice status={failed} onSetShortcut={() => {}} />,
  );
  assert.match(html, /role="alert"/);
  assert.match(html, /data-state="registration-failed"/);
  assert.equal(html.match(/<button/g)?.length, 1);
  assert.match(html, />Change shortcut</);
  assert.match(
    renderToString(
      <PaletteHotkeyNotice status={unconfigured} onSetShortcut={() => {}} />,
    ),
    /role="status"/,
  );
});

test('the notice persists in every palette state until a shortcut registers', () => {
  assert.equal(paletteHotkeyNoticeIsVisible(failed), true);
  assert.equal(paletteHotkeyNoticeIsVisible(unconfigured), true);
  assert.equal(
    paletteHotkeyNoticeIsVisible({ ...failed, registered: true }),
    false,
  );
  assert.equal(paletteHotkeyNoticeIsVisible(null), false);
});

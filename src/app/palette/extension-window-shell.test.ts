import assert from 'node:assert/strict';
import test from 'node:test';
import { extensionWindowShellClassName } from './extension-window-shell';

test('menu bar popovers get the popover panel class', () => {
  assert.equal(
    extensionWindowShellClassName(
      { chrome: 'none', titleBar: 'hidden', position: 'menu-bar' },
      false,
    ),
    'extensionWindowShell extensionWindowChromeNone extensionWindowPopover',
  );
});

test('regular windows keep their existing classes', () => {
  assert.equal(
    extensionWindowShellClassName({ titleBar: 'hidden' }, true),
    'extensionWindowShell loadingBorder extensionWindowTitleBarHidden',
  );
});

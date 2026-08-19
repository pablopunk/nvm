import assert from 'node:assert/strict';
import test from 'node:test';
import { actionMenuPresentation } from './action-menu-presentation';

test('action menus and nested submenus are always compact', () => {
  assert.equal(actionMenuPresentation('actions'), 'compact');
  assert.equal(actionMenuPresentation('submenu'), 'compact');
});

test('workflows entered from an action menu use the full palette', () => {
  assert.equal(actionMenuPresentation('confirmation'), 'default');
  assert.equal(actionMenuPresentation('prompt'), 'default');
});

import assert from 'node:assert/strict';
import test from 'node:test';
import {
  actionMenuPresentation,
  confirmationReturnSurface,
} from './action-menu-presentation';

test('action menus and nested submenus are always compact', () => {
  assert.equal(actionMenuPresentation('actions'), 'compact');
  assert.equal(actionMenuPresentation('submenu'), 'compact');
});

test('confirmations stay compact while prompts use the full palette', () => {
  assert.equal(actionMenuPresentation('confirmation'), 'compact');
  assert.equal(actionMenuPresentation('prompt'), 'default');
});

test('confirmation cancellation returns to its exact source surface', () => {
  assert.equal(confirmationReturnSurface(true, false), 'submenu');
  assert.equal(confirmationReturnSurface(false, true), 'panel');
  assert.equal(confirmationReturnSurface(false, false), 'view');
});
